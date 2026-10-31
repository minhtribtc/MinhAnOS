// Package eventcore is the platform event bus (ADR 0010, contracts/events/events.v1.yaml): a transactional outbox
// with durable, named consumers. Publish writes the event in the caller's transaction; a Bus delivers each matching
// event to each consumer at least once, recording the delivery in the same transaction as the handler's side effect.
// Events never carry authority: security effects stay synchronous in the domain transaction.
package eventcore

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
	"minhanos.local/platform/internal/requestid"
)

//go:embed migrations/001_events.sql
var eventsSQL string

func Module() migrate.Module {
	return migrate.Module{ID: "events", Steps: []migrate.Step{{ID: 1, Name: "events", SQL: eventsSQL}}}
}

func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}

const (
	RetentionDays = 30
	MaxAttempts   = 8
	batch         = 50
	maxBackoff    = time.Hour
	// settle is how old an event must be before a consumer's watermark may pass it: a transaction that took a lower
	// sequence number may still commit after a higher one was delivered.
	settle = 10 * time.Minute
)

var (
	ErrInvalid      = errors.New("invalid event")
	namePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+\.v([1-9][0-9]*)$`)
	consumerPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)*$`)
)

// Event is the contracts/events/event.v1 envelope. Data holds IDs and versions only — no secrets, no file content.
type Event struct {
	ID            string         `json:"eventId"`
	Name          string         `json:"name"`
	SchemaVersion int            `json:"schemaVersion"`
	OccurredAt    time.Time      `json:"occurredAt"`
	Source        string         `json:"source"`
	Subject       string         `json:"subject"`
	Organization  string         `json:"organizationId,omitempty"`
	CorrelationID string         `json:"correlationId"`
	Data          map[string]any `json:"data"`
}

// Publish writes e in tx, so the event exists iff the domain change commits. The schema version is the name's
// version suffix; the correlation ID is the request's.
func Publish(ctx context.Context, tx pgx.Tx, e Event) error {
	m := namePattern.FindStringSubmatch(e.Name)
	if m == nil || e.Source == "" || e.Subject == "" {
		return ErrInvalid
	}
	version, _ := strconv.Atoi(m[2])
	if e.Data == nil {
		e.Data = map[string]any{}
	}
	data, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_events(name,schema_version,source,subject,organization_id,correlation_id,data) VALUES($1,$2,$3,$4,nullif($5,'')::uuid,$6,$7)",
		e.Name, version, e.Source, e.Subject, e.Organization, requestid.From(ctx).Correlation, data)
	return err
}

// Handler applies one event inside tx (a savepoint of the delivery transaction). Returning an error rolls the side
// effect back and retries later; handlers must tolerate redelivery and out-of-order events (compare versions).
type Handler func(ctx context.Context, tx pgx.Tx, e Event) error

type subscription struct {
	consumer string
	names    []string
	handler  Handler
}

// Bus delivers events to the consumers subscribed on it.
type Bus struct {
	DB   *pgxpool.Pool
	subs []subscription
}

// Subscribe registers a durable consumer of the named events; call before Start.
func (b *Bus) Subscribe(consumer string, names []string, h Handler) {
	if !consumerPattern.MatchString(consumer) || len(names) == 0 {
		panic("eventcore: invalid consumer " + consumer)
	}
	for _, n := range names {
		if !namePattern.MatchString(n) {
			panic("eventcore: invalid event name " + n)
		}
	}
	b.subs = append(b.subs, subscription{consumer, names, h})
}

// Register records the consumers; a new one starts after the latest event.
func (b *Bus) Register(ctx context.Context) error {
	for _, s := range b.subs {
		if _, err := b.DB.Exec(ctx, "INSERT INTO platform_event_consumers(consumer,names,since_seq) VALUES($1,$2,(SELECT coalesce(max(seq),0) FROM platform_events)) ON CONFLICT(consumer) DO UPDATE SET names=EXCLUDED.names",
			s.consumer, s.names); err != nil {
			return err
		}
	}
	return nil
}

