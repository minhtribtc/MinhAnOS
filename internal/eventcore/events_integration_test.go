package eventcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The outbox: only committed events are delivered, each consumer gets each matching event once with its side effect
// in the same commit, failures retry with backoff and end dead, a new consumer starts after existing events, and the
// watermark passes settled events only.
func TestEventBus(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("eventcore_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = root.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, _ := pgxpool.ParseConfig(dsn)
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); root.Close() })
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "CREATE TABLE effects(consumer text, subject text)"); e != nil {
		t.Fatal(e)
	}
	publish := func(name, subject string, commit bool) {
		tx, _ := db.Begin(ctx)
		if e := Publish(ctx, tx, Event{Name: name, Source: "test", Subject: subject, Data: map[string]any{"id": subject}}); e != nil {
			t.Fatal(e)
		}
		if commit {
			tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
	}
	effects := func(consumer string) (n int) {
		db.QueryRow(ctx, "SELECT count(*) FROM effects WHERE consumer=$1", consumer).Scan(&n)
		return
	}
	tx, _ := db.Begin(ctx)
	if Publish(ctx, tx, Event{Name: "bad name", Source: "test", Subject: "x"}) != ErrInvalid {
		t.Fatal("invalid name accepted")
	}
	tx.Rollback(ctx)
	publish("test.thing.changed.v1", "before", true)

	fail := true
	b := &Bus{DB: db}
	b.Subscribe("test.ok", []string{"test.thing.changed.v1"}, func(ctx context.Context, tx pgx.Tx, e Event) error {
		if e.SchemaVersion != 1 || e.Data["id"] != e.Subject {
			t.Error("envelope", e)
		}
		_, err := tx.Exec(ctx, "INSERT INTO effects VALUES('test.ok',$1)", e.Subject)
		return err
	})
	b.Subscribe("test.flaky", []string{"test.thing.changed.v1"}, func(ctx context.Context, tx pgx.Tx, e Event) error {
		tx.Exec(ctx, "INSERT INTO effects VALUES('test.flaky',$1)", e.Subject)
		if fail {
			return errors.New("not yet")
		}
		return nil
	})
	if e = b.Register(ctx); e != nil {
		t.Fatal(e)
	}
	publish("test.thing.changed.v1", "rolled-back", false)
	publish("test.thing.changed.v1", "a", true)
	publish("test.other.changed.v1", "ignored", true)
	if n, e := b.Deliver(ctx, "test.ok"); e != nil || n != 1 || effects("test.ok") != 1 {
		t.Fatal("deliver: only the committed matching event after the subscription", n, e, effects("test.ok"))
	}
	if n, _ := b.Deliver(ctx, "test.ok"); n != 0 || effects("test.ok") != 1 {
		t.Fatal("redelivered", n)
	}
	// A failing handler's side effect is rolled back; the delivery waits for its backoff, then succeeds once.
	if n, _ := b.Deliver(ctx, "test.flaky"); n != 1 || effects("test.flaky") != 0 {
		t.Fatal("failed side effect kept", n, effects("test.flaky"))
	}
	if n, _ := b.Deliver(ctx, "test.flaky"); n != 0 {
		t.Fatal("backoff ignored")
	}
	fail = false
	db.Exec(ctx, "UPDATE platform_event_deliveries SET next_attempt_at=now() WHERE consumer='test.flaky'")
	if n, _ := b.Deliver(ctx, "test.flaky"); n != 1 || effects("test.flaky") != 1 {
		t.Fatal("retry", n, effects("test.flaky"))
	}
	// After MaxAttempts a delivery is dead and no longer tried.
	fail = true
	publish("test.thing.changed.v1", "doomed", true)
	for i := 0; i < MaxAttempts; i++ {
		db.Exec(ctx, "UPDATE platform_event_deliveries SET next_attempt_at=now() WHERE consumer='test.flaky' AND state='failed'")
		b.Deliver(ctx, "test.flaky")
	}
	var state string
	var attempts int
	db.QueryRow(ctx, "SELECT d.state,d.attempts FROM platform_event_deliveries d JOIN platform_events e ON e.seq=d.event_seq WHERE d.consumer='test.flaky' AND e.subject='doomed'").Scan(&state, &attempts)
	if state != "dead" || attempts != MaxAttempts {
		t.Fatal("dead letter", state, attempts)
	}
	// The watermark passes settled, delivered events only.
	db.Exec(ctx, "UPDATE platform_events SET occurred_at=now()-interval '1 hour'")
	b.Deliver(ctx, "test.ok")
	var since, last int64
	db.QueryRow(ctx, "SELECT since_seq FROM platform_event_consumers WHERE consumer='test.ok'").Scan(&since)
	db.QueryRow(ctx, "SELECT max(seq) FROM platform_events").Scan(&last)
	if since != last {
		t.Fatal("watermark", since, last)
	}
	// Retention drops old events with their deliveries.
	db.Exec(ctx, "UPDATE platform_events SET occurred_at=now()-interval '31 days'")
	if e = Retain(ctx, db); e != nil {
		t.Fatal(e)
	}
	var left int
	db.QueryRow(ctx, "SELECT count(*) FROM platform_events").Scan(&left)
	if left != 0 {
		t.Fatal("retention", left)
	}
}
