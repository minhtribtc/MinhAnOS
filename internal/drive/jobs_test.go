package drive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/eventcore"
	"minhanos.local/platform/internal/jobcore"
)

// Files jobs on the platform engine (ADR 0010): a folder copy runs to the end, a file error fails it with its code,
// cancel is cooperative, and disabling the account cancels its jobs. Lease, retry and fencing: jobcore tests.
func TestFilesJobsCopyCancelAndLease(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("files_jobs_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = root.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); root.Close() })
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY,public_id uuid); INSERT INTO users VALUES('u1','00000000-0000-4000-8000-000000000001'); CREATE TABLE storage_shares(key text PRIMARY KEY,organization_id uuid)"); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = eventcore.ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	s := &Server{Identity: &SQLIdentity{db: db}, DataDir: t.TempDir(), Jobs: jobcore.NewWorker(db)}
	s.RegisterJobs(s.Jobs, db)
	home := filepath.Join(s.DataDir, "users", "u1", "files")
	if e = os.MkdirAll(filepath.Join(home, "album", "2026"), 0750); e != nil {
		t.Fatal(e)
	}
	big := strings.Repeat("x", 5<<20)
	if e = os.WriteFile(filepath.Join(home, "album", "2026", "a.bin"), []byte(big), 0640); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(home, "album", "b.txt"), []byte("b"), 0640); e != nil {
		t.Fatal(e)
	}

	create := func(user, dest string) Job {
		j, e := jobcore.Enqueue(ctx, db, jobcore.Spec{Type: "files.copy", Actor: user, Unit: "bytes", Payload: copyPayload{"personal", "album", dest}})
		if e != nil {
			t.Fatal(e)
		}
		return j
	}
	jobs := jobcore.Store{DB: db}
	job := create("u1", "album copy")
	if job.State != "queued" || job.Progress.Unit != "bytes" || job.MaxAttempts != 3 {
		t.Fatal("create", job)
	}
	if !s.Jobs.RunOne(ctx) {
		t.Fatal("job not claimed")
	}
	if job, e = jobs.Get(ctx, "u1", job.ID); e != nil || job.State != "succeeded" || job.Progress.Completed != int64(len(big)+1) || *job.Progress.Total != int64(len(big)+1) || job.Error != nil {
		t.Fatal("finished", job, e)
	}
	if data, e := os.ReadFile(filepath.Join(home, "album copy", "2026", "a.bin")); e != nil || string(data) != big {
		t.Fatal("copied tree", e)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.DataDir, "users", "u1", "jobs")); len(entries) != 0 {
		t.Fatal("staging left behind", entries)
	}
	if _, e = jobs.Get(ctx, "u2", job.ID); !errors.Is(e, jobcore.ErrNotFound) {
		t.Fatal("another user's job visible", e)
	}
	if s.Jobs.RunOne(ctx) {
		t.Fatal("finished job claimed again")
	}

	// The destination now exists: a second run fails with the file error instead of overwriting.
	again := create("u1", "album copy")
	s.Jobs.RunOne(ctx)
	if again, _ = jobs.Get(ctx, "u1", again.ID); again.State != "failed" || again.Error == nil || again.Error.Code != "FILES_EXISTS" {
		t.Fatal("existing destination", again)
	}

	// Cancel: a queued job ends at once; cancelling again is idempotent.
	queued := create("u1", "other")
	for range 2 {
		if queued, e = jobs.Cancel(ctx, "u1", queued.ID); e != nil || queued.State != "cancelled" {
			t.Fatal("cancel queued", queued, e)
		}
	}

	// Disabling the account (identity.user.disabled.v1) cancels its unfinished Files jobs.
	pending := create("u1", "fifth")
	bus := &eventcore.Bus{DB: db}
	s.RegisterEvents(bus)
	if e = bus.Register(ctx); e != nil {
		t.Fatal(e)
	}
	tx, _ := db.Begin(ctx)
	if e = eventcore.Publish(ctx, tx, eventcore.Event{Name: "identity.user.disabled.v1", Source: "test", Subject: "00000000-0000-4000-8000-000000000001", Data: map[string]any{"userId": "00000000-0000-4000-8000-000000000001"}}); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	if n, e := bus.Deliver(ctx, "files.jobs"); e != nil || n != 1 {
		t.Fatal("deliver", n, e)
	}
	if pending, _ = jobs.Get(ctx, "u1", pending.ID); pending.State != "cancelled" {
		t.Fatal("disabled account's job not cancelled", pending)
	}
	if e = RetainAudit(ctx, db); e != nil {
		t.Fatal(e)
	}
}
