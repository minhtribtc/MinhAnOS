// Package notifycore is the Notification Center backend (minimal v1): a per-user inbox fed from module
// outboxes by a background worker. Only backend modules publish (no browser publish API); notices carry
// a kind and params, never text, so the UI renders them in the user's language.
package notifycore

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
)

//go:embed migrations/001_inbox.sql
var inboxSQL string

func Module() migrate.Module {
	return migrate.Module{ID: "notifications", Steps: []migrate.Step{{ID: 1, Name: "inbox", SQL: inboxSQL}}}
}

// ApplySchema runs after the identity module: the worker reads identity_notification_outbox.
func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}

const (
	// RetentionDays bounds the inbox; older notices are purged by the worker.
	RetentionDays = 90
	MaxPage       = 50
	MaxMarkIDs    = 100
	deliverBatch  = 100
)

var (
	ErrInvalid = errors.New("invalid notification request")
	uuidRE     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Notification struct {
	ID        string         `json:"id"`
	Source    string         `json:"source"`
	Kind      string         `json:"kind"`
	Severity  string         `json:"severity"`
	Mandatory bool           `json:"mandatory"`
	Params    map[string]any `json:"params"`
	CreatedAt time.Time      `json:"createdAt"`
	ReadAt    *time.Time     `json:"readAt"`
}

type Page struct {
	Items       []Notification `json:"items"`
	NextCursor  *string        `json:"nextCursor"`
	UnreadCount int64          `json:"unreadCount"`
}

type Store struct{ DB *pgxpool.Pool }

// cursor is "<created_at unix microseconds>.<id>" in base64url: the position after the last item.
func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMicro(), 10) + "." + id))
}

