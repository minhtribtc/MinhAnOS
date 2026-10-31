package drive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"minhanos.local/platform/internal/apicontract"
	"minhanos.local/platform/internal/coreprofile"
	"minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/requestid"
	"minhanos.local/platform/internal/storagecore"
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
	// Quota is the soft quota of personal homes; nil disables it (tests, standalone without a ledger).
	Quota QuotaLedger
	// Shares and Units place homes and Shared Folders (storage Q2); nil keeps the legacy layout only.
	Shares ShareRegistry
	Units  UnitAgent
	units  sync.Map
	// measureKick wakes the background measurer (set by RunMeasurer).
	measureKick chan struct{}
	// jobKick wakes the job worker (set by StartJobs).
	jobKick chan struct{}
	locks   sync.Map
	limiter loginLimiter
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// authorized applies the identity policy (user entitlement AND Files app grant AND access rules)
// before the resource rules of each root (own namespace, host operator for the system root).
func (s *Server) authorized(w http.ResponseWriter, r *http.Request, u User, permission string) bool {
	e := s.Identity.Authorize(r.Context(), u, permission)
	if e == nil {
		return true
	}
	var f *identity.Fault
	switch {
	case errors.As(e, &f):
		fail(w, f.Status, "Bạn không có quyền dùng Files cho thao tác này.", f.Code)
	case errors.Is(e, identity.ErrUnauthorized):
		fail(w, 401, "Vui lòng đăng nhập Drive.", "AUTH_REQUIRED")
	case errors.Is(e, identity.ErrNotReady):
		fail(w, 503, "Dữ liệu định danh chưa sẵn sàng.", "IDENTITY_NOT_READY")
	default:
		logError(r.Context(), "files.authorization_failed", e)
		fail(w, 503, "Không thể kiểm tra quyền.", "SERVICE_UNAVAILABLE")
	}
	return false
}
func fail(w http.ResponseWriter, status int, message string, code ...string) {
	jsonResponse(w, status, apicontract.Error(status, message, code...))
}
func decodeBody(r *http.Request, out any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, MaxText*6+8192))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return problem(400, "INVALID_REQUEST", "Dữ liệu yêu cầu không hợp lệ.")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return problem(400, "INVALID_REQUEST", "Yêu cầu phải chứa một đối tượng JSON.")
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
	identity.MountAccounts(mux)
	if adapter, ok := s.Identity.(interface{ IdentityHandler() http.Handler }); ok {
		mux.Handle("/api/v1/identity/", adapter.IdentityHandler())
	}
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"app": "MinhAnOS Files", "version": "0.3.0", "independent": true})
	})
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.limiter.allow(ip) {
			fail(w, 429, "Thử đăng nhập quá nhiều lần; thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
			return
		}
		var b struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if decodeBody(r, &b) != nil || len(b.Username) > 64 || len(b.Password) > 256 {
			fail(w, 400, "Thông tin đăng nhập không hợp lệ.", "INVALID_REQUEST")
			return
		}
		var session Session
		var token string
		var err error
		if rotating, ok := s.Identity.(interface {
			LoginRotating(context.Context, string, string, string) (Session, string, error)
		}); ok {
			session, token, err = rotating.LoginRotating(r.Context(), b.Username, b.Password, cookieToken(r))
		} else {
			session, token, err = s.Identity.Login(r.Context(), b.Username, b.Password)
		}
		if err != nil {
			if errors.Is(err, identity.ErrThrottled) {
				fail(w, 429, "Thử đăng nhập quá nhiều lần; thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
			} else if errors.Is(err, identity.ErrBusy) {
				fail(w, 503, "Dịch vụ danh tính đang bận.", "AUTH_BUSY")
			} else if errors.Is(err, ErrUnauthorized) {
				fail(w, 401, "Tên đăng nhập hoặc mật khẩu chưa đúng.", "INVALID_CREDENTIALS")
			} else {
				fail(w, 503, "Dịch vụ danh tính chưa sẵn sàng.", "IDENTITY_NOT_READY")
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
				fail(w, 403, "Adapter phiên nền tảng không được bật.", "PLATFORM_SESSION_DISABLED")
				return
			}
			if r.URL.Path == "/api/logout" {
				fail(w, 403, "Đăng xuất tại desktop.", "LOGOUT_IN_DESKTOP")
				return
			}
			cookie, e := r.Cookie("minhanos_session")
			if e != nil {
				fail(w, 401, "Phiên desktop đã hết hạn.", "AUTH_REQUIRED")
				return
			}
			session, err = adapter.PlatformSession(r.Context(), cookie.Value)
		} else {
			session, err = s.Identity.Session(r.Context(), cookieToken(r))
		}
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				fail(w, 401, "Vui lòng đăng nhập Drive.", "AUTH_REQUIRED")
			} else {
				fail(w, 503, "Không thể kiểm tra phiên đăng nhập.", "SERVICE_UNAVAILABLE")
			}
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !validCSRF(r, session) {
			fail(w, 403, "Mã CSRF không hợp lệ.", "CSRF_INVALID")
			return
		}
		if r.URL.Path == "/api/session" && r.Method == "GET" {
			jsonResponse(w, 200, session)
			return
		}
		if r.URL.Path == "/api/logout" && r.Method == "POST" {
			if err = s.Identity.Logout(r.Context(), cookieToken(r)); err != nil {
				fail(w, 503, "Chưa đăng xuất được.", "SERVICE_UNAVAILABLE")
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
		if session.PasswordChangeRequired {
			fail(w, 403, "Cần đổi mật khẩu trong Account Center trước khi tiếp tục.", "PASSWORD_CHANGE_REQUIRED")
			return
		}
		if r.URL.Path == "/api/export" && r.Method == "GET" {
			if s.authorized(w, r, session.User, "files.read") {
				s.exportHome(w, r, session.User)
			}
			return
		}
		if r.URL.Path == "/api/roots" && r.Method == "GET" {
			if !s.authorized(w, r, session.User, "files.read") {
				return
			}
			// Roots: the personal home once a share hosts homes, the Shared Folders the user may open (names
			// are the share names; access read or write), and the host system root for operators.
			roots := []map[string]string{}
			homes := true
			if _, e := s.homeDir(r.Context(), session.User.ID, false); errors.Is(e, storagecore.ErrNoHomes) {
				homes = false
			} else if e != nil && !errors.Is(e, fs.ErrNotExist) {
				logError(r.Context(), "files.homes.lookup_failed", e)
				fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
				return
			}
			if homes {
				roots = append(roots, map[string]string{"id": "personal", "kind": "home", "access": "write"})
			}
			if s.Shares != nil {
				shares, e := s.Shares.UserShares(r.Context(), session.User.ID)
				if e != nil {
					logError(r.Context(), "files.shares.lookup_failed", e)
					fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
					return
				}
				for _, share := range shares {
					roots = append(roots, map[string]string{"id": "share:" + share.Key, "kind": "share", "name": share.Name, "access": share.Access})
				}
			}
			if allowed, e := s.hostAllowed(r.Context(), session.User); e != nil {
				fail(w, 503, "Không thể kiểm tra quyền host.", "SERVICE_UNAVAILABLE")
				return
			} else if allowed && s.AgentSocket != "" && s.Identity.Authorize(r.Context(), session.User, "files.system.read") == nil {
				roots = append(roots, map[string]string{"id": "system", "kind": "system", "access": "write"})
			}
			body := map[string]any{"roots": roots, "homes": homes, "maxUploadBytes": MaxUpload, "maxTextBytes": MaxText}
			if homes && s.Quota != nil && validID(session.User.ID) {
				// Never measure here: large homes are walked by the background measurer.
				usage, err := s.Quota.HomeUsage(r.Context(), session.User.ID)
				if err != nil {
					quotaFailure(w, err)
					return
				}
				if !usage.Measured && s.measureKick != nil {
					s.kickMeasurer()
				}
				body["quota"] = usage
			}
			jsonResponse(w, 200, body)
			return
		}
		if r.URL.Path == "/api/quota-folders" && r.Method == "GET" {
			// Quota folders of a Shared Folder the user may read (Files shows the one the current path is in).
			if !s.authorized(w, r, session.User, "files.read") {
				return
			}
			key, _ := strings.CutPrefix(r.URL.Query().Get("root"), "share:")
			found, ok, e := s.userShare(r.Context(), session.User.ID, key)
			if e == nil && ok {
				var items []storagecore.QuotaFolder
				if items, e = s.Shares.QuotaFolderUsage(r.Context(), found.ID); e == nil {
					jsonResponse(w, 200, map[string]any{"items": items})
					return
				}
			}
			if e != nil {
				logError(r.Context(), "files.quota_folders.lookup_failed", e)
				fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
				return
			}
			fail(w, 404, "Không tìm thấy thư mục chia sẻ.", "SHARE_NOT_FOUND")
			return
		}
		if r.URL.Path == "/api/jobs" || strings.HasPrefix(r.URL.Path, "/api/jobs/") {
			permission := "files.read"
			if r.Method == "POST" {
				permission = "files.write"
			}
			if s.authorized(w, r, session.User, permission) {
				s.serveJobs(w, r, session.User)
			}
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/files/") {
			fail(w, 404, "API không tồn tại.", "NOT_FOUND")
			return
		}
		root := r.URL.Query().Get("root")
		if root == "" {
			root = "personal"
		}
		mutation := r.Method != "GET" && r.Method != "HEAD"
		if root == "system" {
			allowed, e := s.hostAllowed(r.Context(), session.User)
			if e != nil {
				fail(w, 503, "Không thể kiểm tra quyền host.", "SERVICE_UNAVAILABLE")
				return
			}
			if !allowed {
				fail(w, 403, "Tài khoản chưa được cấp quyền operator host.", "HOST_OPERATOR_REQUIRED")
				return
			}
		}
		shareKey, isShare := strings.CutPrefix(root, "share:")
		if root != "personal" && root != "system" && !isShare {
			fail(w, 400, "Không gian lưu trữ không hợp lệ.", "FILES_ROOT_INVALID")
			return
		}
		permission := "files.read"
		if mutation {
			permission = "files.write"
		}
		if root == "system" {
			permission = "files.system.read"
			if mutation {
				permission = "files.system.write"
			}
		}
		if !s.authorized(w, r, session.User, permission) {
			return
		}
		// A Shared Folder needs a share permission on top of the Files grant: read for GET, write otherwise.
		var share storagecore.UserShare
		if isShare {
			found, ok, e := s.userShare(r.Context(), session.User.ID, shareKey)
			if e != nil {
				logError(r.Context(), "files.share.lookup_failed", e)
				fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
				return
			}
			if !ok {
				fail(w, 404, "Không tìm thấy thư mục chia sẻ.", "SHARE_NOT_FOUND")
				return
			}
			if mutation && found.Access != "write" {
				fail(w, 403, "Thư mục chia sẻ này chỉ cho phép xem.", "SHARE_READ_ONLY")
				return
			}
			share = found
		}
		if mutation {
			rec, finish, done := s.idempotent(w, r, session.User)
			if done {
				return
			}
			defer finish()
			w = rec
		}
		if mutation {
			ev := AuditEvent{Action: "files." + strings.ReplaceAll(path.Base(r.URL.Path), "-", "_"), Root: root}
			if isShare {
				ev.Root, ev.Share = "share", shareKey
			} else if root == "system" {
				ev.Path = r.URL.Query().Get("path")
			}
			if err = s.Identity.Audit(r.Context(), session.User, ev); err != nil {
				fail(w, 503, "Chưa ghi được nhật ký; thao tác chưa thực hiện.", "AUDIT_UNAVAILABLE")
				return
			}
		}
		if root == "system" {
			if s.AgentSocket == "" {
				fail(w, 503, "Agent Debian chưa cấu hình.", "AGENT_NOT_CONFIGURED")
				return
			}
			s.proxyAgent(w, r, session.User)
			return
		}
		if !validID(session.User.ID) {
			fail(w, 403, "Định danh người dùng không hợp lệ.", "ACCESS_DENIED")
			return
		}
		// One lock per home or share: writes are serialized with each other and with measurements.
		lockKey := session.User.ID
		if isShare {
			lockKey = "share:" + share.ID
		}
		v, _ := s.locks.LoadOrStore(lockKey, &sync.RWMutex{})
		lock := v.(*sync.RWMutex)
		if mutation {
			lock.Lock()
			defer lock.Unlock()
		} else {
			lock.RLock()
			defer lock.RUnlock()
		}
		var dir string
		if isShare {
			dir, err = s.unitPath(r.Context(), share.Volume, "shares", share.Key, false)
		} else {
			dir, err = s.homeDir(r.Context(), session.User.ID, true)
		}
		switch {
		case errors.Is(err, storagecore.ErrNoHomes):
			fail(w, 409, "Chưa có thư mục chứa dữ liệu người dùng; quản trị viên cần tạo trong Storage Manager.", "STORAGE_NOT_READY")
			return
		case errors.Is(err, fs.ErrNotExist):
			fail(w, 404, "Không tìm thấy thư mục chia sẻ.", "SHARE_NOT_FOUND")
			return
		case err != nil:
			logError(r.Context(), "files.storage_unit_failed", err, "root", root)
			fail(w, 503, "Không mở được vùng lưu trữ.", "SERVICE_UNAVAILABLE")
			return
		}
		if err = os.MkdirAll(filepath.Join(dir, "files"), 0750); err != nil {
			handleFileError(w, err)
			return
		}
		if err = os.MkdirAll(filepath.Join(dir, "trash"), 0700); err != nil {
			handleFileError(w, err)
			return
		}
		rootFS, err := os.OpenRoot(dir)
		if err != nil {
			handleFileError(w, err)
			return
		}
		defer rootFS.Close()
		engine := &Engine{Root: rootFS}
		// A folder, or a file too large for one request, is copied by a durable job (row 20): 202 + the job.
		if path.Base(r.URL.Path) == "copy" && r.Method == "POST" {
			job, queued, e := s.copyJob(r, engine, session.User, root)
			if e != nil {
				handleFileError(w, e)
				return
			}
			if queued {
				jsonResponse(w, 202, map[string]any{"ok": true, "job": job})
				return
			}
		}
		// Soft quota: reserve what the write may add on the home and the homes share (or on the Shared
		// Folder), then commit the actual bytes or release on failure. The write lock is held throughout.
		var reservation *storagecore.Reservation
		// Quota folders of a Shared Folder (Q3): new data must fit every enclosing quota folder too; moves,
		// trash and restore shift usage between folders; renames keep the registrations.
		var plan *folderPlan
		if mutation && isShare && s.Quota != nil && s.Shares != nil {
			folders, e := s.Shares.ShareFolders(r.Context(), share.ID)
			if e != nil {
				logError(r.Context(), "files.quota_folders.lookup_failed", e)
				fail(w, 503, "Không kiểm tra được dung lượng.", "SERVICE_UNAVAILABLE")
				return
			}
			if len(folders) > 0 {
				if plan, err = s.planFolders(r, engine, folders); errors.Is(err, errProtectedFolder) {
					fail(w, 409, "Thư mục quota không đưa vào thùng rác được; quản trị viên cần gỡ quota trong Storage Manager trước.", "QUOTA_FOLDER_PROTECTED")
					return
				} else if err != nil {
					handleFileError(w, err)
					return
				}
				if path.Base(r.URL.Path) == "move" && len(plan.to) > 0 && plan.size > 0 {
					hold, e := s.Shares.ReserveShare(r.Context(), "", plan.to, plan.size)
					if e != nil {
						quotaFailure(w, e)
						return
					}
					plan.moveHold = &hold
				}
			}
		}
		if mutation && s.Quota != nil && (!isShare || s.Shares != nil) {
			if size := quotaSize(r, engine); size > 0 {
				var res storagecore.Reservation
				var e error
				if isShare {
					var write []string
					if plan != nil {
						write = plan.write
					}
					res, e = s.Shares.ReserveShare(r.Context(), share.ID, write, size)
				} else {
					res, e = s.Quota.Reserve(r.Context(), session.User.ID, size)
				}
				if e != nil {
					quotaFailure(w, e)
					return
				}
				reservation = &res
			}
		}
		if err = ServeEngine(w, r, engine); err != nil {
			handleFileError(w, err)
			logError(r.Context(), "files.operation_failed", err, "actor", session.User.ID, "operation", path.Base(r.URL.Path), "root", root)
		}
		s.settleFolders(share.ID, plan, err == nil)
		if reservation != nil {
			// Not cancelled with the client (the ledger must still settle) but keeps the request IDs for quota notices.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
			defer cancel()
			if err == nil {
				err = s.Quota.Commit(ctx, *reservation, quotaActual(r, engine, reservation.Bytes))
			} else {
				err = s.Quota.Release(ctx, *reservation)
			}
			if err != nil {
				logError(r.Context(), "files.quota.settle_failed", err, "actor", session.User.ID)
			}
		}
	})
	mux.Handle("/", http.FileServer(http.Dir(s.StaticDir)))
	return requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/v1/identity/") {
			if !sameOrigin(r) {
				fail(w, 403, "Nguồn yêu cầu không hợp lệ.", "ORIGIN_INVALID")
				return
			}
			limit := int64(MaxText*6 + 8192)
			if r.URL.Path == "/api/files/upload" {
				limit = MaxUpload + 1
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		mux.ServeHTTP(w, r)
	}))
}
func (s *Server) proxyAgent(w http.ResponseWriter, r *http.Request, u User) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", s.AgentSocket)
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(r.Context(), r.Method, "http://agent"+r.URL.RequestURI(), r.Body)
	if err != nil {
		fail(w, 500, "Không tạo được yêu cầu agent.", "INTERNAL_ERROR")
		return
	}
	request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if match := r.Header.Get("If-Match"); match != "" {
		request.Header.Set("If-Match", match)
	}
	request.Header.Set("X-Drive-Actor", u.Username)
	response, err := (&http.Client{Transport: transport, Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		fail(w, 503, "Không kết nối được agent Debian.", "AGENT_UNAVAILABLE")
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
			page, next, err := pageEntries(v, r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
			if err != nil {
				return err
			}
			jsonResponse(w, 200, map[string]any{"path": p, "entries": page, "nextCursor": next})
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
		return problem(405, "METHOD_NOT_ALLOWED", "Phương thức không được hỗ trợ.")
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
			return problem(400, "FILES_NAME_INVALID", "Tên không hợp lệ.")
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
			return problem(400, "FILES_CONFIRM_REQUIRED", "Cần xác nhận xóa tệp hệ thống.")
		}
		err = e.Remove(p)
	case "save-text":
		// The version precondition comes in If-Match (contract); the body field stays for older clients.
		tag := strings.Trim(r.Header.Get("If-Match"), `"`)
		if tag == "" {
			tag = b.ETag
		}
		if tag == "" {
			return problem(428, "PRECONDITION_REQUIRED", "Cần If-Match.")
		}
		var backup string
		backup, err = e.SaveText(p, b.Content, tag)
		result["backup"] = backup
	default:
		return problem(404, "NOT_FOUND", "Thao tác chưa hỗ trợ.")
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
			fail(w, 404, "Không có thao tác này.", "NOT_FOUND")
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
		logInfo(r.Context(), "host.files.operation", "actor", r.Header.Get("X-Drive-Actor"), "method", r.Method, "operation", path.Base(r.URL.Path), "success", err == nil)
	})
}

// An IdentityStore without an explicit host-policy adapter must fail closed.
func (s *Server) hostAllowed(ctx context.Context, u User) (bool, error) {
	adapter, ok := s.Identity.(interface {
		CanAccessHost(context.Context, User) (bool, error)
	})
	if !ok {
		return false, nil
	}
	return adapter.CanAccessHost(ctx, u)
}
