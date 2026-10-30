package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const legacySchema = `CREATE TABLE users(id text PRIMARY KEY,username text UNIQUE NOT NULL,display_name text NOT NULL,role text NOT NULL CHECK(role IN ('admin','user')),password_hash text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()); CREATE TABLE sessions(token_hash text PRIMARY KEY,user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,csrf_token text NOT NULL,expires_at timestamptz NOT NULL); CREATE TABLE drive_sessions(LIKE sessions INCLUDING ALL); ALTER TABLE drive_sessions ADD FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE;`

func fixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated identity database not configured")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("identity_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	config.MaxConns = 8
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	if _, e = db.Exec(ctx, legacySchema); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,'legacy','Legacy','admin',$2)", strings.Repeat("a", 64), strings.Repeat("b", 32)+":"+strings.Repeat("c", 64)); e != nil {
		t.Fatal(e)
	}
	return db
}
func legacyFingerprint(ctx context.Context, db *pgxpool.Pool) (string, error) {
	var s string
	e := db.QueryRow(ctx, `SELECT md5(coalesce(string_agg(jsonb_build_array(id,username,display_name,role,password_hash,created_at)::text,'' ORDER BY id),'')) FROM users`).Scan(&s)
	return s, e
}
func TestAdoptionBackfillAndCompatibility(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	before, e := legacyFingerprint(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	// Both existing cookie realms must retain their old tokens/FKs after adoption.
	for _, realm := range []string{"sessions", "drive_sessions"} {
		if _, e = db.Exec(ctx, "INSERT INTO "+realm+" VALUES($1,$2,$3,now()+interval '1 hour')", strings.Repeat("d", 64), strings.Repeat("a", 64), strings.Repeat("e", 64)); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- ApplySchema(ctx, db) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	repo := Repository{db}
	status, e := repo.Status(ctx)
	if e != nil || status.Pending != 1 {
		t.Fatal(status, e)
	}
	if _, e = repo.PublicUser(ctx, strings.Repeat("a", 64)); !errors.Is(e, ErrNotReady) {
		t.Fatal("unbackfilled user exposed", e)
	}
	if n, e := repo.BackfillBatch(ctx, 1); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	user, e := repo.PublicUser(ctx, strings.Repeat("a", 64))
	if e != nil || len(user.ID) != 36 {
		t.Fatal(user, e)
	}
	if storage, e := repo.LegacyID(ctx, user.ID); e != nil || storage != strings.Repeat("a", 64) {
		t.Fatal(storage, e)
	}
	raw, _ := json.Marshal(user)
	if strings.Contains(string(raw), `"password"`) || strings.Contains(string(raw), "password_hash") || strings.Contains(string(raw), "passwordHash") || strings.Contains(string(raw), strings.Repeat("a", 64)) {
		t.Fatal("private identity data leaked")
	}
	if n, e := repo.BackfillBatch(ctx, 1); e != nil || n != 0 {
		t.Fatal("backfill was not idempotent", n, e)
	}
	for _, query := range []string{"UPDATE users SET public_id=gen_random_uuid()", "UPDATE users SET public_id=NULL", "UPDATE users SET id=repeat('f',64)", "UPDATE users SET password_credential_id=gen_random_uuid()"} {
		if _, e = db.Exec(ctx, query); e == nil {
			t.Fatal("immutable key updated")
		}
	}
	after, e := legacyFingerprint(ctx, db)
	if e != nil || before != after {
		t.Fatal("legacy users changed", e)
	}
	// A legacy INSERT need not know any new column; defaults create UUIDs.
	_, e = db.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,'next','Next','user',$2)", strings.Repeat("f", 64), strings.Repeat("b", 32)+":"+strings.Repeat("c", 64))
	if e != nil {
		t.Fatal(e)
	}
	status, e = repo.Status(ctx)
	if e != nil || !status.Complete || status.Users != 2 {
		t.Fatal(status, e)
	}
	for _, realm := range []string{"sessions", "drive_sessions"} {
		var n int
		db.QueryRow(ctx, "SELECT count(*) FROM "+realm).Scan(&n)
		if n != 1 {
			t.Fatal("sessions changed")
		}
	}
	var grants int
	db.QueryRow(ctx, "SELECT count(*) FROM identity_role_bindings").Scan(&grants)
	if grants != 0 {
		t.Fatal("implicit privileges created")
	}
	if _, e = repo.BackfillBatch(ctx, 1001); e == nil {
		t.Fatal("unbounded batch accepted")
	}
}
func TestRejectIncompatibleIdentity(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	db.Exec(ctx, "ALTER TABLE users DROP CONSTRAINT users_username_key")
	if e := ApplySchema(ctx, db); e == nil {
		t.Fatal("bad adoption accepted")
	}
	var exists bool
	db.QueryRow(ctx, "SELECT to_regclass('platform_migrations') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("failed adoption left history")
	}
}
func TestRestoreUpgradeIdentity(t *testing.T) {
	dsn := os.Getenv("MIGRATION_UPGRADE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated restored database not configured")
	}
	parsed, e := url.Parse(dsn)
	if e != nil || !strings.HasPrefix(parsed.Hostname(), "minhanos-migration-upgrade-") {
		t.Fatal("isolated restore database required")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	before, e := legacyFingerprint(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	repo := Repository{db}
	for i := 0; i < 1000; i++ {
		n, err := repo.BackfillBatch(ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	status, e := repo.Status(ctx)
	if e != nil || !status.Complete {
		t.Fatal(status, e)
	}
	after, e := legacyFingerprint(ctx, db)
	if e != nil || before != after {
		t.Fatal("restored identities changed", e)
	}
}

func TestBoundedConcurrentBackfill(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	if _, e := db.Exec(ctx, `INSERT INTO users(id,username,display_name,role,password_hash) SELECT lpad(to_hex(i),64,'0'),'sample_'||i,'Sample','user',repeat('b',32)||':'||repeat('c',64) FROM generate_series(1,40) i`); e != nil {
		t.Fatal(e)
	}
	before, e := legacyFingerprint(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	repo := Repository{db}
	n, e := repo.BackfillBatch(ctx, 7)
	if e != nil || n != 7 {
		t.Fatal(n, e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := 0; batch < 20; batch++ {
				n, e := repo.BackfillBatch(ctx, 5)
				if e != nil {
					errs <- e
					return
				}
				if n > 5 {
					errs <- errors.New("batch bound exceeded")
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	status, e := repo.Status(ctx)
	if e != nil || !status.Complete || status.Users != 41 {
		t.Fatal(status, e)
	}
	var unique int
	db.QueryRow(ctx, "SELECT count(DISTINCT public_id) FROM users").Scan(&unique)
	if unique != 41 {
		t.Fatal("UUIDs were not unique")
	}
	after, e := legacyFingerprint(ctx, db)
	if e != nil || before != after {
		t.Fatal("legacy data changed", e)
	}
}
