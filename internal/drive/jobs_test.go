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
)

// Files jobs (files SQL 003): a folder copy runs to the end, cancel is cooperative, and a lost lease fences out
// the old run so only the reclaiming one may finish.
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
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY,public_id uuid); CREATE TABLE storage_shares(key text PRIMARY KEY,organization_id uuid)"); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	store := &SQLIdentity{db: db}
	s := &Server{Identity: store, DataDir: t.TempDir()}
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

	job, e := store.CreateJob(ctx, "u1", "personal", "album", "album copy")
	if e != nil || job.State != "queued" || job.Progress.Unit != "bytes" || job.MaxAttempts != 3 {
		t.Fatal("create", job, e)
	}
	if !s.runJob(ctx) {
		t.Fatal("job not claimed")
	}
	if job, e = store.UserJob(ctx, "u1", job.ID); e != nil || job.State != "succeeded" || job.Progress.Completed != int64(len(big)+1) || *job.Progress.Total != int64(len(big)+1) || job.Error != nil {
		t.Fatal("finished", job, e)
	}
	if data, e := os.ReadFile(filepath.Join(home, "album copy", "2026", "a.bin")); e != nil || string(data) != big {
		t.Fatal("copied tree", e)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.DataDir, "users", "u1", "jobs")); len(entries) != 0 {
		t.Fatal("staging left behind", entries)
	}
	if _, e = store.UserJob(ctx, "u2", job.ID); !errors.Is(e, errJobNotFound) {
		t.Fatal("another user's job visible", e)
	}
	if s.runJob(ctx) {
		t.Fatal("finished job claimed again")
	}

	// The destination now exists: a second run fails with the file error instead of overwriting.
	again, _ := store.CreateJob(ctx, "u1", "personal", "album", "album copy")
	s.runJob(ctx)
	if again, _ = store.UserJob(ctx, "u1", again.ID); again.State != "failed" || again.Error == nil || again.Error.Code != "FILES_EXISTS" {
		t.Fatal("existing destination", again)
	}

	// Cancel: a queued job ends at once; cancelling again is idempotent.
	queued, _ := store.CreateJob(ctx, "u1", "personal", "album", "other")
	for range 2 {
		if queued, e = store.CancelJob(ctx, "u1", queued.ID); e != nil || queued.State != "cancelled" {
			t.Fatal("cancel queued", queued, e)
		}
	}

	// Lease: an expired running job is reclaimed with a new fencing token; the old run can no longer report.
	leased, _ := store.CreateJob(ctx, "u1", "personal", "album", "third")
	first, ok, e := store.ClaimJob(ctx)
	if e != nil || !ok || first.ID != leased.ID || first.Attempt != 1 {
		t.Fatal("claim", first, ok, e)
	}
	if _, e = db.Exec(ctx, "UPDATE files_jobs SET lease_until=now()-interval '1 second' WHERE id=$1", leased.ID); e != nil {
		t.Fatal(e)
	}
	second, ok, e := store.ClaimJob(ctx)
	if e != nil || !ok || second.Fencing != first.Fencing+1 || second.Attempt != 2 {
		t.Fatal("reclaim", second, ok, e)
	}
	if _, e = store.ReportJob(ctx, leased.ID, first.Fencing, 1, nil); !errors.Is(e, errJobLeaseLost) {
		t.Fatal("old run still reports", e)
	}
	// A running job is cancelled when the worker next reports.
	if _, e = store.CancelJob(ctx, "u1", leased.ID); e != nil {
		t.Fatal(e)
	}
	if cancel, e := store.ReportJob(ctx, leased.ID, second.Fencing, 1, nil); e != nil || !cancel {
		t.Fatal("cancel not seen", cancel, e)
	}
	if e = store.FinishJob(ctx, leased.ID, second.Fencing, "cancelled", "", false); e != nil {
		t.Fatal(e)
	}
	// A transient failure requeues while attempts remain; a cancel request wins over the retry.
	if leased, _ = store.UserJob(ctx, "u1", leased.ID); leased.State != "cancelled" {
		t.Fatal("cancelled", leased)
	}
	retry, _ := store.CreateJob(ctx, "u1", "personal", "album", "fourth")
	claimed, _, _ := store.ClaimJob(ctx)
	if e = store.FinishJob(ctx, retry.ID, claimed.Fencing, "failed", "JOB_FAILED", true); e != nil {
		t.Fatal(e)
	}
	if retry, _ = store.UserJob(ctx, "u1", retry.ID); retry.State != "queued" {
		t.Fatal("transient failure not requeued", retry)
	}
	if e = RetainAudit(ctx, db); e != nil {
		t.Fatal(e)
	}
}