// Start registers the consumers and delivers every interval until ctx ends.
func (b *Bus) Start(ctx context.Context, interval time.Duration) error {
	if err := b.Register(ctx); err != nil || len(b.subs) == 0 {
		return err
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			for _, s := range b.subs {
				for {
					n, err := b.Deliver(ctx, s.consumer)
					if err != nil && ctx.Err() == nil {
						slog.Error("events.delivery_failed", "consumer", s.consumer, "error", err)
					}
					if err != nil || n < batch {
						break
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

// Deliver hands one batch of pending events to a consumer and reports how many it handled. One instance at a time
// delivers to a consumer (advisory lock); others skip.
func (b *Bus) Deliver(ctx context.Context, consumer string) (int, error) {
	var sub *subscription
	for i := range b.subs {
		if b.subs[i].consumer == consumer {
			sub = &b.subs[i]
		}
	}
	if sub == nil {
		return 0, fmt.Errorf("eventcore: unknown consumer %s", consumer)
	}
	tx, err := b.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtext('minhanos.events.'||$1))", consumer).Scan(&locked); err != nil || !locked {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT e.seq,e.id::text,e.name,e.schema_version,e.source,e.subject,coalesce(e.organization_id::text,''),e.correlation_id,e.data,e.occurred_at,coalesce(d.attempts,0)
		FROM platform_events e JOIN platform_event_consumers c ON c.consumer=$1
		LEFT JOIN platform_event_deliveries d ON d.consumer=$1 AND d.event_seq=e.seq
		WHERE e.seq>c.since_seq AND e.name=ANY(c.names) AND (d.state IS NULL OR (d.state='failed' AND d.next_attempt_at<=now()))
		ORDER BY e.seq LIMIT $2`, consumer, batch)
	if err != nil {
		return 0, err
	}
	type pending struct {
		seq      int64
		attempts int
		event    Event
	}
	var list []pending
	for rows.Next() {
		var p pending
		var data []byte
		if err = rows.Scan(&p.seq, &p.event.ID, &p.event.Name, &p.event.SchemaVersion, &p.event.Source, &p.event.Subject, &p.event.Organization, &p.event.CorrelationID, &data, &p.event.OccurredAt, &p.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		if err = json.Unmarshal(data, &p.event.Data); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, p)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range list {
		herr := handle(ctx, tx, sub.handler, p.event)
		if herr == nil {
			_, err = tx.Exec(ctx, "INSERT INTO platform_event_deliveries(consumer,event_seq,state,attempts) VALUES($1,$2,'done',$3) ON CONFLICT(consumer,event_seq) DO UPDATE SET state='done',attempts=EXCLUDED.attempts,next_attempt_at=NULL,updated_at=now()", consumer, p.seq, p.attempts+1)
		} else {
			attempts, state := p.attempts+1, "failed"
			if attempts >= MaxAttempts {
				state = "dead"
				slog.Error("events.delivery_dead", "consumer", consumer, "event", p.event.ID, "name", p.event.Name, "error", herr)
			}
			msg := herr.Error()
			if len(msg) > 500 {
				msg = msg[:500]
			}
			backoff := min(5*time.Second<<min(attempts, 12), maxBackoff)
			_, err = tx.Exec(ctx, `INSERT INTO platform_event_deliveries(consumer,event_seq,state,attempts,next_attempt_at,last_error) VALUES($1,$2,$3,$4,now()+make_interval(secs=>$5),$6)
				ON CONFLICT(consumer,event_seq) DO UPDATE SET state=EXCLUDED.state,attempts=EXCLUDED.attempts,next_attempt_at=EXCLUDED.next_attempt_at,last_error=EXCLUDED.last_error,updated_at=now()`,
				consumer, p.seq, state, attempts, backoff.Seconds(), msg)
		}
		if err != nil {
			return 0, err
		}
	}
	// Advance the watermark over settled events that are done or dead, so the scan stays short.
	if _, err = tx.Exec(ctx, `WITH c AS (SELECT since_seq,names FROM platform_event_consumers WHERE consumer=$1),
		settled AS (SELECT max(e.seq) s FROM platform_events e,c WHERE e.seq>c.since_seq AND e.occurred_at<now()-make_interval(secs=>$2)),
		open AS (SELECT min(e.seq) s FROM platform_events e,c WHERE e.seq>c.since_seq AND e.name=ANY(c.names)
			AND NOT EXISTS(SELECT 1 FROM platform_event_deliveries d WHERE d.consumer=$1 AND d.event_seq=e.seq AND d.state IN ('done','dead')))
		UPDATE platform_event_consumers SET since_seq=least((SELECT s FROM settled),coalesce((SELECT s FROM open)-1,(SELECT s FROM settled)))
		WHERE consumer=$1 AND (SELECT s FROM settled) IS NOT NULL AND least((SELECT s FROM settled),coalesce((SELECT s FROM open)-1,(SELECT s FROM settled)))>since_seq`,
		consumer, settle.Seconds()); err != nil {
		return 0, err
	}
	return len(list), tx.Commit(ctx)
}

// handle runs a handler in a savepoint: its writes commit with the delivery record or not at all.
func handle(ctx context.Context, tx pgx.Tx, h Handler, e Event) (err error) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("handler panic: %v", p)
		}
		if err != nil {
			_ = sp.Rollback(ctx)
		}
	}()
	if err = h(ctx, sp, e); err != nil {
		return err
	}
	return sp.Commit(ctx)
}

// Retain removes events older than RetentionDays (their deliveries go with them) in bounded batches.
func Retain(ctx context.Context, db *pgxpool.Pool) error {
	for {
		tag, err := db.Exec(ctx, "DELETE FROM platform_events WHERE seq IN (SELECT seq FROM platform_events WHERE occurred_at<now()-make_interval(days=>$1) ORDER BY seq LIMIT 500)", RetentionDays)
		if err != nil || tag.RowsAffected() < 500 {
			return err
		}
	}
}
