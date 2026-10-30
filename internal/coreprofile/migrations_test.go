package coreprofile

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func profileTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated migration test database not configured")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("migration_test_profile_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
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
	t.Cleanup(func() { db.Close(); admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY); INSERT INTO users VALUES('existing-user');"); e != nil {
		t.Fatal(e)
	}
	return db
}
func TestProfileFreshAndAdoptPreservesData(t *testing.T) {
	db := profileTestDB(t)
	ctx := context.Background()
	if _, e := New(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, "INSERT INTO user_profiles(user_id,locale,theme,time_zone) VALUES('existing-user','en-US','dark','UTC'); DELETE FROM platform_migrations WHERE module='profile';"); e != nil {
		t.Fatal(e)
	}
	if _, e := New(ctx, db); e != nil {
		t.Fatal(e)
	}
	var locale, theme, zone string
	if e := db.QueryRow(ctx, "SELECT locale,theme,time_zone FROM user_profiles WHERE user_id='existing-user'").Scan(&locale, &theme, &zone); e != nil || locale != "en-US" || theme != "dark" || zone != "UTC" {
		t.Fatal(locale, theme, zone, e)
	}
}
func TestProfileRejectsIncompatibleExistingSchema(t *testing.T) {
	db := profileTestDB(t)
	ctx := context.Background()
	if _, e := db.Exec(ctx, "CREATE TABLE user_profiles(user_id integer);"); e != nil {
		t.Fatal(e)
	}
	if _, e := New(ctx, db); e == nil {
		t.Fatal("incompatible adoption accepted")
	}
	var exists bool
	db.QueryRow(ctx, "SELECT to_regclass('platform_migrations') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("failed adoption committed ledger")
	}
}

func TestUpgradeRestoredDatabase(t *testing.T) {
	dsn := os.Getenv("MIGRATION_UPGRADE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("restored isolated database not configured")
	}
	parsed, e := url.Parse(dsn)
	if e != nil || !strings.HasPrefix(parsed.Hostname(), "minhanos-migration-upgrade-") {
		t.Fatal("upgrade test requires the isolated rehearsal container")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	snapshot := func() string {
		var fingerprint string
		e := db.QueryRow(ctx, `SELECT md5((SELECT coalesce(string_agg(row_to_json(u)::text,'' ORDER BY id),'') FROM users u)||(SELECT coalesce(string_agg(row_to_json(p)::text,'' ORDER BY user_id),'') FROM user_profiles p))`).Scan(&fingerprint)
		if e != nil {
			t.Fatal(e)
		}
		return fingerprint
	}
	before := snapshot()
	if _, e = New(ctx, db); e != nil {
		t.Fatal(e)
	}
	if after := snapshot(); before != after {
		t.Fatal("existing user/profile data changed")
	}
	var count int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM platform_migrations WHERE module='profile' AND version=1 AND status='applied'").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}
