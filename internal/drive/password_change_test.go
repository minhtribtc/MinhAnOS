package drive

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilesClosedWhilePasswordChangeRequired(t *testing.T) {
	s := &Server{Identity: fakeIdentity{role: "user", mustChange: true}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
	handler := s.Handler()
	for path, want := range map[string]int{"/api/session": 200, "/api/files/list?root=personal": 403, "/api/roots": 403} {
		r := httptest.NewRequest("GET", "http://drive.test"+path, nil)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: want %d got %d: %s", path, want, w.Code, w.Body.String())
		}
		if want == 403 && !strings.Contains(w.Body.String(), "PASSWORD_CHANGE_REQUIRED") {
			t.Fatal("stable error code missing", w.Body.String())
		}
		if path == "/api/session" && !strings.Contains(w.Body.String(), `"passwordChangeRequired":true`) {
			t.Fatal("session must tell the UI", w.Body.String())
		}
	}
}

func TestFilesRequestsUseIdentityPolicy(t *testing.T) {
	for _, test := range []struct {
		denied, method, path string
		want                 int
	}{
		{"files.read", "GET", "/api/roots", 403},
		{"files.read", "GET", "/api/files/list?root=personal", 403},
		{"files.write", "GET", "/api/files/list?root=personal", 200},
		{"files.write", "POST", "/api/files/mkdir?root=personal", 403},
	} {
		s := &Server{Identity: fakeIdentity{role: "user", denied: test.denied}, DataDir: t.TempDir(), StaticDir: t.TempDir()}
		r := httptest.NewRequest(test.method, "http://drive.test"+test.path, strings.NewReader(`{"name":"x"}`))
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
		r.Header.Set("X-CSRF-Token", "csrf")
		r.Header.Set("Origin", "http://drive.test")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("denied %s %s %s: want %d got %d %s", test.denied, test.method, test.path, test.want, w.Code, w.Body.String())
		}
		if test.want == 403 && !strings.Contains(w.Body.String(), "APP_ACCESS_DENIED") {
			t.Fatal("policy code missing", w.Body.String())
		}
	}
}
