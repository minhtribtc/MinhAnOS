package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/storagecore"
)

// Tenancy over HTTP (ADR 0008, app standard row 27): an organization admin manages only its own shares and
// sees only its own audit; another organization's resources read as not found, by list and by ID.
func TestTenancyStorageAndLogsOverHTTP(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated tenancy database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	name := fmt.Sprintf("tenancy_%d", time.Now().UnixNano())
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
	const pass = "tenancy-test-password-123"
	file := filepath.Join(t.TempDir(), "bootstrap")
	os.WriteFile(file, []byte(pass), 0600)
	t.Setenv("BOOTSTRAP_PASSWORD_FILE", file)
	t.Setenv("MINHANOS_BOOTSTRAP_ADMIN", "true")
	app, e := New(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	socket := filepath.Join(t.TempDir(), "agent.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	agent := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		fmt.Fprint(w, `{"path":"/srv/x","referencedBytes":0}`)
	})}
	go agent.Serve(l)
	defer agent.Close()
	app.storage.Agent = storagecore.AgentClient{Socket: socket}
	h := app.Handler()

	type session struct{ cookie, csrf, token string }
	call := func(s session, method, path, body string, headers ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://example.com")
		r.Header.Set("Cookie", s.cookie)
		r.Header.Set("X-CSRF-Token", s.csrf)
		for i := 0; i+1 < len(headers); i += 2 {
			r.Header.Set(headers[i], headers[i+1])
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := func(user string) session {
		w := call(session{}, "POST", "/api/login", `{"username":"`+user+`","password":"`+pass+`"}`)
		if w.Code != 200 {
			t.Fatal("login", user, w.Code, w.Body.String())
		}
		var out struct {
			CSRF string `json:"csrfToken"`
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		c := w.Result().Cookies()[0]
		if e := app.authCore.Reauthenticate(ctx, identityschema.DesktopRealm, c.Value, pass); e != nil {
			t.Fatal(e)
		}
		return session{c.Name + "=" + c.Value, out.CSRF, c.Value}
	}
	key := func() string { return fmt.Sprintf("tenancy-key-%d", time.Now().UnixNano()) }
	decode := func(w *httptest.ResponseRecorder, v any) {
		t.Helper()
		if e := json.Unmarshal(w.Body.Bytes(), v); e != nil {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	admin := login("admin")
	org := func(name string) string {
		w := call(admin, "POST", "/api/v1/identity/organizations", `{"name":"`+name+`","parentId":null,"allowChildren":false}`, "Idempotency-Key", key())
		if w.Code != 201 {
			t.Fatal("create org", w.Code, w.Body.String())
		}
		var o struct{ ID string }
		decode(w, &o)
		w = call(admin, "POST", "/api/v1/identity/organizations/"+o.ID+"/users", `{"username":"`+strings.ToLower(name)+`_admin","displayName":"`+name+`","password":"`+pass+`","role":"admin"}`, "Idempotency-Key", key())
		if w.Code != 201 {
			t.Fatal("create org user", w.Code, w.Body.String())
		}
		return o.ID
	}
	acme, beta := org("Acme"), org("Beta")
	acmeAdmin, betaAdmin := login("acme_admin"), login("beta_admin")
	if _, e = db.Exec(ctx, "INSERT INTO storage_volumes(name,kind) VALUES('tank','zfs')"); e != nil {
		t.Fatal(e)
	}

	// Top-level allocations are Root's: an organization admin cannot grant itself capacity.
	if w := call(acmeAdmin, "POST", "/api/v1/storage/organizations/"+acme+"/allocations", `{"volume":"tank","limitBytes":1048576}`, "Idempotency-Key", key()); w.Code != 404 {
		t.Fatal("self allocation", w.Code, w.Body.String())
	}
	for _, o := range []string{acme, beta} {
		if w := call(admin, "POST", "/api/v1/storage/organizations/"+o+"/allocations", `{"volume":"tank","limitBytes":104857600}`, "Idempotency-Key", key()); w.Code != 201 {
			t.Fatal("allocation", w.Code, w.Body.String())
		}
	}
	share := func(s session, org, k string) *httptest.ResponseRecorder {
		return call(s, "POST", "/api/v1/storage/shares", `{"key":"`+k+`","name":"`+k+`","volume":"tank","limitBytes":null,"hostsHomes":false,"organizationId":"`+org+`"}`, "Idempotency-Key", key())
	}
	w := share(acmeAdmin, acme, "acme-docs")
	if w.Code != 201 {
		t.Fatal("org admin share", w.Code, w.Body.String())
	}
	var acmeShare storagecore.Share
	decode(w, &acmeShare)
	if w = share(betaAdmin, acme, "beta-into-acme"); w.Code != 403 {
		t.Fatal("cross-tenant share create", w.Code)
	}
	if w = share(betaAdmin, beta, "beta-docs"); w.Code != 201 {
		t.Fatal("beta share", w.Code, w.Body.String())
	}

	// Lists are scoped; Root sees both.
	names := func(s session) []string {
		var out struct{ Items []storagecore.Share }
		decode(call(s, "GET", "/api/v1/storage/shares", ""), &out)
		list := []string{}
		for _, x := range out.Items {
			list = append(list, x.Key)
		}
		return list
	}
	if got := strings.Join(names(betaAdmin), ","); got != "beta-docs" {
		t.Fatal("beta admin lists", got)
	}
	if got := strings.Join(names(admin), ","); !strings.Contains(got, "acme-docs") || !strings.Contains(got, "beta-docs") {
		t.Fatal("root lists", got)
	}
	// IDOR: another organization's share by ID reads as not found for every route.
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/v1/storage/shares/" + acmeShare.ID + "/access", ""},
		{"GET", "/api/v1/storage/shares/" + acmeShare.ID + "/folders", ""},
		{"PUT", "/api/v1/storage/shares/" + acmeShare.ID + "/quota", `{"limitBytes":1}`},
		{"PUT", "/api/v1/storage/shares/" + acmeShare.ID + "/access", `{"entries":[]}`},
		{"GET", "/api/v1/storage/subjects?organization=" + acme, ""},
	} {
		if w = call(betaAdmin, r.method, r.path, r.body, "If-Match", `"1"`); w.Code != 404 {
			t.Fatal("IDOR", r.method, r.path, w.Code, w.Body.String())
		}
	}
	// Platform-only areas stay Root's.
	for _, p := range []string{"/api/v1/storage/homes", "/api/v1/storage/settings"} {
		if w = call(acmeAdmin, "GET", p, ""); w.Code != 403 {
			t.Fatal("platform area for org admin", p, w.Code)
		}
	}
	// A permission for a non-member is refused even by the owning admin.
	var betaUser string
	db.QueryRow(ctx, "SELECT public_id::text FROM users WHERE username='beta_admin'").Scan(&betaUser)
	if w = call(acmeAdmin, "PUT", "/api/v1/storage/shares/"+acmeShare.ID+"/access", `{"entries":[{"subjectType":"user","subjectId":"`+betaUser+`","access":"write"}]}`, "If-Match", fmt.Sprintf(`"%d"`, acmeShare.Version)); w.Code != 400 {
		t.Fatal("non-member permission", w.Code, w.Body.String())
	}

	// Log Center: scoped to the admin's organizations; another organization by filter is not found.
	var page struct {
		Items []struct {
			Action         string  `json:"action"`
			OrganizationID *string `json:"organizationId"`
		}
	}
	if w = call(betaAdmin, "GET", "/api/v1/logs/events?limit=100", ""); w.Code != 200 {
		t.Fatal("org admin logs", w.Code, w.Body.String())
	}
	decode(w, &page)
	if len(page.Items) == 0 {
		t.Fatal("org admin sees no events of its own organization")
	}
	for _, ev := range page.Items {
		if ev.OrganizationID == nil || *ev.OrganizationID != beta {
			t.Fatal("foreign or platform event leaked", ev.Action)
		}
	}
	if w = call(betaAdmin, "GET", "/api/v1/logs/events?organization="+acme, ""); w.Code != 404 {
		t.Fatal("foreign organization logs", w.Code)
	}
	// The launcher shows Storage Manager and Log Center to organization admins.
	var apps []struct{ ID string }
	decode(call(betaAdmin, "GET", "/api/apps", ""), &apps)
	seen := map[string]bool{}
	for _, a := range apps {
		seen[a.ID] = true
	}
	if !seen[identityschema.StorageApp] || !seen[identityschema.LogsApp] {
		t.Fatal("scoped apps hidden from org admin", apps)
	}
}
