package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// recordingIdentity keeps every audit event so the test can see what each level records.
type recordingIdentity struct {
	fakeIdentity
	mu     *sync.Mutex
	events *[]AuditEvent
}

func (f recordingIdentity) Audit(_ context.Context, _ User, e AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.events = append(*f.events, e)
	return nil
}

type memActivity struct {
	level   string
	version int
}

func (m *memActivity) Level(context.Context) (string, int, error) { return m.level, m.version, nil }
func (m *memActivity) SetLevel(_ context.Context, level string, version int, _ string) (bool, error) {
	if version != m.version {
		return false, nil
	}
	m.level, m.version = level, m.version+1
	return true, nil
}

func TestActivityLevels(t *testing.T) {
	var mu sync.Mutex
	var events []AuditEvent
	id := recordingIdentity{fakeIdentity: fakeIdentity{role: "admin"}, mu: &mu, events: &events}
	s := &Server{Identity: id, Activity: &memActivity{level: LevelMinimal, version: 1}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
	h := s.Handler()
	call := func(method, target, body string, header ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://drive.test"+target, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
		r.Header.Set("X-CSRF-Token", "csrf")
		r.Header.Set("Idempotency-Key", "k-"+method+target+body+strings.Join(header, ""))
		r.Header.Set("User-Agent", "activity-test")
		for i := 0; i+1 < len(header); i += 2 {
			r.Header.Set(header[i], header[i+1])
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	take := func() []AuditEvent {
		mu.Lock()
		defer mu.Unlock()
		out := events
		events = nil
		return out
	}

	// minimal: reads leave nothing, a write is recorded without its name.
	if w := call("GET", "/api/files/list?root=personal&path=", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/files/mkdir?root=personal&path=", `{"name":"Docs"}`); w.Code >= 300 {
		t.Fatal(w.Code, w.Body.String())
	}
	if ev := take(); len(ev) != 1 || ev[0].Action != "files.mkdir" || ev[0].Path != "" || ev[0].Details["name"] != nil {
		t.Fatalf("minimal: %+v", ev)
	}
	var roots struct {
		ActivityLevel string `json:"activityLevel"`
	}
	if w := call("GET", "/api/roots", ""); json.Unmarshal(w.Body.Bytes(), &roots) != nil || roots.ActivityLevel != LevelMinimal {
		t.Fatal("roots level", w.Body.String())
	}

	// The level needs If-Match with the current version, and the change itself is recorded.
	if w := call("GET", "/api/activity/settings", ""); w.Code != 200 || w.Header().Get("ETag") != `"1"` {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	if w := call("PUT", "/api/activity/settings", `{"level":"access"}`); w.Code != 428 {
		t.Fatal("no If-Match", w.Code)
	}
	if w := call("PUT", "/api/activity/settings", `{"level":"loud"}`, "If-Match", `"1"`); w.Code != 400 {
		t.Fatal("bad level", w.Code)
	}
	if w := call("PUT", "/api/activity/settings", `{"level":"access"}`, "If-Match", `"7"`); w.Code != 412 {
		t.Fatal("stale version", w.Code)
	}
	if w := call("PUT", "/api/activity/settings", `{"level":"access"}`, "If-Match", `"1"`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if ev := take(); len(ev) != 1 || ev[0].Action != "files.activity_level" || ev[0].Details["to"] != LevelAccess {
		t.Fatalf("level change: %+v", ev)
	}

	// access: folders opened and files read are recorded with their path; a failed read with its outcome and code.
	if w := call("POST", "/api/files/upload?root=personal&path=Docs&name=a.txt", "hello"); w.Code >= 300 {
		t.Fatal(w.Code, w.Body.String())
	}
	call("GET", "/api/files/list?root=personal&path=Docs", "")
	call("GET", "/api/files/text?root=personal&path=Docs/a.txt", "")
	call("GET", "/api/files/download?root=personal&path=Docs/missing.txt", "")
	call("GET", "/api/files/search?root=personal&path=&q=a.t", "")
	ev := take()
	got := map[string]AuditEvent{}
	for _, e := range ev {
		got[e.Action] = e
	}
	if e := got["files.upload"]; e.Path != "Docs" || e.Details["name"] != "a.txt" {
		t.Fatalf("upload: %+v", ev)
	}
	if e := got["files.list"]; e.Path != "Docs" || e.Outcome != "success" {
		t.Fatalf("list: %+v", ev)
	}
	if e := got["files.text"]; e.Path != "Docs/a.txt" || e.Details["bytes"] == nil {
		t.Fatalf("text: %+v", ev)
	}
	if e := got["files.download"]; e.Outcome != "failure" || e.Details["code"] == "" {
		t.Fatalf("failed download: %+v", ev)
	}
	if e := got["files.search"]; e.Details["query"] != "a.t" {
		t.Fatalf("search: %+v", ev)
	}
	if e := got["files.list"]; e.Details["client"] != nil {
		t.Fatal("client recorded below full", e.Details)
	}
	var live struct {
		Level string      `json:"level"`
		Items []liveEntry `json:"items"`
	}
	if w := call("GET", "/api/activity/live", ""); json.Unmarshal(w.Body.Bytes(), &live) != nil || live.Level != LevelAccess || len(live.Items) != 0 {
		t.Fatal("live below full", w.Body.String())
	}
	if w := call("POST", "/api/activity/presence", `{"root":"personal","path":"Docs"}`); !strings.Contains(w.Body.String(), `"stored":false`) {
		t.Fatal("presence below full", w.Body.String())
	}

	// full: client details, and who has what open.
	if w := call("PUT", "/api/activity/settings", `{"level":"full"}`, "If-Match", `"2"`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	take()
	call("GET", "/api/files/list?root=personal&path=Docs", "")
	if ev := take(); len(ev) != 1 || ev[0].Details["agent"] != "activity-test" || ev[0].Details["client"] == nil || ev[0].Details["ms"] == nil {
		t.Fatalf("full list: %+v", ev)
	}
	w := call("GET", "/api/activity/live", "")
	if json.Unmarshal(w.Body.Bytes(), &live) != nil || len(live.Items) != 1 || live.Items[0].Kind != "view" || live.Items[0].Path != "Docs" || live.Items[0].User != "test" {
		t.Fatal("live view", w.Body.String())
	}
	if w := call("POST", "/api/activity/presence", `{"root":"personal","rootName":"Home","path":"Docs","file":"Docs/a.txt"}`); !strings.Contains(w.Body.String(), `"stored":true`) {
		t.Fatal("presence", w.Body.String())
	}
	w = call("GET", "/api/activity/live", "")
	if json.Unmarshal(w.Body.Bytes(), &live) != nil || len(live.Items) != 1 || live.Items[0].Kind != "open" || live.Items[0].File != "Docs/a.txt" {
		t.Fatal("live open", w.Body.String())
	}

	// Only the System Admin sees who is where and changes the level.
	user := &Server{Identity: fakeIdentity{role: "user"}, Activity: &memActivity{level: LevelFull, version: 1}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
	for _, target := range []string{"/api/activity/live", "/api/activity/settings"} {
		r := httptest.NewRequest("GET", "http://drive.test"+target, nil)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
		rw := httptest.NewRecorder()
		user.Handler().ServeHTTP(rw, r)
		if rw.Code != 403 {
			t.Fatal(target, rw.Code)
		}
	}
}

func TestSQLActivityLevel(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("files_activity_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = root.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); root.Close() })
	if _, err = db.Exec(ctx, `CREATE TABLE users(id text PRIMARY KEY, username text, display_name text, public_id uuid, status text NOT NULL DEFAULT 'active');
		CREATE TABLE identity_organizations(id uuid PRIMARY KEY, status text NOT NULL DEFAULT 'active');
		CREATE TABLE identity_memberships(organization_id uuid, user_id uuid);
		CREATE TABLE storage_shares(key text PRIMARY KEY, organization_id uuid)`); err != nil {
		t.Fatal(err)
	}
	if err = ApplySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	a := SQLActivity{DB: db}
	if level, version, err := a.Level(ctx); err != nil || level != LevelMinimal || version != 1 {
		t.Fatal(level, version, err)
	}
	if ok, err := a.SetLevel(ctx, LevelFull, 1, "nobody"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := a.SetLevel(ctx, LevelAccess, 1, "nobody"); err != nil || ok {
		t.Fatal("stale version accepted", ok, err)
	}
	if level, version, _ := a.Level(ctx); level != LevelFull || version != 2 {
		t.Fatal(level, version)
	}
	// Events carry their outcome, and paths are allowed in every root now.
	if _, err := db.Exec(ctx, `INSERT INTO files_events(request_id,action,root,outcome,details) VALUES('r','files.list','personal','denied','{"path":"Docs"}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO files_events(request_id,action,root,outcome,details) VALUES('r','files.list','personal','maybe','{}')`); err == nil {
		t.Fatal("unknown outcome accepted")
	}
}
