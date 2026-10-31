package jobcore

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

type codedError string

func (e codedError) Error() string   { return string(e) }
func (e codedError) JobCode() string { return string(e) }

func testDB(t *testing.T) *pgxpool.Pool {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("jobcore_test_%d", time.Now().UnixNano())
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
	return db
}

// The engine: outcomes by error kind, bounded retry with backoff, lease takeover with fencing, cooperative cancel,
// dedupe of unfinished jobs, workers claiming only their own types, and schedules running once across instances.
func TestJobEngine(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	w := NewWorker(db)
	var outcome error
	var seen *Run
	w.Handle("test.work", func(ctx context.Context, r *Run) error { seen = r; return outcome }, Options{Retry: true})
	w.Handle("test.once", func(ctx context.Context, r *Run) error { return errors.New("host side effect failed") }, Options{})
	store := Store{DB: db}
	state := func(id string) Job {
		j, e := store.Get(ctx, "u1", id)
		if e != nil {
			t.Fatal(e)
		}
		return j
	}
	enqueue := func(s Spec) Job {
		j, e := Enqueue(ctx, db, s)
		if e != nil {
			t.Fatal(e)
		}
		return j
	}

	// Success, with the payload and actor handed to the handler.
	ok := enqueue(Spec{Type: "test.work", Actor: "u1", Payload: map[string]string{"k": "v"}})
	if !w.RunOne(ctx) || state(ok.ID).State != "succeeded" || seen.Actor != "u1" {
		t.Fatal("success", state(ok.ID))
	}
	var payload map[string]string
	if seen.Decode(&payload); payload["k"] != "v" {
		t.Fatal("payload", payload)
	}
	// A coded error fails at once with its code.
	outcome = codedError("TEST_REFUSED")
	coded := enqueue(Spec{Type: "test.work", Actor: "u1"})
	w.RunOne(ctx)
	if j := state(coded.ID); j.State != "failed" || j.Error == nil || j.Error.Code != "TEST_REFUSED" || j.Attempt != 1 {
		t.Fatal("coded", j)
	}
	// A transient error requeues with a backoff (not claimable at once) until attempts run out.
	outcome = errors.New("transient")
	retry := enqueue(Spec{Type: "test.work", Actor: "u1", MaxAttempts: 2})
	w.RunOne(ctx)
	if j := state(retry.ID); j.State != "queued" || j.Attempt != 1 || j.Error.Code != "JOB_FAILED" {
		t.Fatal("retry", j)
	}
	if w.RunOne(ctx) {
		t.Fatal("backoff ignored")
	}
	db.Exec(ctx, "UPDATE platform_jobs SET run_after=now() WHERE id=$1", retry.ID)
	w.RunOne(ctx)
	if j := state(retry.ID); j.State != "failed" || j.Attempt != 2 {
		t.Fatal("attempts exhausted", j)
	}
	// A handler registered without Retry is never repeated.
	once := enqueue(Spec{Type: "test.once", Actor: "u1"})
	w.RunOne(ctx)
	if j := state(once.ID); j.State != "failed" || j.Attempt != 1 {
		t.Fatal("non-retryable", j)
	}

	// Lease: another worker takes over an expired lease; the old run is fenced out and cancel reaches the new run.
	outcome = nil
	leased := enqueue(Spec{Type: "test.work", Actor: "u1"})
	var first *Run
	w.Handle("test.work", func(ctx context.Context, r *Run) error {
		if first == nil {
			first = r
			db.Exec(ctx, "UPDATE platform_jobs SET lease_until=now()-interval '1 second' WHERE id=$1", r.ID)
			if !w.RunOne(ctx) { // a second instance reclaims the job while this run is still going
				t.Error("expired lease not reclaimed")
			}
			_, err := r.Report(ctx, 1, nil)
			return err
		}
		if r.Fencing != first.Fencing+1 || r.Attempt != 2 {
			t.Error("reclaim fencing", r.Fencing, r.Attempt)
		}
		store.Cancel(ctx, "u1", r.ID)
		return r.Progress(ctx, 1, nil)
	}, Options{Retry: true})
	w.RunOne(ctx)
	if j := state(leased.ID); j.State != "cancelled" {
		t.Fatal("lease takeover and cancel", j)
	}
	// An expired lease beyond maxAttempts fails as interrupted.
	lost := enqueue(Spec{Type: "test.work", Actor: "u1", MaxAttempts: 1})
	db.Exec(ctx, "UPDATE platform_jobs SET state='running',attempt=1,lease_until=now()-interval '1 second' WHERE id=$1", lost.ID)
	w.RunOne(ctx)
	if j := state(lost.ID); j.State != "failed" || j.Error.Code != "JOB_INTERRUPTED" {
		t.Fatal("interrupted", j)
	}

	// Dedupe: the same unfinished work is returned, not queued twice; after it finishes it may be queued again.
	a := enqueue(Spec{Type: "test.work", Actor: "u1", DedupeKey: "k"})
	if b := enqueue(Spec{Type: "test.work", Actor: "u1", DedupeKey: "k"}); b.ID != a.ID {
		t.Fatal("dedupe", a.ID, b.ID)
	}
	if _, e := store.Cancel(ctx, "u1", a.ID); e != nil {
		t.Fatal(e)
	}
	if c := enqueue(Spec{Type: "test.work", Actor: "u1", DedupeKey: "k"}); c.ID == a.ID {
		t.Fatal("finished job blocks new work")
	}
	db.Exec(ctx, "UPDATE platform_jobs SET state='cancelled',finished_at=now() WHERE state='queued'")

	// Workers claim only their own types; other users' jobs are invisible.
	other := NewWorker(db)
	foreign := enqueue(Spec{Type: "test.work", Actor: "u1"})
	if other.RunOne(ctx) {
		t.Fatal("worker claimed a type it does not handle")
	}
	if _, e := store.Get(ctx, "u2", foreign.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("another actor's job visible", e)
	}
	if list, _ := store.List(ctx, "u1", []string{"test.work"}, 0); len(list) < 5 {
		t.Fatal("list", len(list))
	}
	if CancelActor(ctx, db, "u1"); state(foreign.ID).State != "cancelled" {
		t.Fatal("cancel actor", state(foreign.ID))
	}

	// Schedules: two instances, one job per due slot; the next slot is an interval away.
	runs := 0
	for _, wk := range []*Worker{NewWorker(db), NewWorker(db)} {
		wk.Handle("test.tick", func(ctx context.Context, r *Run) error { runs++; return nil }, Options{})
		wk.Every("test.tick", "test.tick", time.Hour)
		if e := wk.register(ctx); e != nil {
			t.Fatal(e)
		}
		wk.due(ctx)
		for wk.RunOne(ctx) {
		}
	}
	var next time.Time
	db.QueryRow(ctx, "SELECT next_run_at FROM platform_schedules WHERE key='test.tick'").Scan(&next)
	if runs != 1 || time.Until(next) < 50*time.Minute {
		t.Fatal("schedule", runs, next)
	}
	// Retention removes old finished jobs only.
	db.Exec(ctx, "UPDATE platform_jobs SET finished_at=now()-interval '8 days' WHERE id=$1", ok.ID)
	if e := w.retain(ctx, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := store.Get(ctx, "u1", ok.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("retention", e)
	}
	if _, e := store.Get(ctx, "u1", coded.ID); e != nil {
		t.Fatal("recent job removed", e)
	}
}
