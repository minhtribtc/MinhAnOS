package drive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Files audit: structured rows, no personal paths, append-only, bounded retention.
func TestFilesAuditPrivacyAndAppendOnly(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("files_audit_test_%d", time.Now().UnixNano())
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
	if _, e = db.Exec(ctx, "INSERT INTO users VALUES('u1','11111111-1111-4111-8111-111111111111')"); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	s := &SQLIdentity{db: db}
	u := User{ID: "u1"}
	for _, ev := range []AuditEvent{{Action: "files.upload", Root: "personal", Path: "/private/diary.txt"}, {Action: "files.mkdir", Root: "share", Share: "media", Path: "/x"}, {Action: "files.delete", Root: "system", Path: "/etc/motd"}} {
		if e = s.Audit(ctx, u, ev); e != nil {
			t.Fatal(ev, e)
		}
	}
	var paths, shares, actors int
	if e = db.QueryRow(ctx, "SELECT count(*) FILTER (WHERE details ? 'path'),count(share_key),count(actor_id) FROM files_events").Scan(&paths, &shares, &actors); e != nil {
		t.Fatal(e)
	}
	if paths != 1 || shares != 1 || actors != 3 {
		t.Fatal("privacy rules", paths, shares, actors)
	}
	if _, e = db.Exec(ctx, "INSERT INTO files_events(action,root,request_id,details) VALUES('files.upload','personal','r','{\"path\":\"/a\"}')"); e == nil {
		t.Fatal("personal path accepted")
	}
	if _, e = db.Exec(ctx, "DELETE FROM files_events"); e == nil {
		t.Fatal("delete allowed")
	}
	if _, e = db.Exec(ctx, "UPDATE files_events SET action='files.x'"); e == nil {
		t.Fatal("update allowed")
	}
	if e = RetainAudit(ctx, db); e != nil {
		t.Fatal(e)
	}
	// Replay store (files SQL 002): same key + same request replays, another request conflicts.
	key, req := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, _, found, e := s.Replay(ctx, "u1", key, req); e != nil || found {
		t.Fatal("empty store", found, e)
	}
	if e = s.Remember(ctx, "u1", key, req, 201, []byte(`{"ok":true}`)); e != nil {
		t.Fatal(e)
	}
	if status, body, found, e := s.Replay(ctx, "u1", key, req); e != nil || !found || status != 201 || !strings.Contains(string(body), "ok") {
		t.Fatal("replay", status, string(body), found, e)
	}
	if _, _, _, e := s.Replay(ctx, "u1", key, strings.Repeat("c", 64)); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatal("conflict", e)
	}
	if _, _, found, _ := s.Replay(ctx, "u2", key, req); found {
		t.Fatal("replay crossed users")
	}
}
