package platform

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// First-run setup: a fresh install has no account until the operator (holding the setup code) creates the System
// Admin and names the first organization; afterwards setup is closed, also under concurrent attempts.
func TestFirstRunSetup(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated setup database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	name := fmt.Sprintf("setup_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = root.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	defer root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	config, _ := pgxpool.ParseConfig(dsn)
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	const code = "setup-code-for-tests-123"
	file := filepath.Join(t.TempDir(), "bootstrap")
	os.WriteFile(file, []byte(code+"\n"), 0600)
	t.Setenv("BOOTSTRAP_PASSWORD_FILE", file)
	app, e := New(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	h := app.Handler()
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://example.com")
		r.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	var users int
	db.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users)
	if users != 0 {
		t.Fatal("a fresh install must not create an account by itself", users)
	}
	if w := call("GET", "/api/setup", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"required":true`) {
		t.Fatal("setup status", w.Code, w.Body.String())
	}
	body := func(setupCode, password, org string) string {
		return `{"setupCode":"` + setupCode + `","username":"owner","displayName":"Owner","password":"` + password + `","organizationName":"` + org + `"}`
	}
	for _, c := range []struct {
		body, code string
		status     int
	}{
		{body("wrong-code-000000", "owner-password-123", "Acme"), "SETUP_CODE_INVALID", 403},
		{body(code, "short", "Acme"), "SETUP_WEAK_PASSWORD", 400},
		{body(code, code, "Acme"), "SETUP_WEAK_PASSWORD", 400},
		{body(code, "owner-password-123", " "), "SETUP_INVALID_ORGANIZATION", 400},
		{strings.Replace(body(code, "owner-password-123", "Acme"), `"owner"`, `"Owner!"`, 1), "SETUP_INVALID_USERNAME", 400},
	} {
		if w := call("POST", "/api/setup", c.body); w.Code != c.status || !strings.Contains(w.Body.String(), c.code) {
			t.Fatal("expected", c.code, w.Code, w.Body.String())
		}
	}
	// Cross-origin setup is refused like any other write.
	r := httptest.NewRequest("POST", "http://example.com/api/setup", strings.NewReader(body(code, "owner-password-123", "Acme")))
	r.Header.Set("Origin", "http://attacker.invalid")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin setup", w.Code)
	}
	// Concurrent setups: exactly one wins.
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, done := 0, 0
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := call("POST", "/api/setup", body(" "+code+" ", "owner-password-123", "Acme"))
			mu.Lock()
			defer mu.Unlock()
			switch w.Code {
			case 201:
				created++
			case 409:
				done++
			default:
				t.Error("concurrent setup", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if created != 1 || done != 3 {
		t.Fatal("concurrent setups", created, done)
	}
	if w := call("GET", "/api/setup", ""); !strings.Contains(w.Body.String(), `"required":false`) {
		t.Fatal("setup must close", w.Body.String())
	}
	var role, org, member string
	var events int
	db.QueryRow(ctx, "SELECT role FROM users WHERE username='owner'").Scan(&role)
	db.QueryRow(ctx, "SELECT o.name,m.role FROM identity_organizations o JOIN identity_memberships m ON m.organization_id=o.id JOIN users u ON u.public_id=m.user_id WHERE o.is_default AND u.username='owner'").Scan(&org, &member)
	db.QueryRow(ctx, "SELECT count(*) FROM identity_admin_events WHERE action='setup.completed' AND organization_id IS NOT NULL").Scan(&events)
	if role != "admin" || org != "Acme" || member != "admin" || events != 1 {
		t.Fatal("setup result", role, org, member, events)
	}
	if w := call("POST", "/api/login", `{"username":"owner","password":"owner-password-123"}`); w.Code != 200 {
		t.Fatal("System Admin signs in after setup", w.Code, w.Body.String())
	}
	// The setup code never becomes a sign-in password.
	if w := call("POST", "/api/login", `{"username":"admin","password":"`+code+`"}`); w.Code != 401 {
		t.Fatal("bootstrap admin exists after setup", w.Code)
	}
}
