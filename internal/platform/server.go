package platform

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
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
	db           *pgxpool.Pool
	started      time.Time
	secureCookie bool
	limiterMu    sync.Mutex
	attempts     map[string]attempt
	dummyHash    string
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
type AppDefinition struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
	Category string `json:"category"`
	Status   string `json:"status"`
}

var registry = []AppDefinition{
	{"overview", "Tổng quan", "Không gian làm việc của bạn", "overview", "mint", "Hệ thống", "ready"},
	{"files", "File Station", "Drive độc lập · mở trong tab riêng", "files", "amber", "Dữ liệu", "ready"},
	{"apps", "Trung tâm ứng dụng", "Khám phá MinhAnOS", "apps", "blue", "Hệ thống", "ready"},
	{"connections", "Kết nối", "NetBird · Authentik · 1Panel", "connections", "violet", "Quản trị", "planned"},
	{"settings", "Cài đặt", "Cá nhân hóa desktop", "settings", "slate", "Hệ thống", "ready"},
	{"activity", "Nhật ký", "Hoạt động tài khoản", "activity", "coral", "Hệ thống", "ready"},
	{"notes", "Ghi chú", "Một nơi cho ý tưởng mới", "notes", "lime", "Tiện ích", "ready"},
	{"about", "Về MinhAnOS", "Hành trình đang bắt đầu", "about", "teal", "Hệ thống", "ready"},
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
	a.dummyHash, err = hashPassword(token())
	if err != nil {
		return nil, err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
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
		log.Print("bootstrap administrator created; password is not logged")
	}
	if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE expires_at < now()"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if a.db.Ping(ctx) != nil {
			writeError(w, 503, "Database chưa sẵn sàng.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok", "version": Version})
	})
	mux.HandleFunc("POST /api/login", a.login)
	mux.Handle("/api/drive/", a.driveProxy())
	mux.Handle("GET /api/session", a.auth(http.HandlerFunc(a.session)))
	mux.Handle("POST /api/logout", a.auth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/apps", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, registry) })))
	mux.Handle("GET /api/preferences", a.auth(http.HandlerFunc(a.getPreferences)))
	mux.Handle("PUT /api/preferences", a.auth(http.HandlerFunc(a.putPreferences)))
	mux.Handle("GET /api/system", a.auth(http.HandlerFunc(a.system)))
	mux.Handle("GET /api/audit", a.auth(http.HandlerFunc(a.auditList)))
	mux.Handle("GET /api/admin/users", a.auth(a.admin(http.HandlerFunc(a.users))))
	mux.Handle("POST /api/admin/users", a.auth(a.admin(http.HandlerFunc(a.createUser))))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "API không tồn tại.") })
	root := os.Getenv("STATIC_DIR")
	if root == "" {
		root = "/app/web"
	}
	fs := http.FileServer(http.Dir(root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			writeError(w, http.StatusMethodNotAllowed, "Phương thức không được hỗ trợ.")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api") {
			writeError(w, 404, "API không tồn tại.")
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !sameOrigin(r) {
				writeError(w, 403, "Nguồn yêu cầu không hợp lệ.")
				return
			}
			if !strings.HasPrefix(r.URL.Path, "/api/drive/") {
				r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
			}
		}
		mux.ServeHTTP(w, r)
	})
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
			writeError(w, 401, "Phiên đăng nhập đã hết hạn.")
			return
		}
		var i Identity
		err = a.db.QueryRow(r.Context(), `SELECT u.id,u.username,u.display_name,u.role,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()`, digest(cookie.Value)).Scan(&i.User.ID, &i.User.Username, &i.User.DisplayName, &i.User.Role, &i.CSRF)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				log.Print("session lookup failed: ", err)
				writeError(w, 503, "Không kiểm tra được phiên đăng nhập.")
				return
			}
			writeError(w, 401, "Phiên đăng nhập đã hết hạn.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(i.CSRF)) != 1 {
			writeError(w, 403, "Mã xác thực yêu cầu không hợp lệ.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, i)))
	})
}
func (a *App) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if identity(r).User.Role != "admin" {
			writeError(w, 403, "Bạn không có quyền quản trị tài khoản.")
			return
		}
		next.ServeHTTP(w, r)
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
		writeError(w, 429, "Đã thử đăng nhập quá nhiều lần. Vui lòng thử lại sau 10 phút.")
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
		writeError(w, 400, "Thông tin đăng nhập không hợp lệ.")
		return
	}
	var user User
	var hash string
	err := a.db.QueryRow(r.Context(), "SELECT id,username,display_name,role,password_hash FROM users WHERE username=$1", strings.ToLower(strings.TrimSpace(body.Username))).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Print(err)
		writeError(w, 503, "Dịch vụ đăng nhập tạm thời chưa sẵn sàng.")
		return
	}
	if hash == "" {
		hash = a.dummyHash
	}
	valid := verifyPassword(body.Password, hash)
	if err != nil || !valid {
		writeError(w, 401, "Tên đăng nhập hoặc mật khẩu chưa đúng.")
		return
	}
	sessionToken, csrf := token(), token()
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if old, e := r.Cookie("minhanos_session"); e == nil {
		if _, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", digest(old.Value)); err != nil {
			internalError(w, err)
			return
		}
	}
	_, err = tx.Exec(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)", digest(sessionToken), user.ID, csrf, time.Now().Add(24*time.Hour))
	if err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO audit(user_id,actor,action,detail) VALUES($1,$2,'auth.login','Đăng nhập bằng tài khoản cục bộ')", user.ID, user.Username)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		internalError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "minhanos_session", Value: sessionToken, Path: "/", HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	writeJSON(w, 200, map[string]any{"user": user, "csrfToken": csrf})
}
func (a *App) session(w http.ResponseWriter, r *http.Request) {
	i := identity(r)
	writeJSON(w, 200, map[string]any{"user": i.User, "csrfToken": i.CSRF})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("minhanos_session")
	i := identity(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", digest(cookie.Value))
	if err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO audit(user_id,actor,action,detail) VALUES($1,$2,'auth.logout','Đăng xuất')", i.User.ID, i.User.Username)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		internalError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "minhanos_session", Value: "", Path: "/", HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func defaultPreferences() Preferences {
	return Preferences{Theme: "light", Wallpaper: "mountain", Mode: "focus", Windows: []Window{{ID: "overview", X: 250, Y: 95, Width: 1000, Height: 650}}, Pinned: []string{"overview", "files", "apps", "settings"}}
}
func (a *App) getPreferences(w http.ResponseWriter, r *http.Request) {
	var raw []byte
	err := a.db.QueryRow(r.Context(), "SELECT value FROM preferences WHERE user_id=$1", identity(r).User.ID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, defaultPreferences())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	var prefs Preferences
	if err = json.Unmarshal(raw, &prefs); err != nil {
		internalError(w, err)
		return
	}
	if prefs.Mode == "" {
		prefs.Mode = "focus"
	}
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
		writeError(w, 400, "Cài đặt desktop không hợp lệ.")
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		internalError(w, err)
		return
	}
	_, err = a.db.Exec(r.Context(), "INSERT INTO preferences(user_id,value) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET value=EXCLUDED.value,updated_at=now()", identity(r).User.ID, raw)
	if err != nil {
		internalError(w, err)
		return
	}
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
		"version": Version, "node": "MinhAnOS · VM phát triển", "hostname": hostname, "os": runtime.GOOS, "architecture": runtime.GOARCH, "goVersion": runtime.Version(),
		"cpus": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "heapBytes": mem.HeapAlloc, "uptimeSeconds": int(time.Since(a.started).Seconds()),
		"diskTotalBytes": disk.Blocks * uint64(disk.Bsize), "diskFreeBytes": disk.Bavail * uint64(disk.Bsize), "database": dbState,
		"scope": "Thông số tiến trình Go và filesystem /data của container; chưa có agent đo toàn bộ máy chủ.",
		"integrations": []map[string]string{
			{"id": "netbird", "name": "NetBird", "status": "unconfigured", "description": "Quản lý thiết bị và mạng riêng"},
			{"id": "authentik", "name": "Authentik", "status": "unconfigured", "description": "Đăng nhập tập trung và danh tính"},
			{"id": "onepanel", "name": "1Panel", "status": "unconfigured", "description": "Máy chủ, container và ứng dụng"},
		},
	})
}
func (a *App) auditList(w http.ResponseWriter, r *http.Request) {
	i := identity(r)
	rows, err := a.db.Query(r.Context(), "SELECT id,actor,action,detail,created_at FROM audit WHERE ($1='admin' OR user_id=$2) ORDER BY id DESC LIMIT 60", i.User.Role, i.User.ID)
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id int64
		var actor, action, detail string
		var created time.Time
		if err = rows.Scan(&id, &actor, &action, &detail, &created); err != nil {
			internalError(w, err)
			return
		}
		result = append(result, map[string]any{"id": id, "actor": actor, "action": action, "detail": detail, "createdAt": created.UTC().Format(time.RFC3339)})
	}
	if err = rows.Err(); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *App) users(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), "SELECT id,username,display_name,role FROM users ORDER BY created_at")
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role); err != nil {
			internalError(w, err)
			return
		}
		users = append(users, u)
	}
	if err = rows.Err(); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, 200, users)
}

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

