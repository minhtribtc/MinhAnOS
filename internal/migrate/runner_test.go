package migrate

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPlan(t *testing.T) {
	first := Module{ID: "identity", Steps: []Step{{1, "create_identity", "SELECT 1"}}}
	second := Module{ID: "profile", Dependencies: map[string]Range{"identity": {1, 2}}, Steps: []Step{{1, "create_profile", "SELECT 1"}}}
	plan, e := Plan([]Module{second, first})
	if e != nil || plan[0].ID != "identity" {
		t.Fatal(plan, e)
	}
	bad := [][]Module{{first, first}, {second}, {{ID: "bad id", Steps: first.Steps}}, {{ID: "gap", Steps: []Step{{2, "gap", "SELECT 1"}}}}, {{ID: "a", Dependencies: map[string]Range{"b": {1, 2}}, Steps: first.Steps}, {ID: "b", Dependencies: map[string]Range{"a": {1, 2}}, Steps: first.Steps}}, {{ID: "profile", Dependencies: map[string]Range{"identity": {2, 3}}, Steps: second.Steps}, first}}
	for _, modules := range bad {
		if _, e := Plan(modules); e == nil {
			t.Fatal("invalid plan accepted", modules)
		}
	}
}
func testPool(t *testing.T) *pgxpool.Pool {
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
	name := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		admin.Close()
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
	t.Cleanup(func() { db.Close(); admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	return db
}
func TestDatabaseAtomicityConcurrencyAndDrift(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	module := Module{ID: "files", Steps: []Step{{1, "create_marker", "CREATE TABLE marker(id integer PRIMARY KEY); INSERT INTO marker VALUES(1);"}}}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Apply(ctx, db, module) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e := db.QueryRow(ctx, "SELECT count(*) FROM marker").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	if e := db.QueryRow(ctx, "SELECT count(*) FROM platform_migrations").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	drift := module
	drift.Steps = []Step{{1, "create_marker", module.Steps[0].SQL + " -- edited"}}
	if e := Apply(ctx, db, drift); e == nil || !strings.Contains(e.Error(), "checksum drift") {
		t.Fatal("drift accepted", e)
	}
	newer := module
	newer.Steps = append(append([]Step{}, module.Steps...), Step{2, "broken", "CREATE TABLE rolled_back(id integer); SELECT missing_function();"})
	if e := Apply(ctx, db, newer); e == nil {
		t.Fatal("failed SQL accepted")
	}
	var exists bool
	db.QueryRow(ctx, "SELECT to_regclass('rolled_back') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("DDL did not roll back")
	}
	db.QueryRow(ctx, "SELECT count(*) FROM platform_migrations").Scan(&count)
	if count != 1 {
		t.Fatal("failed step entered ledger")
	}
	newer.Steps[1] = Step{2, "expand_marker", "ALTER TABLE marker ADD COLUMN value text;"}
	if e := Apply(ctx, db, newer); e != nil {
		t.Fatal(e)
	}
	if e := Apply(ctx, db, module); e == nil {
		t.Fatal("older binary accepted newer schema")
	}
}
func TestDatabaseDependencyOrderAndCancellation(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	parent := Module{ID: "identity", Steps: []Step{{1, "parent", "CREATE TABLE parent(id int PRIMARY KEY);"}}}
	child := Module{ID: "files", Dependencies: map[string]Range{"identity": {1, 2}}, Steps: []Step{{1, "child", "CREATE TABLE child(id int REFERENCES parent(id));"}}}
	if e := Apply(ctx, db, child, parent); e != nil {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	slow := Module{ID: "slow", Steps: []Step{{1, "cancelled", "CREATE TABLE cancelled(id int); SELECT pg_sleep(10);"}}}
	if e := Apply(cancelled, db, slow); e == nil {
		t.Fatal("cancelled migration committed")
	}
	var exists bool
	db.QueryRow(ctx, "SELECT to_regclass('cancelled') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("cancelled DDL survived")
	}
}

func TestDatabaseConnectionLossRecovery(t *testing.T) {
	db := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	module := Module{ID: "crash", Steps: []Step{{1, "create_after_crash", "CREATE TABLE migration_crash_target(id int); SELECT pg_sleep(1);"}}}
	done := make(chan error, 1)
	go func() { done <- Apply(ctx, db, module) }()
	var pid int
	for i := 0; i < 100; i++ {
		e := db.QueryRow(ctx, "SELECT pid FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND query LIKE $1 LIMIT 1", "CREATE TABLE migration_crash_target%").Scan(&pid)
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("migration backend not observed")
	}
	if _, e := db.Exec(ctx, "SELECT pg_terminate_backend($1)", pid); e != nil {
		t.Fatal(e)
	}
	if e := <-done; e == nil {
		t.Fatal("terminated migration succeeded")
	}
	var exists bool
	db.QueryRow(ctx, "SELECT to_regclass('migration_crash_target') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("uncommitted crash DDL survived")
	}
	if e := Apply(ctx, db, module); e != nil {
		t.Fatal("recovery failed", e)
	}
}
