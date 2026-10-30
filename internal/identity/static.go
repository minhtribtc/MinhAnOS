package identity

import (
	"net/http"
	"os"
)

// MountAccounts adds a standalone frontend to either existing host. The frontend
// authenticates against that host's real realm; it does not require Desktop JS.
func MountAccounts(mux *http.ServeMux) {
	root := os.Getenv("ACCOUNTS_STATIC_DIR")
	if root == "" {
		root = "/app/accounts"
	}
	mux.HandleFunc("GET /accounts", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/accounts/", http.StatusTemporaryRedirect)
	})
	mux.Handle("GET /accounts/", http.StripPrefix("/accounts/", http.FileServer(http.Dir(root))))
}
