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
	_ = csrf
}
