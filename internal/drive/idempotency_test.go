package drive

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilesWritesNeedIdempotencyKey(t *testing.T) {
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
	r := httptest.NewRequest("POST", "http://drive.test/api/files/mkdir?root=personal&path=", strings.NewReader(`{"name":"docs"}`))
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r.Header.Set("Origin", "http://drive.test")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "IDEMPOTENCY_KEY_REQUIRED") {
		t.Fatal("write without key", w.Code, w.Body.String())
	}
}

func TestListPages(t *testing.T) {
	all := make([]Entry, 25)
	for i := range all {
		all[i].Name = fmt.Sprint(i)
	}
	first, next, err := pageEntries(all, "", "10")
	if err != nil || len(first) != 10 || next == nil {
		t.Fatal("first page", len(first), next, err)
	}
	seen := len(first)
	for next != nil {
		var page []Entry
		if page, next, err = pageEntries(all, *next, "10"); err != nil {
			t.Fatal(err)
		}
		seen += len(page)
	}
	if seen != 25 {
		t.Fatal("pages lost entries", seen)
	}
	for _, bad := range [][2]string{{"", "0"}, {"", "1001"}, {"!!", ""}, {"OTk5", ""}} {
		if _, _, err := pageEntries(all, bad[0], bad[1]); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}
