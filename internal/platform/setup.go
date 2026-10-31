package platform

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"minhanos.local/platform/internal/requestid"
)

// First-run setup (ADR 0008 addendum): a fresh install has no account. The first visitor creates the System Admin
// and names the first (default) organization in one transaction. The setup code is the operator-held bootstrap
// secret (secrets/bootstrap_password), so nobody else on the network can claim the install. Once any account
// exists, setup is closed for good. MINHANOS_BOOTSTRAP_ADMIN=true keeps the old unattended bootstrap (automation).

var setupUsername = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

func (a *App) setupRequired(r *http.Request) (bool, error) {
	var count int
	err := a.db.QueryRow(r.Context(), "SELECT count(*) FROM users").Scan(&count)
	return count == 0, err
}

func (a *App) setupStatus(w http.ResponseWriter, r *http.Request) {
	required, err := a.setupRequired(r)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"required": required})
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.allowLogin(ip) {
		writeError(w, 429, "Đã thử quá nhiều lần. Vui lòng thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
		return
	}
	var body struct {
		SetupCode        string `json:"setupCode"`
		Username         string `json:"username"`
		DisplayName      string `json:"displayName"`
		Password         string `json:"password"`
		OrganizationName string `json:"organizationName"`
	}
	if !decode(w, r, &body) {
		return
	}
	if required, err := a.setupRequired(r); err != nil {
		internalError(w, r, err)
		return
	} else if !required {
		writeError(w, 409, "MinhAnOS đã được khởi tạo.", "SETUP_DONE")
		return
	}
	secret, err := os.ReadFile(os.Getenv("BOOTSTRAP_PASSWORD_FILE"))
	code := strings.TrimSpace(string(secret))
	if err != nil || len(code) < 12 {
		writeError(w, 503, "Chưa cấu hình mã cài đặt trên máy chủ.", "SETUP_UNAVAILABLE")
		return
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.SetupCode)), []byte(code)) != 1 {
		writeError(w, 403, "Mã cài đặt chưa đúng.", "SETUP_CODE_INVALID")
		return
	}
	name, org := strings.TrimSpace(body.DisplayName), strings.TrimSpace(body.OrganizationName)
	switch {
	case !setupUsername.MatchString(body.Username):
		writeError(w, 400, "Tên đăng nhập không hợp lệ.", "SETUP_INVALID_USERNAME")
		return
	case name == "" || utf8.RuneCountInString(name) > 100:
		writeError(w, 400, "Tên hiển thị không hợp lệ.", "SETUP_INVALID_NAME")
		return
	case utf8.RuneCountInString(body.Password) < 12 || len(body.Password) > 256:
		writeError(w, 400, "Mật khẩu cần 12–256 ký tự.", "SETUP_WEAK_PASSWORD")
		return
	case body.Password == code:
		writeError(w, 400, "Mật khẩu không được trùng mã cài đặt.", "SETUP_WEAK_PASSWORD")
		return
	case org == "" || utf8.RuneCountInString(org) > 100:
		writeError(w, 400, "Tên tổ chức không hợp lệ.", "SETUP_INVALID_ORGANIZATION")
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err = a.completeSetup(r, body.Username, name, hash, org); errors.Is(err, errSetupDone) {
		writeError(w, 409, "MinhAnOS đã được khởi tạo.", "SETUP_DONE")
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	slog.Info("platform.setup_completed")
	writeJSON(w, 201, map[string]any{"user": map[string]string{"username": body.Username, "displayName": name}, "organization": org})
}

var errSetupDone = errors.New("setup already completed")

// completeSetup creates the System Admin, names the default organization and makes the admin its admin, under the
// same advisory lock as startup so two setups (or a setup and a bootstrap) cannot both win.
func (a *App) completeSetup(r *http.Request, username, displayName, hash, organization string) error {
	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(681021)"); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errSetupDone
	}
	var user, org string
	id := token()
	if err = tx.QueryRow(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,$2,$3,'admin',$4) RETURNING public_id::text", id, username, displayName, hash).Scan(&user); err != nil {
		return err
	}
	// Whoever holds the setup code owns the server: like the former bootstrap admin, the first System Admin is a host
	// operator (Files "System" root); it can be revoked like any other.
	if _, err = tx.Exec(ctx, "INSERT INTO identity_host_operators(user_id,state,origin) VALUES($1,'enabled','first-run-setup')", id); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "UPDATE identity_organizations SET name=$1,version=version+1 WHERE is_default RETURNING id::text", organization).Scan(&org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO identity_memberships(organization_id,user_id,role) VALUES($1::uuid,$2::uuid,'admin') ON CONFLICT(organization_id,user_id) DO UPDATE SET role='admin'", org, user); err != nil {
		return err
	}
	ids := requestid.From(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,organization_id) VALUES($1::uuid,'setup.completed',$1::text,'success',$2,$3::uuid)", user, ids.Request, org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
