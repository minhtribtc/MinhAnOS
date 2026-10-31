package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Favorites (files SQL 007): add is an upsert bounded by MaxFavorites, the API validates root and path, and
// RemoveDeletedUsers drops the favorites of deleted accounts.
func TestFavoritesStoreAndAPI(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("files_favorites_test_%d", time.Now().UnixNano())
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
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY,public_id uuid,status text NOT NULL DEFAULT 'active'); INSERT INTO users(id) VALUES('u1'),('u2'); CREATE TABLE storage_shares(key text PRIMARY KEY,organization_id uuid)"); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	store := SQLFavorites{DB: db}
	if ok, e := store.Add(ctx, "u1", Favorite{Root: "personal", Path: "Docs/a.txt"}); !ok || e != nil {
		t.Fatal("add", ok, e)
	}
	if ok, e := store.Add(ctx, "u1", Favorite{Root: "personal", Path: "Docs/a.txt", Dir: true}); !ok || e != nil {
		t.Fatal("re-add", ok, e)
	}
	if list, e := store.List(ctx, "u1"); e != nil || len(list) != 1 || !list[0].Dir {
		t.Fatal("upsert", list, e)
	}
	if e = store.Remove(ctx, "u1", "personal", "Docs/a.txt"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "INSERT INTO files_favorites(user_id,root,path) SELECT 'u2','personal','f'||g FROM generate_series(1,$1) g", MaxFavorites); e != nil {
		t.Fatal(e)
	}
	if ok, e := store.Add(ctx, "u2", Favorite{Root: "personal", Path: "one-more"}); ok || e != nil {
		t.Fatal("limit", ok, e)
	}

	s := &Server{Favorites: store}
	u := User{ID: strings.Repeat("a", 64)} // the API accepts identity IDs only
	call := func(method, target, body string) (int, string) {
		w := httptest.NewRecorder()
		s.serveFavorites(w, httptest.NewRequest(method, target, strings.NewReader(body)), u)
		return w.Code, w.Body.String()
	}
	if code, body := call("POST", "/api/favorites", `{"root":"personal","path":"Docs","dir":true}`); code != 200 {
		t.Fatal(code, body)
	}
	var page struct{ Items []Favorite }
	_, body := call("GET", "/api/favorites", "")
	if json.Unmarshal([]byte(body), &page); len(page.Items) != 1 || page.Items[0].Path != "Docs" {
		t.Fatal(body)
	}
	if code, body := call("POST", "/api/favorites", `{"root":"share:nope","path":"x"}`); code != 400 || !strings.Contains(body, "FILES_ROOT_INVALID") {
		t.Fatal(code, body)
	}
	if code, body := call("POST", "/api/favorites", `{"root":"personal","path":""}`); code != 400 || !strings.Contains(body, "FILES_FAVORITE_INVALID") {
		t.Fatal(code, body)
	}
	if code, body := call("POST", "/api/favorites/remove", `{"root":"personal","path":"Docs"}`); code != 200 {
		t.Fatal(code, body)
	}
	if _, body = call("GET", "/api/favorites", ""); !strings.Contains(body, `"items":[]`) {
		t.Fatal(body)
	}

	// A deleted account loses its favorites at once (its home waits for the grace period).
	if _, e = db.Exec(ctx, "UPDATE users SET status='deleted' WHERE id='u2'"); e != nil {
		t.Fatal(e)
	}
	s.DataDir = t.TempDir()
	if e = s.RemoveDeletedUsers(ctx, db); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM files_favorites WHERE user_id='u2'").Scan(&n); e != nil || n != 0 {
		t.Fatal("favorites of a deleted account kept", n, e)
	}
}
