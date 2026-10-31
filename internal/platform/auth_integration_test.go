package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/drive"
	identityschema "minhanos.local/platform/internal/identity"
)

func TestAuthAdaptersAcrossHTTPRealms(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated adapter database not configured")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	name := fmt.Sprintf("auth_adapter_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	const pass = "adapter-test-password-123"
	file := filepath.Join(t.TempDir(), "bootstrap")
	if e = os.WriteFile(file, []byte(pass), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("BOOTSTRAP_PASSWORD_FILE", file)
	app, e := New(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	store, e := drive.NewSQLIdentity(ctx, db, file)
	if e != nil {
		t.Fatal(e)
	}
	driveHandler := (&drive.Server{Identity: store, AllowPlatformSessions: true, StaticDir: t.TempDir(), DataDir: t.TempDir()}).Handler()
	driveServer := httptest.NewServer(driveHandler)
	defer driveServer.Close()
	t.Setenv("DRIVE_INTERNAL_URL", driveServer.URL)
	desktop := app.Handler()
	call := func(h http.Handler, path, method, cookie, csrf, origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", cookie)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := func(h http.Handler, old string) (string, string) {
		w := call(h, "/api/login", "POST", old, "", "http://example.com", `{"username":"admin","password":"`+pass+`"}`)
		if w.Code != 200 {
			t.Fatalf("login: %d %s", w.Code, w.Body.String())
		}
		var result struct {
			CSRF string `json:"csrfToken"`
		}
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
		c := w.Result().Cookies()[0]
		if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
			t.Fatal("cookie protections lost")
		}
		return c.Name + "=" + c.Value, result.CSRF
	}
	dc, csrf := login(desktop, "")
	fc, _ := login(driveHandler, "")
	for _, h := range []http.Handler{desktop, driveHandler} {
		w := call(h, "/api/login", "POST", "", "", "http://attacker.invalid", `{"username":"admin","password":"`+pass+`"}`)
		if w.Code != 403 {
			t.Fatal("cross-origin login", w.Code)
		}
	}
	if w := call(desktop, "/api/logout", "POST", dc, "", "http://example.com", "{}"); w.Code != 403 {
		t.Fatal("csrf missing", w.Code)
	}
	if w := call(driveHandler, "/api/logout", "POST", fc, "", "http://example.com", "{}"); w.Code != 403 {
		t.Fatal("drive csrf missing", w.Code)
	}
	if w := call(desktop, "/api/drive/session", "GET", dc, "", "", ""); w.Code != 200 {
		t.Fatal("native gateway", w.Code, w.Body.String())
	}
	oldDC, oldFC := dc, fc
	dc, csrf = login(desktop, dc)
	fc, _ = login(driveHandler, fc)
	if w := call(desktop, "/api/session", "GET", oldDC, "", "", ""); w.Code != 401 {
		t.Fatal("desktop fixation", w.Code)
	}
	if w := call(driveHandler, "/api/session", "GET", oldFC, "", "", ""); w.Code != 401 {
		t.Fatal("drive fixation", w.Code)
	}
	// The legacy /api/admin/users adapter is gone; creating an account goes through the v1 identity API,
	// which needs a recent reauthentication, and a web System Admin never inherits host privilege.
	if w := call(desktop, "/api/admin/users", "GET", dc, "", "", ""); w.Code != 404 {
		t.Fatal("legacy admin API still served", w.Code)
	}
	v1 := func(path, method, body, match string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", dc)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Origin", "http://example.com")
		r.Header.Set("Idempotency-Key", token())
		if match != "" {
			r.Header.Set("If-Match", match)
		}
		w := httptest.NewRecorder()
		desktop.ServeHTTP(w, r)
		return w
	}
	createBody := `{"username":"web_admin","displayName":"Web Admin","password":"adapter-test-password-123"}`
	if w := v1("/api/v1/identity/users", "POST", createBody, ""); w.Code != 403 {
		t.Fatal("create bypassed reauth", w.Code, w.Body.String())
	}
	if w := call(desktop, "/api/v1/identity/reauthenticate", "POST", dc, csrf, "http://example.com", `{"password":"adapter-test-password-123"}`); w.Code != 200 {
		t.Fatal("reauth endpoint", w.Code, w.Body.String())
	}
	w := v1("/api/v1/identity/users", "POST", createBody, "")
	if w.Code != 201 {
		t.Fatal("v1 create", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	if e = json.Unmarshal(w.Body.Bytes(), &created); e != nil {
		t.Fatal(e)
	}
	if w = v1("/api/v1/identity/users/"+created.ID+"/role", "PUT", `{"role":"system-admin"}`, w.Header().Get("ETag")); w.Code != 200 {
		t.Fatal("promote", w.Code, w.Body.String())
	}
	var webAdmin User
	if e = db.QueryRow(ctx, "SELECT id FROM users WHERE username='web_admin'").Scan(&webAdmin.ID); e != nil {
		t.Fatal(e)
	}
	// Preferences are versioned: GET gives an ETag, PUT needs it as If-Match and returns the next one.
	prefs := call(desktop, "/api/preferences", "GET", dc, "", "", "")
	first := prefs.Header().Get("ETag")
	if prefs.Code != 200 || first == "" {
		t.Fatal("preferences etag", prefs.Code)
	}
	put := func(match string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "http://example.com/api/preferences", strings.NewReader(prefs.Body.String()))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", dc)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Origin", "http://example.com")
		if match != "" {
			r.Header.Set("If-Match", match)
		}
		w := httptest.NewRecorder()
		desktop.ServeHTTP(w, r)
		return w
	}
	if w := put(""); w.Code != 428 {
		t.Fatal("preferences without If-Match", w.Code)
	}
	if w := put(first); w.Code != 200 || w.Header().Get("ETag") == "" {
		t.Fatal("preferences save", w.Code, w.Body.String())
	}
	if _, e = db.Exec(ctx, "UPDATE preferences SET value=jsonb_set(value,'{notes}','\"other tab\"')"); e != nil {
		t.Fatal(e)
	}
	if w := put(first); w.Code != 412 {
		t.Fatal("stale preferences accepted", w.Code)
	}
	// Row 17: the user exports their own Desktop data; a deleted account's preferences are removed.
	if export := call(desktop, "/api/export", "GET", dc, "", "", ""); export.Code != 200 || !strings.Contains(export.Body.String(), `"notes":"other tab"`) || !strings.Contains(export.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("export", export.Code, export.Body.String())
	}
	if allowed, e := app.identityService.CanAccessHost(ctx, webAdmin.ID); e != nil || allowed {
		t.Fatal("web admin received host privilege", e)
	}
	// Reinitialization must not treat a later web admin as a pre-upgrade operator.
	again, e := drive.NewSQLIdentity(ctx, db, file)
	if e != nil {
		t.Fatal(e)
	}
	if allowed, e := again.CanAccessHost(ctx, drive.User{ID: webAdmin.ID, Role: "admin"}); e != nil || allowed {
		t.Fatal("startup promoted operator", e)
	}
	if _, e = db.Exec(ctx, "UPDATE identity_app_grants SET state='revoked' WHERE permission_key='identity.users.read'"); e != nil {
		t.Fatal(e)
	}
	if w := call(desktop, "/api/v1/identity/users", "GET", dc, "", "", ""); w.Code != 403 {
		t.Fatal("v1 read bypassed revoked grant", w.Code)
	}
	if _, e = db.Exec(ctx, "UPDATE identity_app_grants SET state='enabled' WHERE permission_key='identity.users.read'"); e != nil {
		t.Fatal(e)
	}
	raw := strings.SplitN(dc, "=", 2)[1]
	if e = app.authCore.ChangePassword(ctx, identityschema.DesktopRealm, raw, pass, "replacement-password-123"); e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		h            http.Handler
		path, cookie string
	}{{desktop, "/api/session", dc}, {desktop, "/api/drive/session", dc}, {driveHandler, "/api/session", fc}} {
		if w := call(test.h, test.path, "GET", test.cookie, "", "", ""); w.Code != 401 {
			t.Fatal("credential mutation did not revoke adapter", test.path, w.Code)
		}
	}
	// Row 17: a deleted account's Desktop data is removed; active accounts keep theirs (runs last: it deletes a user).
	var owner string
	if e = db.QueryRow(ctx, "SELECT user_id FROM preferences LIMIT 1").Scan(&owner); e != nil {
		t.Fatal(e)
	}
	if n, e := RemoveDeletedUsers(ctx, db); e != nil || n != 0 {
		t.Fatal("active user data removed", n, e)
	}
	if _, e = db.Exec(ctx, "UPDATE users SET status='deleted' WHERE id=$1", owner); e != nil {
		t.Fatal(e)
	}
	if n, e := RemoveDeletedUsers(ctx, db); e != nil || n < 1 {
		t.Fatal("deleted user data kept", n, e)
	}
	_ = csrf
}
