package storagecore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const mib = int64(1 << 20)

func fixture(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated storage database not configured")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("storage_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	config.MaxConns = 16
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	// Minimal stand-in for the identity-owned users table (only columns read by Homes).
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY,public_id uuid,username text,display_name text,status text)"); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	return Store{DB: db}
}

func usage(t *testing.T, s Store, kind, id string) (used, reserved int64) {
	t.Helper()
	if kind == "share" {
		s.DB.QueryRow(context.Background(), "SELECT id::text FROM storage_shares WHERE key='homes'").Scan(&id)
	}
	if e := s.DB.QueryRow(context.Background(), "SELECT used_bytes,reserved_bytes FROM storage_quota WHERE scope_type=$1 AND scope_id=$2", kind, id).Scan(&used, &reserved); e != nil {
		t.Fatal(e)
	}
	return
}

func TestReserveCommitReleaseAndLimits(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	m := Manager{Store: s}
	limit := 50 * mib
	if e := m.SetDefaultHomeLimit(ctx, "", &limit); e != nil {
		t.Fatal(e)
	}
	r, e := s.Reserve(ctx, "home-a", 30*mib)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Reserve(ctx, "home-a", 30*mib); !errors.As(e, &QuotaExceeded{}) {
		t.Fatal("over-reservation accepted", e)
	}
	if e = s.Commit(ctx, r, 28*mib); e != nil {
		t.Fatal(e)
	}
	if used, reserved := usage(t, s, "home", "home-a"); used != 28*mib || reserved != 0 {
		t.Fatal(used, reserved)
	}
	if used, _ := usage(t, s, "share", ""); used != 28*mib {
		t.Fatal("homes share total", used)
	}
	r, e = s.Reserve(ctx, "home-a", 20*mib)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Release(ctx, r); e != nil {
		t.Fatal(e)
	}
	if used, reserved := usage(t, s, "home", "home-a"); used != 28*mib || reserved != 0 {
		t.Fatal("release", used, reserved)
	}
	// Share total limit applies across homes.
	var share string
	s.DB.QueryRow(ctx, "SELECT id::text FROM storage_shares WHERE key='homes'").Scan(&share)
	var version int64
	s.DB.QueryRow(ctx, "SELECT version FROM storage_shares WHERE id::text=$1", share).Scan(&version)
	total := 40 * mib
	if e = m.SetShareLimit(ctx, "", share, version, &total); e != nil {
		t.Fatal(e)
	}
	if e = m.SetShareLimit(ctx, "", share, version, &total); !errors.Is(e, ErrConflict) {
		t.Fatal("stale version accepted", e)
	}
	_, e = s.Reserve(ctx, "home-b", 15*mib)
	var q QuotaExceeded
	if !errors.As(e, &q) || q.Scope != "share" {
		t.Fatal("share limit not enforced", e)
	}
	// The refused reservation rolled back entirely: no ledger row, nothing held.
	var held int64
	s.DB.QueryRow(ctx, "SELECT coalesce(sum(reserved_bytes),0) FROM storage_quota WHERE scope_id='home-b'").Scan(&held)
	if held != 0 {
		t.Fatal("failed reservation left bytes on the home")
	}
}

func TestConcurrentReservationsNeverExceedLimit(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	limit := 50 * mib
	if e := (Manager{Store: s}).SetDefaultHomeLimit(ctx, "", &limit); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Reserve(ctx, "busy", 10*mib); e == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if _, reserved := usage(t, s, "home", "busy"); ok != 5 || reserved != 50*mib {
		t.Fatal("concurrent reservations", ok, reserved)
	}
}

func TestExpiredReservationAndMeasurement(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	r, e := s.Reserve(ctx, "crash", 10*mib)
	if e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(ctx, "UPDATE storage_reservations SET expires_at=now()-interval '1 minute'")
	if _, e = s.Reserve(ctx, "other", 0); e != nil { // any reservation releases expired ones
		t.Fatal(e)
	}
	if _, reserved := usage(t, s, "home", "crash"); reserved != 0 {
		t.Fatal("expired reservation kept space", reserved)
	}
	if e = s.Commit(ctx, r, 4*mib); e != nil { // late commit: usage counted, nothing subtracted twice
		t.Fatal(e)
	}
	if used, reserved := usage(t, s, "home", "crash"); used != 4*mib || reserved != 0 {
		t.Fatal(used, reserved)
	}
	if need, _ := s.NeedsMeasure(ctx, "fresh"); !need {
		t.Fatal("new homes must be measured first")
	}
	if e = s.SetMeasured(ctx, "crash", 9*mib); e != nil {
		t.Fatal(e)
	}
	if used, _ := usage(t, s, "share", ""); used != 9*mib {
		t.Fatal("share total must follow measurement delta", used)
	}
	if need, _ := s.NeedsMeasure(ctx, "crash"); need {
		t.Fatal("measured home still flagged")
	}
	limit := 5 * mib
	if e = (Manager{Store: s}).SetDefaultHomeLimit(ctx, "", &limit); e != nil { // writes one audit row
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(ctx, "UPDATE storage_events SET action='x'"); e == nil {
		t.Fatal("storage audit must be append-only")
	}
}

// fakeAgent serves the agent's create endpoint on a Unix socket with a fixed status.
func fakeAgent(t *testing.T, status int, code string) AgentClient {
	socket := filepath.Join(t.TempDir(), "agent.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if code != "" {
			fmt.Fprintf(w, `{"code":%q}`, code)
		} else {
			fmt.Fprint(w, `{"path":"/srv/x","referencedBytes":0}`)
		}
	})}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return AgentClient{Socket: socket}
}

func TestCreateShareRollsBackOnAgentFailure(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	s.DB.Exec(ctx, "INSERT INTO storage_volumes(name,kind) VALUES('tank','zfs')")
	m := Manager{Store: s, Agent: fakeAgent(t, 500, "STORAGE_FAILED")}
	if _, e := m.CreateShare(ctx, "", "projects", "Projects", "tank", nil); e == nil {
		t.Fatal("agent failure ignored")
	}
	var n int
	s.DB.QueryRow(ctx, "SELECT count(*) FROM storage_shares WHERE key='projects'").Scan(&n)
	if n != 0 {
		t.Fatal("registry kept a share the agent did not create")
	}
	m.Agent = fakeAgent(t, 201, "")
	share, e := m.CreateShare(ctx, "", "projects", "Projects", "tank", nil)
	if e != nil || share.ID == "" {
		t.Fatal(e)
	}
	if _, e = m.CreateShare(ctx, "", "projects", "Again", "tank", nil); !errors.Is(e, ErrExists) {
		t.Fatal("duplicate key", e)
	}
	for _, bad := range []struct{ key, volume string }{{"homes", "tank"}, {"Bad Key", "tank"}, {"ok", "files"}} {
		if _, e = m.CreateShare(ctx, "", bad.key, "X", bad.volume, nil); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted", bad, e)
		}
	}
	if _, e = m.CreateShare(ctx, "", "nowhere", "X", "missing", nil); !errors.Is(e, ErrNotFound) {
		t.Fatal("unknown volume", e)
	}
}
