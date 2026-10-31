package platform

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"minhanos.local/platform/internal/apicontract"
	"minhanos.local/platform/internal/coreprofile"
	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/logcore"
	"minhanos.local/platform/internal/notifycore"
	"minhanos.local/platform/internal/requestid"
	"minhanos.local/platform/internal/storagecore"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
	"minhanos.local/platform/internal/drive"
)

const Version = "0.2.0"
const passwordIterations = 600000
const schema = `
CREATE TABLE IF NOT EXISTS schema_version (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS users (
 id text PRIMARY KEY, username text UNIQUE NOT NULL, display_name text NOT NULL,
 role text NOT NULL CHECK(role IN ('admin','user')), password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 csrf_token text NOT NULL, expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS preferences (
 user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 value jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS audit (
 id bigserial PRIMARY KEY, user_id text REFERENCES users(id) ON DELETE SET NULL,
 actor text NOT NULL, action text NOT NULL, detail text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_version(version) VALUES(1) ON CONFLICT DO NOTHING;
`

type App struct {
	db              *pgxpool.Pool
	started         time.Time
	secureCookie    bool
	limiterMu       sync.Mutex
	attempts        map[string]attempt
	profile         *coreprofile.Store
	authCore        *identityschema.AuthCore
	identityService *identityschema.Service
	storage         storagecore.Manager
	notifications   notifycore.Store
	logs            logcore.Store
}
type attempt struct {
	Count int
	Start time.Time
}
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}
type Identity struct {
	User User
	CSRF string
	// PasswordChangeRequired: only session, logout and profile work until the password is changed.
	PasswordChangeRequired bool
}
type identityKey struct{}
type Window struct {
	ID        string  `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Minimized bool    `json:"minimized"`
	Maximized bool    `json:"maximized"`
}
type Preferences struct {
	Theme     string   `json:"theme"`
	Wallpaper string   `json:"wallpaper"`
	Mode      string   `json:"mode"`
	Windows   []Window `json:"windows"`
	Notes     string   `json:"notes"`
	Pinned    []string `json:"pinned"`
}

// AppDefinition is one launcher entry. Names and descriptions are not sent: the Shell reads them from the
// app's manifest catalog (nameKey, <namespace>.app.description) so they follow the user's locale.
type AppDefinition struct {
	ID    string `json:"id"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// registry lists the first-party apps compiled into this Shell by App ID. The legacy IDs "files" and
// "settings" stay so saved layouts survive (contracts/apps/catalog.v1.json maps them).
var registry = []AppDefinition{
	{"files", "files", "amber"},
	{"settings", "settings", "slate"},
	{"com.minhanos.notes", "notes", "lime"},
	{"com.minhanos.account-center", "shield", "blue"},
	{"com.minhanos.storage", "storage", "teal"},
	{"com.minhanos.notifications", "bell", "coral"},
	{"com.minhanos.logs", "logs", "slate"},
}

func New(ctx context.Context, db *pgxpool.Pool) (*App, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(681021)"); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return nil, err
	}

	a := &App{db: db, started: time.Now(), secureCookie: os.Getenv("COOKIE_SECURE") == "true", attempts: make(map[string]attempt)}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return nil, err
	}
	// A fresh install waits for the first-run setup (setup.go); unattended installs opt into the bootstrap admin.
	if count == 0 && os.Getenv("MINHANOS_BOOTSTRAP_ADMIN") == "true" {
		password, readErr := os.ReadFile(os.Getenv("BOOTSTRAP_PASSWORD_FILE"))
		if readErr != nil {
			return nil, fmt.Errorf("bootstrap password file: %w", readErr)
		}
		pass := strings.TrimSpace(string(password))
		if len(pass) < 12 {
			return nil, errors.New("bootstrap password must be at least 12 characters")
		}
		hash, err := hashPassword(pass)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,'admin','Minh An','admin',$2)", token(), hash)
		if err != nil {
			return nil, err
		}
		slog.Info("platform.bootstrap_admin_created")
	}
	if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE expires_at < now()"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err = identityschema.ApplySchema(ctx, db); err != nil {
		return nil, err
	}
	if err = storagecore.ApplySchema(ctx, db); err != nil {
		return nil, err
	}
	// Log Center reads the Files audit, so the Files schema exists before the first query.
	if err = drive.ApplySchema(ctx, db); err != nil {
		return nil, err
	}
	a.storage = storagecore.Manager{Store: storagecore.Store{DB: db}, Agent: storagecore.AgentClient{Socket: os.Getenv("STORAGE_AGENT_SOCKET")}}
	if err = notifycore.ApplySchema(ctx, db); err != nil {
		return nil, err
	}
	a.notifications, a.logs = notifycore.Store{DB: db}, logcore.Store{DB: db}
	// Security notices reach the inbox within seconds; the bell polls the unread count.
	go a.notifications.Run(ctx, 10*time.Second)
	a.authCore, err = identityschema.NewAuthCore(db)
	if err != nil {
		return nil, err
	}
	a.identityService, err = identityschema.NewService(ctx, db, a.authCore)
	if err != nil {
		return nil, err
	}
	a.profile, err = coreprofile.New(ctx, db)
	if err != nil {
		return nil, err
	}
	go a.runRemovals(ctx)
	return a, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	identityschema.MountAccounts(mux)
	if a.identityService != nil {
		mux.Handle("/api/v1/identity/", a.identityService.HTTPHandler(identityschema.DesktopRealm))
	}
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if a.db.Ping(ctx) != nil {
			writeError(w, 503, "Database chưa sẵn sàng.", "SERVICE_UNAVAILABLE")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok", "version": Version})
	})
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("GET /api/setup", a.setupStatus)
	mux.HandleFunc("POST /api/setup", a.setup)
	mux.Handle("/api/drive/", a.driveProxy())
	mux.Handle("GET /api/session", a.auth(http.HandlerFunc(a.session)))
	mux.Handle("POST /api/logout", a.auth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/apps", a.auth(http.HandlerFunc(a.apps)))
	mux.Handle("/api/v1/storage/", a.auth(a.storageHandler()))
	mux.Handle("/api/v1/notifications/", a.auth(a.notificationsHandler()))
	mux.Handle("/api/v1/logs/", a.auth(a.logsHandler()))
	mux.Handle("/api/profile", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.profile.Handle(w, r, identity(r).User.ID) })))
	mux.Handle("GET /api/preferences", a.auth(http.HandlerFunc(a.getPreferences)))
	mux.Handle("PUT /api/preferences", a.auth(http.HandlerFunc(a.putPreferences)))
	mux.Handle("GET /api/export", a.auth(http.HandlerFunc(a.exportData)))
	mux.Handle("GET /api/system", a.auth(http.HandlerFunc(a.system)))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, 404, "API không tồn tại.", "NOT_FOUND")
	})
	root := os.Getenv("STATIC_DIR")
	if root == "" {
		root = "/app/web"
	}
	fs := http.FileServer(http.Dir(root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			writeError(w, http.StatusMethodNotAllowed, "Phương thức không được hỗ trợ.", "METHOD_NOT_ALLOWED")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api") {
			writeError(w, 404, "API không tồn tại.", "NOT_FOUND")
			return
		}
		if r.URL.Path != "/" && filepath.Ext(r.URL.Path) == "" {
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})
	return requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/v1/identity/") {
			if !sameOrigin(r) {
				writeError(w, 403, "Nguồn yêu cầu không hợp lệ.", "ACCESS_DENIED")
				return
			}
			if !strings.HasPrefix(r.URL.Path, "/api/drive/") {
				r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
			}
		}
		mux.ServeHTTP(w, r)
	}))
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("minhanos_session")
		if err != nil || len(cookie.Value) != 64 {
			writeError(w, 401, "Phiên đăng nhập đã hết hạn.", "AUTH_REQUIRED")
			return
		}
		v, err := a.authCore.Authenticate(r.Context(), identityschema.DesktopRealm, cookie.Value)
		i := Identity{User: User{ID: v.Principal.StorageID, Username: v.Principal.Username, DisplayName: v.Principal.DisplayName, Role: v.Principal.Role}, CSRF: v.CSRF, PasswordChangeRequired: v.Principal.PasswordChangeRequired}
		if err != nil {
			if !errors.Is(err, identityschema.ErrUnauthorized) {
				logError(r.Context(), "platform.session_lookup_failed", err)
				writeError(w, 503, "Không kiểm tra được phiên đăng nhập.", "SERVICE_UNAVAILABLE")
				return
			}
			writeError(w, 401, "Phiên đăng nhập đã hết hạn.", "AUTH_REQUIRED")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(i.CSRF)) != 1 {
			writeError(w, 403, "Mã xác thực yêu cầu không hợp lệ.", "ACCESS_DENIED")
			return
		}
		if i.PasswordChangeRequired && !(r.URL.Path == "/api/session" || r.URL.Path == "/api/logout" || r.URL.Path == "/api/profile") {
			writeError(w, 403, "Cần đổi mật khẩu trước khi tiếp tục.", "PASSWORD_CHANGE_REQUIRED")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, i)))
	})
}
func identity(r *http.Request) Identity { return r.Context().Value(identityKey{}).(Identity) }

