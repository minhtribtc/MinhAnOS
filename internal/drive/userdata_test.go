package drive

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/jobcore"
)

// Export (row 17): the archive holds the personal files and folders only, never links, trash or other users.
func TestExportHomeArchive(t *testing.T) {
	s := &Server{DataDir: t.TempDir()}
	me, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	home := filepath.Join(s.DataDir, "users", me)
	for _, dir := range []string{"files/Docs", "trash/x"} {
		if e := os.MkdirAll(filepath.Join(home, dir), 0750); e != nil {
			t.Fatal(e)
		}
	}
	os.WriteFile(filepath.Join(home, "files", "Docs", "a.txt"), []byte("xin chào"), 0640)
	os.WriteFile(filepath.Join(home, "trash", "x", "old.txt"), []byte("old"), 0640)
	os.Symlink("/etc/passwd", filepath.Join(home, "files", "link"))
	os.MkdirAll(filepath.Join(s.DataDir, "users", other, "files"), 0750)
	os.WriteFile(filepath.Join(s.DataDir, "users", other, "files", "secret.txt"), []byte("no"), 0640)
	w := httptest.NewRecorder()
	s.exportHome(w, httptest.NewRequest("GET", "/api/export", nil), User{ID: me, Username: "an"})
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "minhanos-files-an.tar") {
		t.Fatal(w.Code, w.Header())
	}
	tr := tar.NewReader(w.Body)
	names, content := []string{}, ""
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		names = append(names, h.Name)
		if h.Name == "Docs/a.txt" {
			b, _ := io.ReadAll(tr)
			content = string(b)
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "Docs/,Docs/a.txt" || content != "xin chào" {
		t.Fatal("archive", names, content)
	}
}

// Removal (row 17, files SQL 004): a deleted account loses its jobs and replay rows at once and its home
// after the grace period; active accounts are untouched.
func TestRemoveDeletedUsers(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("files_removal_test_%d", time.Now().UnixNano())
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
	gone, kept := strings.Repeat("c", 64), strings.Repeat("d", 64)
	if _, e = db.Exec(ctx, "CREATE TABLE users(id text PRIMARY KEY,public_id uuid,status text NOT NULL); CREATE TABLE storage_shares(key text PRIMARY KEY,organization_id uuid)"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "INSERT INTO users VALUES($1,gen_random_uuid(),'deleted'),($2,gen_random_uuid(),'active')", gone, kept); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	s := &Server{Identity: &SQLIdentity{db: db}, DataDir: t.TempDir()}
	for _, u := range []string{gone, kept} {
		os.MkdirAll(filepath.Join(s.DataDir, "users", u, "files"), 0750)
		os.WriteFile(filepath.Join(s.DataDir, "users", u, "files", "a.txt"), []byte("a"), 0640)
		if _, e = jobcore.Enqueue(ctx, db, jobcore.Spec{Type: "files.copy", Actor: u, Payload: copyPayload{"personal", "a.txt", "b.txt"}}); e != nil {
			t.Fatal(e)
		}
	}
	count := func(user string) (n int) {
		db.QueryRow(ctx, "SELECT count(*) FROM platform_jobs WHERE actor_id=$1", user).Scan(&n)
		return
	}
	exists := func(user string) bool {
		_, err := os.Stat(filepath.Join(s.DataDir, "users", user, "files", "a.txt"))
		return err == nil
	}
	if e = s.RemoveDeletedUsers(ctx, db); e != nil {
		t.Fatal(e)
	}
	if count(gone) != 0 || count(kept) != 1 || !exists(gone) || !exists(kept) {
		t.Fatal("first pass: rows dropped, homes kept for the grace period")
	}
	if _, e = db.Exec(ctx, "UPDATE files_removals SET seen_at=now()-make_interval(days=>$1+1)", RemovalGraceDays); e != nil {
		t.Fatal(e)
	}
	if e = s.RemoveDeletedUsers(ctx, db); e != nil {
		t.Fatal(e)
	}
	var removed bool
	db.QueryRow(ctx, "SELECT removed_at IS NOT NULL FROM files_removals WHERE user_id=$1", gone).Scan(&removed)
	if exists(gone) || !exists(kept) || !removed {
		t.Fatal("after grace: deleted home emptied, active home kept", exists(gone), exists(kept), removed)
	}
}
