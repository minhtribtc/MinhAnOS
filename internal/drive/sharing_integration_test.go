package drive

import (
	"context"
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
	"minhanos.local/platform/internal/authn"
)

func TestSharingStoreAndPublicAPI(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("files_sharing_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = root.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
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
	t.Cleanup(func() { db.Close(); root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); root.Close() })

	owner, bob, eve := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("e", 64)
	ownerPub, bobPub, evePub := "00000000-0000-4000-8000-00000000000a", "00000000-0000-4000-8000-00000000000b", "00000000-0000-4000-8000-00000000000e"
	org1, org2 := "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"
	if _, e = db.Exec(ctx, `CREATE TABLE users(id text PRIMARY KEY, username text, display_name text, public_id uuid, status text NOT NULL DEFAULT 'active');
		CREATE TABLE identity_organizations(id uuid PRIMARY KEY, status text NOT NULL DEFAULT 'active');
		CREATE TABLE identity_memberships(organization_id uuid, user_id uuid);
		CREATE TABLE storage_shares(key text PRIMARY KEY, organization_id uuid)`); e != nil {
		t.Fatal(e)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users(id,username,display_name,public_id) VALUES ($1,'owner','Owner Name',$4),($2,'bob','Bob Name',$5),($3,'eve','Eve Name',$6)", []any{owner, bob, eve, ownerPub, bobPub, evePub}},
		{"INSERT INTO identity_organizations(id) VALUES ($1),($2)", []any{org1, org2}},
		{"INSERT INTO identity_memberships(organization_id,user_id) VALUES ($1,$2),($1,$3),($4,$5)", []any{org1, ownerPub, bobPub, org2, evePub}},
	} {
		if _, e = db.Exec(ctx, q.sql, q.args...); e != nil {
			t.Fatal(e)
		}
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}

	sh := SQLSharing{DB: db}
	if list, e := sh.Recipients(ctx, owner, ""); e != nil || len(list) != 1 || list[0].ID != bobPub {
		t.Fatal("recipients", list, e)
	}
	g, e := sh.AddGrant(ctx, owner, Grant{Path: "Docs", Dir: true, Grantee: bobPub, Permission: "view"})
	if e != nil || g.ID == "" || g.Permission != "view" {
		t.Fatal("grant", g, e)
	}
	g2, e := sh.AddGrant(ctx, owner, Grant{Path: "Docs", Dir: true, Grantee: bobPub, Permission: "edit"})
	if e != nil || g2.ID != g.ID || g2.Permission != "edit" {
		t.Fatal("grant update", g2, e)
	}
	var n int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM files_grants WHERE owner_id=$1", owner).Scan(&n); e != nil || n != 1 {
		t.Fatal("grant rows", n, e)
	}
	if _, e = sh.AddGrant(ctx, owner, Grant{Path: "Docs", Dir: true, Grantee: evePub, Permission: "view"}); e != errNoRecipient {
		t.Fatal("eve grant", e)
	}
	if list, e := sh.Received(ctx, bob); e != nil || len(list) != 1 || list[0].Owner != "Owner Name" {
		t.Fatal("received", list, e)
	}
	if got, ok, e := sh.ReceivedGrant(ctx, g.ID, bob); e != nil || !ok || got.ID != g.ID {
		t.Fatal("received grant", got, ok, e)
	}
	if _, e = db.Exec(ctx, "DELETE FROM identity_memberships WHERE user_id=$1", bobPub); e != nil {
		t.Fatal(e)
	}
	if _, ok, e := sh.ReceivedGrant(ctx, g.ID, bob); e != nil || ok {
		t.Fatal("membership removed but grant visible", ok, e)
	}
	if _, e = db.Exec(ctx, "INSERT INTO identity_memberships(organization_id,user_id) VALUES ($1,$2)", org1, bobPub); e != nil {
		t.Fatal(e)
	}

	hash, e := authn.Hash("secret1")
	if e != nil {
		t.Fatal(e)
	}
	l, e := sh.AddLink(ctx, owner, Link{Path: "Docs", Dir: true, Permission: "drop"}, hash)
	if e != nil || !l.HasPassword || len(l.Token) != 64 {
		t.Fatal("link", l, e)
	}
	l2, ok, e := sh.LinkByToken(ctx, l.Token)
	if e != nil || !ok || !authn.Verify("secret1", l2.hash) {
		t.Fatal("link by token", l2, ok, e)
	}
	empty := ""
	l3, ok, e := sh.UpdateLink(ctx, owner, Link{ID: l.ID, Permission: "view"}, &empty)
	if e != nil || !ok || l3.HasPassword {
		t.Fatal("remove password", l3, ok, e)
	}
	if _, e = sh.AddLink(ctx, owner, Link{Path: "Docs/a.txt", Dir: false, Permission: "drop"}, ""); e == nil {
		t.Fatal("drop on file accepted")
	}

	fav := SQLFavorites{DB: db}
	if ok, e := fav.Add(ctx, owner, Favorite{Root: "personal", Path: "Docs/a.txt"}); !ok || e != nil {
		t.Fatal("favorite", ok, e)
	}
	if e = sh.Retarget(ctx, owner, "personal", "Docs", "Work"); e != nil {
		t.Fatal(e)
	}
	if list, e := sh.Grants(ctx, owner, ""); e != nil || len(list) != 1 || list[0].Path != "Work" {
		t.Fatal("grant retarget", list, e)
	}
	if list, e := sh.Links(ctx, owner, ""); e != nil || len(list) != 1 || list[0].Path != "Work" {
		t.Fatal("link retarget", list, e)
	}
	if list, e := fav.List(ctx, owner); e != nil || len(list) != 1 || list[0].Path != "Work/a.txt" {
		t.Fatal("favorite retarget", list, e)
	}
	if ok, e := fav.Add(ctx, bob, Favorite{Root: "shared:" + g.ID, Path: "Work/a.txt"}); !ok || e != nil {
		t.Fatal("bob shared favorite", ok, e)
	}
	if e = sh.Retarget(ctx, owner, "personal", "Work", ""); e != nil {
		t.Fatal(e)
	}
	if list, e := sh.Grants(ctx, owner, ""); e != nil || len(list) != 0 {
		t.Fatal("grants not deleted", list, e)
	}
	if list, e := sh.Links(ctx, owner, ""); e != nil || len(list) != 0 {
		t.Fatal("links not deleted", list, e)
	}
	if list, e := fav.List(ctx, owner); e != nil || len(list) != 0 {
		t.Fatal("owner favorite not deleted", list, e)
	}
	if list, e := fav.List(ctx, bob); e != nil || len(list) != 0 {
		t.Fatal("shared favorite not deleted", list, e)
	}

	s := &Server{Sharing: SQLSharing{DB: db}, Identity: &SQLIdentity{db: db}, DataDir: t.TempDir()}
	pubDir := filepath.Join(s.DataDir, "users", owner, "files", "Pub")
	if e = os.MkdirAll(pubDir, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(pubDir, "hello.txt"), []byte("hi"), 0644); e != nil {
		t.Fatal(e)
	}
	lpub, e := sh.AddLink(ctx, owner, Link{Path: "Pub", Dir: true, Permission: "view"}, "")
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, target, nil)
		} else {
			r = httptest.NewRequest(method, target, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		}
		s.servePublic(w, r)
		return w
	}
	w := call("GET", "/api/public/"+lpub.Token, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"needsPassword":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/api/public/"+lpub.Token+"/files/list?path=", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hello.txt") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/api/public/"+lpub.Token+"/files/download?path=hello.txt", "")
	if w.Code != 200 || w.Body.String() != "hi" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/api/public/"+lpub.Token+"/files/upload?path=&name=x.txt", "")
	if w.Code != 403 || !strings.Contains(w.Body.String(), "FILES_LINK_FORBIDDEN") {
		t.Fatal(w.Code, w.Body.String())
	}

	hash2, e := authn.Hash("right1")
	if e != nil {
		t.Fatal(e)
	}
	lpw, e := sh.AddLink(ctx, owner, Link{Path: "Pub", Dir: true, Permission: "view"}, hash2)
	if e != nil {
		t.Fatal(e)
	}
	w = call("GET", "/api/public/"+lpw.Token+"/files/list?path=", "")
	if w.Code != 401 || !strings.Contains(w.Body.String(), "FILES_LINK_PASSWORD_REQUIRED") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/api/public/"+lpw.Token+"/unlock", `{"password":"wrong1"}`)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "FILES_LINK_PASSWORD_WRONG") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/api/public/"+lpw.Token+"/unlock", `{"password":"right1"}`)
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		t.Fatal(w.Code, w.Body.String(), w.Header().Get("Set-Cookie"))
	}
	cookie := w.Result().Cookies()[0]
	req := httptest.NewRequest("GET", "/api/public/"+lpw.Token+"/files/list?path=", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.servePublic(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hello.txt") {
		t.Fatal(w.Code, w.Body.String())
	}
}