func (a *App) allowLogin(ip string) bool {
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()
	now := time.Now()
	for key, value := range a.attempts {
		if now.Sub(value.Start) > 10*time.Minute {
			delete(a.attempts, key)
		}
	}
	if _, ok := a.attempts[ip]; !ok && len(a.attempts) >= 4096 {
		return false
	}
	v := a.attempts[ip]
	if v.Start.IsZero() {
		v.Start = now
	}
	v.Count++
	a.attempts[ip] = v
	return v.Count <= 20
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.allowLogin(ip) {
		writeError(w, 429, "Đã thử đăng nhập quá nhiều lần. Vui lòng thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Username) > 64 || len(body.Password) > 256 {
		writeError(w, 400, "Thông tin đăng nhập không hợp lệ.", "INVALID_REQUEST")
		return
	}
	old := ""
	if c, e := r.Cookie("minhanos_session"); e == nil {
		old = c.Value
	}
	v, sessionToken, err := a.authCore.Login(r.Context(), identityschema.DesktopRealm, body.Username, body.Password, old)
	if err != nil {
		switch {
		case errors.Is(err, identityschema.ErrUnauthorized):
			writeError(w, 401, "Tên đăng nhập hoặc mật khẩu chưa đúng.", "INVALID_CREDENTIALS")
		case errors.Is(err, identityschema.ErrThrottled):
			writeError(w, 429, "Đã thử đăng nhập quá nhiều lần. Vui lòng thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
		case errors.Is(err, identityschema.ErrBusy):
			writeError(w, 503, "Dịch vụ đăng nhập đang bận.", "AUTH_BUSY")
		default:
			internalError(w, r, err)
		}
		return
	}
	user := User{ID: v.Principal.StorageID, Username: v.Principal.Username, DisplayName: v.Principal.DisplayName, Role: v.Principal.Role}
	csrf := v.CSRF
	http.SetCookie(w, &http.Cookie{Name: "minhanos_session", Value: sessionToken, Path: "/", HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	writeJSON(w, 200, map[string]any{"user": user, "csrfToken": csrf, "passwordChangeRequired": v.Principal.PasswordChangeRequired})
}

// launcherPolicy is the app grant and permission that decide whether a registry entry is listed.
var launcherPolicy = map[string][2]string{
	"files":                         {identityschema.FilesApp, "files.read"},
	identityschema.StorageApp:       {identityschema.StorageApp, "storage.manage"},
	identityschema.NotificationsApp: {identityschema.NotificationsApp, "notifications.own.read"},
	identityschema.LogsApp:          {identityschema.LogsApp, "logs.audit.read"},
}

// apps lists the launcher; apps under the identity policy are hidden when it denies them to this user.
func (a *App) apps(w http.ResponseWriter, r *http.Request) {
	if a.identityService == nil {
		visible := []AppDefinition{}
		for _, app := range registry {
			if _, governed := launcherPolicy[app.ID]; !governed || app.ID == "files" {
				visible = append(visible, app)
			}
		}
		writeJSON(w, 200, visible)
		return
	}
	visible := make([]AppDefinition, 0, len(registry))
	for _, app := range registry {
		if p, governed := launcherPolicy[app.ID]; governed {
			allowed := a.identityService.CheckApp(r.Context(), identity(r).User.ID, p[0], p[1]) == nil
			// Storage Manager and Log Center also open for organization admins (scoped; ADR 0008).
			if !allowed && (app.ID == identityschema.StorageApp || app.ID == identityschema.LogsApp) {
				scope, err := a.orgAdminScope(r, p[0], p[1])
				allowed = err == nil && scope != nil
			}
			if !allowed {
				continue
			}
		}
		visible = append(visible, app)
	}
	writeJSON(w, 200, visible)
}
func (a *App) session(w http.ResponseWriter, r *http.Request) {
	i := identity(r)
	writeJSON(w, 200, map[string]any{"user": i.User, "csrfToken": i.CSRF, "passwordChangeRequired": i.PasswordChangeRequired})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("minhanos_session")
	if err := a.authCore.Logout(r.Context(), identityschema.DesktopRealm, cookie.Value); err != nil {
		internalError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "minhanos_session", Value: "", Path: "/", HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func defaultPreferences() Preferences {
	return Preferences{Theme: "light", Wallpaper: "mountain", Mode: "focus", Windows: []Window{}, Pinned: []string{"files", "settings", "com.minhanos.notes"}}
}

// knownApps drops windows and pins of apps no longer in the registry (Overview, App Center, Connections,
// Activity and About were removed on 2026-10-31) and maps the legacy "notes" ID to com.minhanos.notes.
func knownApps(p Preferences) Preferences {
	known := map[string]bool{}
	for _, app := range registry {
		known[app.ID] = true
	}
	rename := func(id string) string {
		if id == "notes" {
			return "com.minhanos.notes"
		}
		return id
	}
	windows, pinned, seen := []Window{}, []string{}, map[string]bool{}
	for _, w := range p.Windows {
		if w.ID = rename(w.ID); known[w.ID] && !seen[w.ID] {
			seen[w.ID] = true
			windows = append(windows, w)
		}
	}
	seen = map[string]bool{}
	for _, id := range p.Pinned {
		if id = rename(id); known[id] && !seen[id] {
			seen[id] = true
			pinned = append(pinned, id)
		}
	}
	p.Windows, p.Pinned = windows, pinned
	return p
}

// preferencesTag is the version of a user's preferences as GET returns them (strong ETag over the JSON).
func preferencesTag(p Preferences) string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// storedPreferences reads the user's preferences as GET returns them (defaults when none are saved).
func storedPreferences(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, user string, lock bool) (Preferences, error) {
	var raw []byte
	query := "SELECT value FROM preferences WHERE user_id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRow(ctx, query, user).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultPreferences(), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	var prefs Preferences
	if err = json.Unmarshal(raw, &prefs); err != nil {
		return Preferences{}, err
	}
	if prefs.Mode == "" {
		prefs.Mode = "focus"
	}
	return knownApps(prefs), nil
}
func (a *App) getPreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := storedPreferences(r.Context(), a.db, identity(r).User.ID, false)
	if err != nil {
		internalError(w, r, err)
		return
	}
	w.Header().Set("ETag", preferencesTag(prefs))
	writeJSON(w, 200, prefs)
}
func validPreferences(p Preferences) bool {
	if p.Mode != "focus" && p.Mode != "windows" {
		return false
	}
	if (p.Theme != "light" && p.Theme != "dark") || (p.Wallpaper != "mountain" && p.Wallpaper != "midnight" && p.Wallpaper != "sage") || len(p.Notes) > 32000 || len(p.Windows) > len(registry) || len(p.Pinned) > len(registry) {
		return false
	}
	known := map[string]bool{}
	for _, app := range registry {
		known[app.ID] = true
	}
	seen := map[string]bool{}
	for _, w := range p.Windows {
		if !known[w.ID] || seen[w.ID] || w.X < 0 || w.X > 20000 || w.Y < 0 || w.Y > 20000 || w.Width < 260 || w.Width > 10000 || w.Height < 200 || w.Height > 10000 {
			return false
		}
		seen[w.ID] = true
	}
	seen = map[string]bool{}
	for _, id := range p.Pinned {
		if !known[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func (a *App) putPreferences(w http.ResponseWriter, r *http.Request) {
	var p Preferences
	if !decode(w, r, &p) {
		return
	}
	if p.Mode == "" {
		p.Mode = "focus"
	}
	if !validPreferences(p) {
		writeError(w, 400, "Cài đặt desktop không hợp lệ.", "INVALID_REQUEST")
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		internalError(w, r, err)
		return
	}
	// Writes are versioned: If-Match must carry the ETag of the preferences this client last read.
	match := r.Header.Get("If-Match")
	if match == "" {
		writeError(w, 428, "Cần If-Match.", "PRECONDITION_REQUIRED")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	current, err := storedPreferences(r.Context(), tx, identity(r).User.ID, true)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if match != preferencesTag(current) {
		writeError(w, 412, "Cài đặt desktop đã thay đổi ở nơi khác.", "VERSION_CONFLICT")
		return
	}
	_, err = tx.Exec(r.Context(), "INSERT INTO preferences(user_id,value) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET value=EXCLUDED.value,updated_at=now()", identity(r).User.ID, raw)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	w.Header().Set("ETag", preferencesTag(knownApps(p)))
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) system(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	var disk syscall.Statfs_t
	_ = syscall.Statfs("/data", &disk)
	hostname, _ := os.Hostname()
	dbState := "connected"
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if a.db.Ping(ctx) != nil {
		dbState = "disconnected"
	}
	writeJSON(w, 200, map[string]any{
		"version": Version, "hostname": hostname, "os": runtime.GOOS, "architecture": runtime.GOARCH, "goVersion": runtime.Version(),
		"cpus": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "heapBytes": mem.HeapAlloc, "uptimeSeconds": int(time.Since(a.started).Seconds()),
		"diskTotalBytes": disk.Blocks * uint64(disk.Bsize), "diskFreeBytes": disk.Bavail * uint64(disk.Bsize), "database": dbState,
	})
}
func hashPassword(password string) (string, error) { return authn.Hash(password) }
func verifyPassword(password, hash string) bool    { return authn.Verify(password, hash) }
func token() string                                { return authn.Token() }
func digest(value string) string                   { return authn.Digest(value) }
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		writeError(w, 400, "Dữ liệu yêu cầu không hợp lệ.", "INVALID_REQUEST")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "Yêu cầu cần đúng một đối tượng JSON.", "INVALID_REQUEST")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string, code ...string) {
	writeJSON(w, status, apicontract.Error(status, message, code...))
}
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	logError(r.Context(), "platform.request_failed", err, "method", r.Method, "route", r.Pattern)
	writeError(w, 500, "Không thể hoàn thành yêu cầu. Vui lòng thử lại.", "INTERNAL_ERROR")
}
