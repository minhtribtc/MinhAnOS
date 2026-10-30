// Package migrate applies immutable, transactional, additive module migrations.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"sort"
	"strings"
	"time"
)

const lockID int64 = 681024
const RunnerVersion = 1

type Step struct {
	ID        int
	Name, SQL string
}
type Range struct{ Min, Max int } // Inclusive minimum, exclusive maximum.
type Module struct {
	ID           string
	Dependencies map[string]Range
	Steps        []Step
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func checksum(s Step) string {
	sum := sha256.Sum256([]byte(s.Name + "\x00" + s.SQL))
	return hex.EncodeToString(sum[:])
}

// Plan validates the entire registered graph before the database is touched.
func Plan(modules []Module) ([]Module, error) {
	registry := map[string]Module{}
	for _, m := range modules {
		if !identifier.MatchString(m.ID) || len(m.Steps) == 0 {
			return nil, fmt.Errorf("invalid migration module %q", m.ID)
		}
		if _, ok := registry[m.ID]; ok {
			return nil, fmt.Errorf("duplicate module %s", m.ID)
		}
		for i, s := range m.Steps {
			if s.ID != i+1 || !identifier.MatchString(s.Name) || strings.TrimSpace(s.SQL) == "" {
				return nil, fmt.Errorf("invalid step %s/%d", m.ID, s.ID)
			}
		}
		registry[m.ID] = m
	}
	state := map[string]int{}
	ordered := []Module{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("migration dependency cycle at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		m, ok := registry[id]
		if !ok {
			return fmt.Errorf("missing migration dependency %s", id)
		}
		state[id] = 1
		// Sort dependencies to keep lock/DDL execution order deterministic.
		deps := make([]string, 0, len(m.Dependencies))
		for dep := range m.Dependencies {
			deps = append(deps, dep)
		}
		sortStrings(deps)
		for _, dep := range deps {
			bounds := m.Dependencies[dep]
			target, ok := registry[dep]
			if !ok {
				return fmt.Errorf("missing migration dependency %s", dep)
			}
			version := len(target.Steps)
			if bounds.Min < 1 || bounds.Max <= bounds.Min || version < bounds.Min || version >= bounds.Max {
				return fmt.Errorf("incompatible migration dependency %s -> %s", id, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		ordered = append(ordered, m)
		return nil
	}
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sortStrings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
func sortStrings(values []string) { sort.Strings(values) }

// Apply commits the registered plan and its ledger atomically. Cancellation or any
// failure rolls back both DDL and history. Nontransactional/backfill/destructive
// operations are intentionally not supported by this startup runner.
func Apply(ctx context.Context, db *pgxpool.Pool, modules ...Module) error {
	plan, err := Plan(modules)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='30s'; SET LOCAL statement_timeout='120s';"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_migrations(module text NOT NULL,version integer NOT NULL CHECK(version>0),name text NOT NULL,checksum text NOT NULL,status text NOT NULL CHECK(status='applied'),runner_version integer NOT NULL,applied_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(module,version));`); err != nil {
		return err
	}
	// Validate every registered history before executing any pending migration.
	applied := map[string]int{}
	for _, m := range plan {
		rows, e := tx.Query(ctx, "SELECT version,name,checksum,runner_version FROM platform_migrations WHERE module=$1 ORDER BY version", m.ID)
		if e != nil {
			return e
		}
		n := 0
		for rows.Next() {
			var id, runner int
			var name, digest string
			if e = rows.Scan(&id, &name, &digest, &runner); e != nil {
				rows.Close()
				return e
			}
			n++
			if id != n || id > len(m.Steps) || (runner < 1 || runner > RunnerVersion) {
				rows.Close()
				return fmt.Errorf("unsupported migration history %s/%d", m.ID, id)
			}
			step := m.Steps[id-1]
			if name != step.Name || digest != checksum(step) {
				rows.Close()
				return fmt.Errorf("migration checksum drift %s/%d", m.ID, id)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		applied[m.ID] = n
	}
	for _, m := range plan {
		for _, step := range m.Steps[applied[m.ID]:] {
			if _, err = tx.Exec(ctx, step.SQL); err != nil {
				return fmt.Errorf("migration %s/%d failed: %w", m.ID, step.ID, err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO platform_migrations(module,version,name,checksum,status,runner_version) VALUES($1,$2,$3,$4,'applied',$5)", m.ID, step.ID, step.Name, checksum(step), RunnerVersion); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