func (a *App) createUser(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Username = strings.ToLower(strings.TrimSpace(b.Username))
	b.DisplayName = strings.TrimSpace(b.DisplayName)
	if !usernamePattern.MatchString(b.Username) || len(b.DisplayName) < 1 || len(b.DisplayName) > 100 || len(b.Password) < 12 || len(b.Password) > 256 || (b.Role != "admin" && b.Role != "user") {
		writeError(w, 400, "Tên đăng nhập gồm 3–32 ký tự a-z, số, _ hoặc -. Mật khẩu cần ít nhất 12 ký tự.")
		return
	}
	hash, err := hashPassword(b.Password)
	if err != nil {
		internalError(w, err)
		return
	}
	u := User{ID: token(), Username: b.Username, DisplayName: b.DisplayName, Role: b.Role}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,$2,$3,$4,$5) ON CONFLICT(username) DO NOTHING", u.ID, u.Username, u.DisplayName, u.Role, hash)
	if err != nil {
		internalError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 409, "Tên đăng nhập đã tồn tại.")
		return
	}
	i := identity(r)
	_, err = tx.Exec(r.Context(), "INSERT INTO audit(user_id,actor,action,detail) VALUES($1,$2,'user.create',$3)", i.User.ID, i.User.Username, "Tạo tài khoản "+u.Username+" ("+u.Role+")")
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, 201, u)
}
func hashPassword(password string) (string, error) { return authn.Hash(password) }
func verifyPassword(password, hash string) bool    { return authn.Verify(password, hash) }
func token() string                                { return authn.Token() }
func digest(value string) string                   { return authn.Digest(value) }
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		writeError(w, 400, "Dữ liệu yêu cầu không hợp lệ.")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "Yêu cầu cần đúng một đối tượng JSON.")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func internalError(w http.ResponseWriter, err error) {
	log.Print("request failed: ", err)
	writeError(w, 500, "Không thể hoàn thành yêu cầu. Vui lòng thử lại.")
}
