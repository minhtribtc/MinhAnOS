package identity

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountsStandaloneAssets(t *testing.T) {
	root := t.TempDir()
	if e := os.Mkdir(filepath.Join(root, "assets"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "index.html"), []byte(`<script type="module" src="/accounts/assets/app.js"></script>`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("export const standalone=true"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ACCOUNTS_STATIC_DIR", root)
	mux := http.NewServeMux()
	MountAccounts(mux)
	for _, path := range []string{"/accounts/", "/accounts/assets/app.js"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/accounts", nil))
	if w.Code != 307 || w.Header().Get("Location") != "/accounts/" {
		t.Fatal("standalone base redirect")
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/assets/missing.js", nil))
	if w.Code != 404 || strings.Contains(w.Body.String(), "standalone=true") {
		t.Fatal("missing assets must not return a fake SPA bundle")
	}
}
