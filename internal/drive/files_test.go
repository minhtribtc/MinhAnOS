package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"minhanos.local/platform/internal/identity"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"files", "trash"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return &Engine{Root: root}
}
func TestPrivateFilesLifecycle(t *testing.T) {
	e := testEngine(t)
	if err := e.Mkdir("", "Documents"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Upload("Documents", "hello.txt", strings.NewReader("xin chào")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Upload("Documents", "hello.txt", strings.NewReader("overwrite")); err == nil {
		t.Fatal("overwritten existing file")
	}
	content, tag, err := e.Text("Documents/hello.txt")
	if err != nil || content != "xin chào" {
		t.Fatal(content, err)
	}
	if _, err = e.SaveText("Documents/hello.txt", "changed", "stale"); err == nil {
		t.Fatal("accepted stale etag")
	}
	backup, err := e.SaveText("Documents/hello.txt", "changed", tag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(backup, "minhanos-backup") {
		t.Fatal(backup)
	}
	if err = e.Rename("Documents/hello.txt", "Documents/moved.txt"); err != nil {
		t.Fatal(err)
	}
	item, err := e.Trash("Documents/moved.txt")
	if err != nil {
		t.Fatal(err)
	}
	items, err := e.TrashList()
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	if err = e.Restore(item.ID); err != nil {
		t.Fatal(err)
	}
	content, _, err = e.Text("Documents/moved.txt")
	if err != nil || content != "changed" {
		t.Fatal(content, err)
	}
}

func TestCopyCannotOverwriteAndKeepsSource(t *testing.T) {
	e := testEngine(t)
	_, err := e.Upload("", "a.txt", strings.NewReader("copy me"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Copy("a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Copy("a.txt", "b.txt"); err == nil {
		t.Fatal("copy overwrote destination")
	}
	for _, p := range []string{"a.txt", "b.txt"} {
		v, _, er := e.Text(p)
		if er != nil || v != "copy me" {
			t.Fatal(p, v, er)
		}
	}
	if _, err = e.Copy("a.txt", "../escape"); err == nil {
		t.Fatal("copy traversal accepted")
	}
}

func (f fakeIdentity) PlatformSession(ctx context.Context, token string) (Session, error) {
	v, e := f.Session(ctx, token)
	v.User.Role = "user"
	return v, e
}
func TestOptionalPlatformSessionUsesPlatformRole(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := &Server{Identity: fakeIdentity{role: "admin"}, AllowPlatformSessions: enabled, DataDir: t.TempDir(), StaticDir: t.TempDir()}
		r := httptest.NewRequest("GET", "http://drive.test/api/files/list?root=system", nil)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
		r.AddCookie(&http.Cookie{Name: "minhanos_session", Value: "valid"})
		r.Header.Set("X-Drive-Auth-Mode", "platform")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("platform mode bypassed disabled adapter or standard-user role", enabled, w.Code)
		}
	}
}
func TestTraversalSymlinksAndSpecialFiles(t *testing.T) {
	e := testEngine(t)
	for _, p := range []string{"../outside", "a/../../b", "a\\b"} {
		if _, err := e.List(p); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	if err := e.Root.Symlink("/etc", "files/link"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.List("link"); err == nil {
		t.Fatal("followed symlink")
	}
	if _, _, err := e.Read("link/passwd"); err == nil {
		t.Fatal("followed parent symlink")
	}
	if _, err := e.Upload("", "../bad", bytes.NewReader(nil)); err == nil {
		t.Fatal("invalid filename accepted")
	}
	host := &Engine{Root: e.Root, Host: true}
	for _, p := range []string{"proc/self/environ", "sys/kernel", "dev/zero", "run/secrets"} {
		if _, err := host.List(p); err == nil {
			t.Fatal("virtual filesystem accepted", p)
		}
	}
}
func TestTrashConflictDoesNotOverwrite(t *testing.T) {
	e := testEngine(t)
	_, _ = e.Upload("", "a.txt", strings.NewReader("original"))
	item, err := e.Trash("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = e.Upload("", "a.txt", strings.NewReader("new"))
	if err = e.Restore(item.ID); err == nil {
		t.Fatal("restored over existing target")
	}
	text, _, _ := e.Text("a.txt")
	if text != "new" {
		t.Fatal("target overwritten")
	}
	items, _ := e.TrashList()
	if len(items) != 1 {
		t.Fatal("lost trash item")
	}
}

type fakeIdentity struct {
	role       string
	mustChange bool
	denied     string // permission refused by Authorize
}

func (f fakeIdentity) Session(_ context.Context, t string) (Session, error) {
	if t != "valid" {
		return Session{}, ErrUnauthorized
	}
	return Session{User: User{ID: strings.Repeat("a", 64), Username: "test", Role: f.role}, CSRF: "csrf", PasswordChangeRequired: f.mustChange}, nil
}
func (f fakeIdentity) Login(context.Context, string, string) (Session, string, error) {
	return Session{}, "", ErrUnauthorized
}
func (f fakeIdentity) Logout(context.Context, string) error { return nil }
func (f fakeIdentity) Authorize(_ context.Context, _ User, permission string) error {
	if permission == f.denied {
		return &identity.Fault{Status: 403, Code: "APP_ACCESS_DENIED"}
	}
	return nil
}
func (f fakeIdentity) Audit(context.Context, User, string, string) error { return nil }
func TestAuthorizationBoundary(t *testing.T) {
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
	handler := s.Handler()
	for _, test := range []struct {
		path, method, token, csrf, origin string
		want                              int
	}{{"/api/files/list", "GET", "", "", "", 401}, {"/api/files/list?root=system", "GET", "valid", "", "", 403}, {"/api/files/mkdir", "POST", "valid", "", "", 403}, {"/api/files/mkdir", "POST", "valid", "csrf", "https://evil.test", 403}, {"/api/files/list?root=personal", "GET", "valid", "", "", 200}} {
		r := httptest.NewRequest(test.method, "http://drive.test"+test.path, strings.NewReader(`{"name":"test"}`))
		r.AddCookie(&http.Cookie{Name: CookieName, Value: test.token})
		r.Header.Set("X-CSRF-Token", test.csrf)
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("%s %s: want %d got %d: %s", test.method, test.path, test.want, w.Code, w.Body.String())
		}
	}
}
func TestHostDeletionRequiresExplicitConfirmation(t *testing.T) {
	e := testEngine(t)
	e.Host = true
	r := httptest.NewRequest("POST", "http://agent/api/files/remove?path=files", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	err := ServeEngine(w, r, e)
	var result *FileError
	if !strings.Contains(err.Error(), "xác nhận") {
		t.Fatal(err)
	}
	_ = result
}
func TestReadJSONContract(t *testing.T) {
	e := testEngine(t)
	_, _ = e.Upload("", "test.txt", strings.NewReader("text"))
	r := httptest.NewRequest("GET", "http://drive/api/files/list", nil)
	w := httptest.NewRecorder()
	if err := ServeEngine(w, r, e); err != nil {
		t.Fatal(err)
	}
	var b struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || len(b.Entries) != 1 || b.Entries[0].Name != "test.txt" {
		t.Fatal(w.Body.String(), err)
	}
}

func (f fakeIdentity) CanAccessHost(_ context.Context, u User) (bool, error) {
	return f.role == "admin" && u.Role == "admin", nil
}
