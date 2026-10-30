package drive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"minhanos.local/platform/internal/apicontract"
	"minhanos.local/platform/internal/coreprofile"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const CookieName = "minhanos_drive_session"

type Server struct {
	Profile               *coreprofile.Store
	AllowPlatformSessions bool
	Identity              IdentityStore
	DataDir               string
	StaticDir             string
	AgentSocket           string
	SecureCookie          bool
	locks                 sync.Map
	limiter               loginLimiter
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string, code ...string) {
	jsonResponse(w, status, apicontract.Error(status, message, code...))
}
func decodeBody(r *http.Request, out any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, MaxText*6+8192))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return problem(400, "Dữ liệu yêu cầu không hợp lệ.")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return problem(400, "Yêu cầu phải chứa một đối tượng JSON.")
	}
	return nil
}
func sameOrigin(r *http.Request) bool {
	v := r.Header.Get("Origin")
	if v == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	u, err := url.Parse(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
}
func cookieToken(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
func (s *Server) setCookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.SecureCookie, MaxAge: age})
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"app": "MinhAnOS Drive", "version": "0.1.0", "independent": true})
	})
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.limiter.allow(ip) {
			fail(w, 429, "Thử đăng nhập quá nhiều lần; thử lại sau 10 phút.")
			return
		}
		var b struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if decodeBody(r, &b) != nil || len(b.Username) > 64 || len(b.Password) > 256 {
			fail(w, 400, "Thông tin đăng nhập không hợp lệ.")
			return
		}
		session, token, err := s.Identity.Login(r.Context(), b.Username, b.Password)
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				fail(w, 401, "Tên đăng nhập hoặc mật khẩu chưa đúng.", "INVALID_CREDENTIALS")
			} else {
				fail(w, 503, "Dịch vụ danh tính chưa sẵn sàng.")
			}
			return
		}
		s.setCookie(w, token, 86400)
		jsonResponse(w, 200, session)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		var session Session
		var err error
		if r.Header.Get("X-Drive-Auth-Mode") == "platform" {
			adapter, ok := s.Identity.(interface {
				PlatformSession(context.Context, string) (Session, error)
			})
			if !s.AllowPlatformSessions || !ok {
				fail(w, 403, "Adapter phiên nền tảng không được bật.")
				return
			}
			if r.URL.Path == "/api/logout" {
				fail(w, 403, "Đăng xuất tại desktop.")
				return
			}
			cookie, e := r.Cookie("minhanos_session")
			if e != nil {
				fail(w, 401, "Phiên desktop đã hết hạn.")
				return
			}
			session, err = adapter.PlatformSession(r.Context(), cookie.Value)
		} else {
			session, err = s.Identity.Session(r.Context(), cookieToken(r))
		}
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				fail(w, 401, "Vui lòng đăng nhập Drive.")
			} else {
				fail(w, 503, "Không thể kiểm tra phiên đăng nhập.")
			}
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !validCSRF(r, session) {
			fail(w, 403, "Mã CSRF không hợp lệ.")
			return
		}
		if r.URL.Path == "/api/session" && r.Method == "GET" {
			jsonResponse(w, 200, session)
			return
		}
		if r.URL.Path == "/api/logout" && r.Method == "POST" {
			if err = s.Identity.Logout(r.Context(), cookieToken(r)); err != nil {
				fail(w, 503, "Chưa đăng xuất được.")
				return
			}
			s.setCookie(w, "", -1)
			jsonResponse(w, 200, map[string]bool{"ok": true})
			return
		}
		if r.URL.Path == "/api/profile" && s.Profile != nil {
			s.Profile.Handle(w, r, session.User.ID)
			return
		}
		if r.URL.Path == "/api/roots" && r.Method == "GET" {
			roots := []map[string]string{{"id": "personal", "name": "Tệp của tôi"}}
			if session.User.Role == "admin" && s.AgentSocket != "" {
				roots = append(roots, map[string]string{"id": "system", "name": "Hệ thống Debian"})
			}
			jsonResponse(w, 200, map[string]any{"roots": roots, "maxUploadBytes": MaxUpload, "maxTextBytes": MaxText})
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/files/") {
			fail(w, 404, "API không tồn tại.")
			return
		}
		root := r.URL.Query().Get("root")
		if root == "" {
			root = "personal"
		}
		mutation := r.Method != "GET" && r.Method != "HEAD"
		target := root + ":" + r.URL.Query().Get("path")
		if root == "system" && session.User.Role != "admin" {
			fail(w, 403, "Chỉ admin được quản trị tệp hệ thống.")
			return
		}
		if root != "personal" && root != "system" {
			fail(w, 400, "Không gian lưu trữ không hợp lệ.")
			return
		}
		if mutation {
			if err = s.Identity.Audit(r.Context(), session.User, "files.request."+path.Base(r.URL.Path), target); err != nil {
				fail(w, 503, "Chưa ghi được nhật ký; thao tác chưa thực hiện.")
				return
			}
		}
		if root == "system" {
			if s.AgentSocket == "" {
				fail(w, 503, "Agent Debian chưa cấu hình.")
				return
			}
			s.proxyAgent(w, r, session.User)
			return
		}
		if !validID(session.User.ID) {
			fail(w, 403, "Định danh người dùng không hợp lệ.")
			return
		}
		v, _ := s.locks.LoadOrStore(session.User.ID, &sync.RWMutex{})
		lock := v.(*sync.RWMutex)
		if mutation {
			lock.Lock()
			defer lock.Unlock()
		} else {
			lock.RLock()
			defer lock.RUnlock()
		}
		userDir := filepath.Join(s.DataDir, "users", session.User.ID)
		if err = os.MkdirAll(filepath.Join(userDir, "files"), 0750); err != nil {
			handleFileError(w, err)
			return
		}
		if err = os.MkdirAll(filepath.Join(userDir, "trash"), 0700); err != nil {
			handleFileError(w, err)
			return
		}
		fs, err := os.OpenRoot(userDir)
		if err != nil {
			handleFileError(w, err)
			return
		}
		defer fs.Close()
		engine := &Engine{Root: fs}
		if err = ServeEngine(w, r, engine); err != nil {
			handleFileError(w, err)
			log.Printf("drive actor=%s operation=%s target=%q error=%v", session.User.Username, r.URL.Path, target, err)
		}
	})
	mux.Handle("/", http.FileServer(http.Dir(s.StaticDir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !sameOrigin(r) {
				fail(w, 403, "Nguồn yêu cầu không hợp lệ.")
				return
			}
			limit := int64(MaxText*6 + 8192)
			if r.URL.Path == "/api/files/upload" {
				limit = MaxUpload + 1
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) proxyAgent(w http.ResponseWriter, r *http.Request, u User) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", s.AgentSocket)
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(r.Context(), r.Method, "http://agent"+r.URL.RequestURI(), r.Body)
	if err != nil {
		fail(w, 500, "Không tạo được yêu cầu agent.")
		return
	}
	request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	request.Header.Set("X-Drive-Actor", u.Username)
	response, err := (&http.Client{Transport: transport, Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		fail(w, 503, "Không kết nối được agent Debian.")
		return
	}
	defer response.Body.Close()
	for _, key := range []string{"Content-Type", "Content-Disposition"} {
		if v := response.Header.Get(key); v != "" {
			w.Header().Set(key, v)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

// ServeEngine contains storage operations only; auth is enforced by Server or the
// filesystem permissions on the host agent's Unix socket. It never executes shell commands.
func ServeEngine(w http.ResponseWriter, r *http.Request, e *Engine) error {
	p := r.URL.Query().Get("path")
	op := strings.TrimPrefix(r.URL.Path, "/api/files/")
	if r.Method == "GET" {
		switch op {
		case "list":
			v, err := e.List(p)
			if err != nil {
				return err
			}
			jsonResponse(w, 200, map[string]any{"path": p, "entries": v})
			return nil
		case "download":
			return sendFile(w, r, e, p)
		case "text":
			content, tag, err := e.Text(p)
			if err != nil {
				return err
			}
			jsonResponse(w, 200, map[string]string{"content": content, "etag": tag})
			return nil
		case "trash":
			v, err := e.TrashList()
			if err != nil {
				return err
			}
			jsonResponse(w, 200, v)
			return nil
		}
	}
	if r.Method != "POST" {
		return problem(405, "Phương thức không được hỗ trợ.")
	}
	if op == "upload" {
		v, err := e.Upload(p, r.URL.Query().Get("name"), r.Body)
		if err != nil {
			return err
		}
		jsonResponse(w, 201, v)
		return nil
	}
	var b struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		ID          string `json:"id"`
		Content     string `json:"content"`
		ETag        string `json:"etag"`
		Confirm     bool   `json:"confirm"`
	}
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	var err error
	result := map[string]any{"ok": true}
	switch op {
	case "mkdir":
		err = e.Mkdir(p, b.Name)
	case "rename":
		if !validName(b.Name) {
			return problem(400, "Tên không hợp lệ.")
		}
		if _, err = e.logical(p); err == nil {
			err = e.Rename(p, path.Join(path.Dir(p), b.Name))
		}
	case "move":
		err = e.Rename(p, b.Destination)
	case "copy":
		var item Entry
		item, err = e.Copy(p, b.Destination)
		result["item"] = item
	case "trash":
		var item TrashItem
		item, err = e.Trash(p)
		result["item"] = item
	case "restore":
		err = e.Restore(b.ID)
	case "remove":
		if !b.Confirm {
			return problem(400, "Cần xác nhận xóa tệp hệ thống.")
		}
		err = e.Remove(p)
	case "save-text":
		var backup string
		backup, err = e.SaveText(p, b.Content, b.ETag)
		result["backup"] = backup
	default:
		return problem(404, "Thao tác chưa hỗ trợ.")
	}
	if err != nil {
		return err
	}
	jsonResponse(w, 200, result)
	return nil
}
func AgentHandler(root *os.Root) http.Handler {
	var lock sync.RWMutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/files/") {
			fail(w, 404, "Không có thao tác này.")
			return
		}
		if r.Method == "GET" {
			lock.RLock()
			defer lock.RUnlock()
		} else {
			lock.Lock()
			defer lock.Unlock()
			r.Body = http.MaxBytesReader(w, r.Body, MaxUpload+1)
		}
		err := ServeEngine(w, r, &Engine{Root: root, Host: true})
		if err != nil {
			handleFileError(w, err)
		}
		log.Printf("host.files actor=%q method=%s operation=%q path=%q success=%t", r.Header.Get("X-Drive-Actor"), r.Method, r.URL.Path, r.URL.Query().Get("path"), err == nil)
	})
}