func decodeCursor(c string) (*time.Time, *string, error) {
	if c == "" {
		return nil, nil, nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(c)
	if e != nil {
		return nil, nil, ErrInvalid
	}
	micros, id, ok := strings.Cut(string(raw), ".")
	n, e := strconv.ParseInt(micros, 10, 64)
	if !ok || e != nil || !uuidRE.MatchString(id) {
		return nil, nil, ErrInvalid
	}
	at := time.UnixMicro(n)
	return &at, &id, nil
}

// List returns the recipient's notices, newest first, with the unread count for the bell.
func (s Store) List(ctx context.Context, recipient string, unreadOnly bool, cursor string, limit int) (Page, error) {
	if limit < 1 || limit > MaxPage || !uuidRE.MatchString(recipient) {
		return Page{}, ErrInvalid
	}
	at, id, e := decodeCursor(cursor)
	if e != nil {
		return Page{}, e
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,source,kind,severity,mandatory,params,created_at,read_at FROM notifications
		WHERE recipient_id=$1 AND (NOT $2 OR read_at IS NULL) AND ($3::timestamptz IS NULL OR (created_at,id)<($3,$4::uuid))
		ORDER BY created_at DESC,id DESC LIMIT $5`, recipient, unreadOnly, at, id, limit+1)
	if e != nil {
		return Page{}, e
	}
	defer rows.Close()
	page := Page{Items: []Notification{}}
	for rows.Next() {
		var n Notification
		if e = rows.Scan(&n.ID, &n.Source, &n.Kind, &n.Severity, &n.Mandatory, &n.Params, &n.CreatedAt, &n.ReadAt); e != nil {
			return Page{}, e
		}
		page.Items = append(page.Items, n)
	}
	if e = rows.Err(); e != nil {
		return Page{}, e
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[limit-1]
		next := encodeCursor(last.CreatedAt, last.ID)
		page.NextCursor = &next
	}
	page.UnreadCount, e = s.Unread(ctx, recipient)
	return page, e
}

func (s Store) Unread(ctx context.Context, recipient string) (int64, error) {
	var n int64
	e := s.DB.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE recipient_id=$1 AND read_at IS NULL", recipient).Scan(&n)
	return n, e
}

// MarkRead marks the given notices, or every notice created up to upTo ("mark all" without touching notices
// that arrived after the user looked), as read. Repeating it changes nothing: the operation is idempotent.
func (s Store) MarkRead(ctx context.Context, recipient string, ids []string, upTo *time.Time) (updated, unread int64, err error) {
	if !uuidRE.MatchString(recipient) || (len(ids) == 0) == (upTo == nil) || len(ids) > MaxMarkIDs {
		return 0, 0, ErrInvalid
	}
	for _, id := range ids {
		if !uuidRE.MatchString(id) {
			return 0, 0, ErrInvalid
		}
	}
	if ids == nil {
		ids = []string{}
	}
	tag, e := s.DB.Exec(ctx, "UPDATE notifications SET read_at=now() WHERE recipient_id=$1 AND read_at IS NULL AND (id=ANY($2::uuid[]) OR created_at<=$3::timestamptz)", recipient, ids, upTo)
	if e != nil {
		return 0, 0, e
	}
	unread, e = s.Unread(ctx, recipient)
	return tag.RowsAffected(), unread, e
}

// Deliver moves one batch of undelivered notices from each module outbox (identity security notices,
// storage quota notices) into the inbox in one transaction. The inbox id is the outbox id (ON CONFLICT DO
// NOTHING), so a crash between the two steps cannot duplicate.
func (s Store) Deliver(ctx context.Context) (int64, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, `WITH batch AS (SELECT id FROM identity_notification_outbox WHERE delivered_at IS NULL ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED),
		moved AS (INSERT INTO notifications(id,recipient_id,source,kind,severity,mandatory,params,request_id,correlation_id,created_at)
			SELECT o.id,o.recipient_id,'com.minhanos.account-center',o.kind,o.severity,o.mandatory,o.details,o.request_id,o.correlation_id,o.created_at
			FROM identity_notification_outbox o JOIN batch USING(id) ON CONFLICT(id) DO NOTHING)
		UPDATE identity_notification_outbox SET delivered_at=now() WHERE id IN (SELECT id FROM batch)`, deliverBatch)
	if e != nil {
		return 0, e
	}
	quota, e := tx.Exec(ctx, `WITH batch AS (SELECT id FROM storage_notification_outbox WHERE delivered_at IS NULL ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED),
		moved AS (INSERT INTO notifications(id,recipient_id,source,kind,severity,mandatory,params,request_id,correlation_id,created_at)
			SELECT o.id,o.recipient_id,o.source,o.kind,o.severity,false,o.details,o.request_id,o.correlation_id,o.created_at
			FROM storage_notification_outbox o JOIN batch USING(id) ON CONFLICT(id) DO NOTHING)
		UPDATE storage_notification_outbox SET delivered_at=now() WHERE id IN (SELECT id FROM batch)`, deliverBatch)
	if e != nil {
		return 0, e
	}
	return max(tag.RowsAffected(), quota.RowsAffected()), tx.Commit(ctx)
}

// Purge removes notices older than the retention period, a bounded batch per call.
func (s Store) Purge(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, "DELETE FROM notifications WHERE id IN (SELECT id FROM notifications WHERE created_at<now()-make_interval(days=>$1) LIMIT 500)", RetentionDays)
	if e != nil {
		return e
	}
	// Delivered quota outbox rows are only a hand-over buffer: kept 30 days for diagnosis (storage SQL 005).
	_, e = s.DB.Exec(ctx, "DELETE FROM storage_notification_outbox WHERE id IN (SELECT id FROM storage_notification_outbox WHERE delivered_at<now()-interval '30 days' LIMIT 500)")
	return e
}

// Run delivers pending notices every interval (polling keeps v1 simple; the bell polls the unread count).
func (s Store) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for {
			n, e := s.Deliver(ctx)
			if e != nil {
				if ctx.Err() == nil {
					slog.Error("notifications.delivery_failed", "error", e)
				}
				break
			}
			if n < deliverBatch {
				break
			}
		}
		if e := s.Purge(ctx); e != nil && ctx.Err() == nil {
			slog.Error("notifications.retention_failed", "error", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
