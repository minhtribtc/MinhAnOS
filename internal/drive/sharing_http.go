package drive

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"minhanos.local/platform/internal/authn"
	"minhanos.local/platform/internal/storagecore"
)

// serveSharing answers the owner's sharing API (ADR 0011); every route needs a Files session.
//
//	GET  /api/sharing?path=p            grants and links of an item (all of them without path)
//	GET  /api/sharing/received          items shared with me
//	GET  /api/sharing/recipients?q=     users I may share with
//	POST /api/sharing/grants            {path, recipient, permission}   (adds or changes)
//	POST /api/sharing/grants/remove     {id}
//	POST /api/sharing/links             {path, permission, password, expires}
//	POST /api/sharing/links/update      {id, permission, password, clearPassword, expires}
//	POST /api/sharing/links/remove      {id}
func (s *Server) serveSharing(w http.ResponseWriter, r *http.Request, u User) {
	if s.Sharing == nil {
		fail(w, 503, "Chia sẻ chưa được bật.", "SERVICE_UNAVAILABLE")
		return
	}
	if !validID(u.ID) {
		fail(w, 403, "Định danh người dùng không hợp lệ.", "ACCESS_DENIED")
		return
	}
	ctx, op := r.Context(), strings.TrimPrefix(r.URL.Path, "/api/sharing")
	unavailable := func(e error) {
		logError(ctx, "files.sharing.failed", e, "operation", op)
		fail(w, 503, "Không đọc hoặc lưu được chia sẻ.", "SERVICE_UNAVAILABLE")
	}
	if r.Method == "GET" {
		var body any
		var err error
		switch op {
		case "":
			var grants []Grant
			var links []Link
			p := r.URL.Query().Get("path")
			if p != "" {
				if p, err = clean(p); err != nil {
					handleFileError(w, err)
					return
				}
			}
			if grants, err = s.Sharing.Grants(ctx, u.ID, p); err == nil {
				links, err = s.Sharing.Links(ctx, u.ID, p)
			}
			body = map[string]any{"grants": grants, "links": links, "limits": map[string]int{"grants": MaxGrants, "links": MaxLinks}}
		case "/received":
			var list []Grant
			list, err = s.Sharing.Received(ctx, u.ID)
			body = map[string]any{"items": list}
		case "/recipients":
			var list []Recipient
			q := strings.TrimSpace(r.URL.Query().Get("q"))
			if len(q) > 100 {
				fail(w, 400, "Từ khóa không hợp lệ.", "FILES_SEARCH_INVALID")
				return
			}
			list, err = s.Sharing.Recipients(ctx, u.ID, q)
			body = map[string]any{"items": list}
		default:
			fail(w, 404, "API không tồn tại.", "NOT_FOUND")
			return
		}
		if err != nil {
			unavailable(err)
			return
		}
		jsonResponse(w, 200, body)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Phương thức không được hỗ trợ.", "METHOD_NOT_ALLOWED")
		return
	}
	if !s.authorized(w, r, u, "files.write") {
		return
	}
	var b struct {
		ID            string `json:"id"`
		Path          string `json:"path"`
		Recipient     string `json:"recipient"`
		Permission    string `json:"permission"`
		Password      string `json:"password"`
		ClearPassword bool   `json:"clearPassword"`
		Expires       string `json:"expires"`
	}
	if err := decodeBody(r, &b); err != nil {
		handleFileError(w, err)
		return
	}
	var expires *time.Time
	if b.Expires != "" {
		t, err := time.Parse(time.RFC3339, b.Expires)
		if err != nil || !t.After(time.Now()) {
			fail(w, 400, "Ngày hết hạn không hợp lệ.", "FILES_LINK_EXPIRY_INVALID")
			return
		}
		expires = &t
	}
	if len(b.Password) > 256 || (b.Password != "" && len(b.Password) < 6) {
		fail(w, 400, "Mật khẩu liên kết cần 6–256 ký tự.", "FILES_LINK_PASSWORD_INVALID")
		return
	}
	switch op {
	case "/grants", "/links":
		p, dir, err := s.ownItem(ctx, u.ID, b.Path)
		if err != nil {
			handleFileError(w, err)
			return
		}
		if op == "/grants" {
			if b.Permission != "view" && b.Permission != "edit" {
				fail(w, 400, "Quyền chia sẻ không hợp lệ.", "FILES_SHARE_PERMISSION_INVALID")
				return
			}
			g, err := s.Sharing.AddGrant(ctx, u.ID, Grant{Path: p, Dir: dir, Grantee: b.Recipient, Permission: b.Permission})
			if errors.Is(err, errNoRecipient) {
				fail(w, 400, "Không chia sẻ được với người này.", "FILES_SHARE_RECIPIENT_INVALID")
				return
			}
			if err != nil {
				unavailable(err)
				return
			}
			s.sharingAudit(ctx, u, "files.share_grant")
			jsonResponse(w, 200, g)
			return
		}
		if b.Permission != "view" && !(dir && (b.Permission == "edit" || b.Permission == "drop")) {
			fail(w, 400, "Quyền chia sẻ không hợp lệ.", "FILES_SHARE_PERMISSION_INVALID")
			return
		}
		hash := ""
		if b.Password != "" {
			if hash, err = authn.Hash(b.Password); err != nil {
				unavailable(err)
				return
			}
		}
		l, err := s.Sharing.AddLink(ctx, u.ID, Link{Path: p, Dir: dir, Permission: b.Permission, Expires: expires}, hash)
		if err != nil {
			var f *FileError
			if errors.As(err, &f) {
				handleFileError(w, err)
			} else {
				unavailable(err)
			}
			return
		}
		s.sharingAudit(ctx, u, "files.share_link")
		jsonResponse(w, 200, l)
	case "/links/update":
		if b.Permission != "view" && b.Permission != "edit" && b.Permission != "drop" {
			fail(w, 400, "Quyền chia sẻ không hợp lệ.", "FILES_SHARE_PERMISSION_INVALID")
			return
		}
		var hash *string
		if b.ClearPassword {
			empty := ""
			hash = &empty
		} else if b.Password != "" {
			h, err := authn.Hash(b.Password)
			if err != nil {
				unavailable(err)
				return
			}
			hash = &h
		}
		l, ok, err := s.Sharing.UpdateLink(ctx, u.ID, Link{ID: b.ID, Permission: b.Permission, Expires: expires}, hash)
		if err != nil {
			unavailable(err)
			return
		}
		if !ok {
			fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
			return
		}
		s.sharingAudit(ctx, u, "files.share_link_update")
		jsonResponse(w, 200, l)
	case "/grants/remove", "/links/remove":
		var ok bool
		var err error
		if op == "/grants/remove" {
			ok, err = s.Sharing.RemoveGrant(ctx, u.ID, b.ID)
		} else {
			ok, err = s.Sharing.RemoveLink(ctx, u.ID, b.ID)
		}
		if err != nil {
			unavailable(err)
			return
		}
		if !ok {
			fail(w, 404, "Không tìm thấy chia sẻ.", "SHARE_NOT_FOUND")
			return
		}
		s.sharingAudit(ctx, u, "files.share_remove")
		jsonResponse(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 404, "API không tồn tại.", "NOT_FOUND")
	}
}

// sharingAudit records who shared (never the path: personal homes keep names out of the audit).
func (s *Server) sharingAudit(ctx context.Context, u User, action string) {
	if err := s.Identity.Audit(ctx, u, AuditEvent{Action: action, Root: "personal"}); err != nil {
		logError(ctx, "files.audit_failed", err, "action", action)
	}
}

// ownItem checks that p is a file or folder (not the home itself, not a link or special file) in the user's home.
func (s *Server) ownItem(ctx context.Context, user, value string) (string, bool, error) {
	p, err := clean(value)
	if err != nil {
		return "", false, err
	}
	if p == "." {
		return "", false, problem(400, "FILES_ROOT_PROTECTED", "Không chia sẻ được cả thư mục gốc.")
	}
	dir, err := s.homeDir(ctx, user, false)
	if err != nil {
		return "", false, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", false, err
	}
	defer root.Close()
	e := &Engine{Root: root}
	f, _, err := e.open(p, 0)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", false, problem(400, "FILES_NOT_REGULAR", "Chỉ chia sẻ tệp thường hoặc thư mục.")
	}
	return p, info.IsDir(), nil
}

// sharedEngine opens the owner's home scoped to a shared item: a folder becomes the engine root, a file is the only
// name visible in its folder. It fails when the item is gone or changed kind.
func sharedEngine(dir, item string, isDir bool) (*Engine, *os.Root, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	e := &Engine{Root: root, Base: item}
	if !isDir {
		e.Base, e.Only = path.Dir(item), path.Base(item)
		if e.Base == "." {
			e.Base = ""
		}
	}
	f, _, err := e.open(".", 0)
	if err == nil {
		var info fs.FileInfo
		if info, err = f.Stat(); err == nil && !info.IsDir() {
			err = fs.ErrNotExist
		}
		f.Close()
	}
	if err == nil && e.Only != "" {
		_, err = e.Root.Lstat(e.disk(e.Only))
	}
	if err != nil {
		root.Close()
		return nil, nil, err
	}
	return e, root, nil
}

// linkKey signs the cookie that unlocks a password-protected link; it changes on restart (visitors type the
// password again) and the signature covers the password hash, so a new password logs everyone out.
var linkKey = sync.OnceValue(func() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
})

func linkProof(l Link) string {
	m := hmac.New(sha256.New, linkKey())
	m.Write([]byte(l.ID + "\n" + l.hash))
	return hex.EncodeToString(m.Sum(nil))
}

// servePublic answers /api/public/<token>[/unlock|/files/<op>] without a session (ADR 0011). view: list, download,
// archive; edit: also upload and new folder; drop: upload into the shared folder only, nothing is listed.
func (s *Server) servePublic(w http.ResponseWriter, r *http.Request) {
	if s.Sharing == nil {
		fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
		return
	}
	// Visitors have no session, so no CSRF token: writes (unlock, upload, new folder) must come from the Files page.
	if r.Method != "GET" && !sameOrigin(r) {
		fail(w, 403, "Nguồn yêu cầu không hợp lệ.", "ORIGIN_INVALID")
		return
	}
	ctx := r.Context()
	rest := strings.TrimPrefix(r.URL.Path, "/api/public/")
	token, op, _ := strings.Cut(rest, "/")
	if !validID(token) {
		fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
		return
	}
	l, ok, err := s.Sharing.LinkByToken(ctx, token)
	if err != nil {
		logError(ctx, "files.link.lookup_failed", err)
		fail(w, 503, "Không đọc được liên kết.", "SERVICE_UNAVAILABLE")
		return
	}
	if !ok {
		fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
		return
	}
	if l.Expires != nil && !l.Expires.After(time.Now()) {
		fail(w, 410, "Liên kết đã hết hạn.", "FILES_LINK_EXPIRED")
		return
	}
	cookie := "minhanos_link_" + l.ID
	unlocked := !l.HasPassword
	if c, e := r.Cookie(cookie); e == nil && hmac.Equal([]byte(c.Value), []byte(linkProof(l))) {
		unlocked = true
	}
	switch {
	case op == "" && r.Method == "GET":
		jsonResponse(w, 200, map[string]any{"name": l.Name, "dir": l.Dir, "permission": l.Permission, "owner": l.Owner, "needsPassword": !unlocked, "expires": l.Expires, "maxUploadBytes": MaxUpload})
		return
	case op == "unlock" && r.Method == "POST":
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.linkTries.allow(l.ID + ":" + ip) {
			fail(w, 429, "Thử quá nhiều lần; thử lại sau 10 phút.", "AUTH_RATE_LIMITED")
			return
		}
		var b struct {
			Password string `json:"password"`
		}
		if err = decodeBody(r, &b); err != nil {
			handleFileError(w, err)
			return
		}
		if !l.HasPassword || !authn.Verify(b.Password, l.hash) {
			fail(w, 403, "Mật khẩu không đúng.", "FILES_LINK_PASSWORD_WRONG")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: cookie, Value: linkProof(l), Path: "/api/public/" + token, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.SecureCookie, MaxAge: 12 * 3600})
		jsonResponse(w, 200, map[string]bool{"ok": true})
		return
	}
	if !unlocked {
		fail(w, 401, "Cần mật khẩu để mở liên kết.", "FILES_LINK_PASSWORD_REQUIRED")
		return
	}
	fileOp, isFiles := strings.CutPrefix(op, "files/")
	allowed := map[string][]string{"view": {"list", "download", "archive"}, "edit": {"list", "download", "archive", "upload", "mkdir"}, "drop": {"upload"}}[l.Permission]
	permitted := false
	for _, a := range allowed {
		permitted = permitted || a == fileOp
	}
	read := fileOp == "list" || fileOp == "download" || fileOp == "archive"
	if !isFiles || !permitted || read != (r.Method == "GET") {
		fail(w, 403, "Liên kết không cho phép thao tác này.", "FILES_LINK_FORBIDDEN")
		return
	}
	if l.Permission == "drop" && r.URL.Query().Get("path") != "" {
		fail(w, 403, "Liên kết không cho phép thao tác này.", "FILES_LINK_FORBIDDEN")
		return
	}
	if !validID(l.OwnerID) {
		fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
		return
	}
	v, _ := s.locks.LoadOrStore(l.OwnerID, &sync.RWMutex{})
	lock := v.(*sync.RWMutex)
	if !read {
		lock.Lock()
		defer lock.Unlock()
	} else if fileOp != "archive" {
		lock.RLock()
		defer lock.RUnlock()
	}
	dir, err := s.homeDir(ctx, l.OwnerID, false)
	if err != nil {
		if errors.Is(err, storagecore.ErrNoHomes) || errors.Is(err, fs.ErrNotExist) {
			fail(w, 404, "Không tìm thấy liên kết.", "FILES_LINK_NOT_FOUND")
		} else {
			logError(ctx, "files.link.home_failed", err)
			fail(w, 503, "Không mở được vùng lưu trữ.", "SERVICE_UNAVAILABLE")
		}
		return
	}
	engine, root, err := sharedEngine(dir, l.Path, l.Dir)
	if err != nil {
		fail(w, 404, "Mục được chia sẻ không còn tồn tại.", "FILES_LINK_NOT_FOUND")
		return
	}
	defer root.Close()
	if !read {
		// Writes by visitors count against the owner's quota and are audited as the owner's home.
		if err = s.Identity.Audit(ctx, User{ID: l.OwnerID}, AuditEvent{Action: "files.public_" + fileOp, Root: "personal"}); err != nil {
			fail(w, 503, "Chưa ghi được nhật ký; thao tác chưa thực hiện.", "AUDIT_UNAVAILABLE")
			return
		}
	}
	var reservation *storagecore.Reservation
	if !read && s.Quota != nil {
		if size := quotaSize(r, engine); size > 0 {
			res, e := s.Quota.Reserve(ctx, l.OwnerID, size)
			if e != nil {
				quotaFailure(w, e)
				return
			}
			reservation = &res
		}
	}
	r.URL.Path = "/api/files/" + fileOp
	if err = ServeEngine(w, r, engine); err != nil {
		handleFileError(w, err)
	}
	if reservation != nil {
		settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err == nil {
			err = s.Quota.Commit(settle, *reservation, quotaActual(r, engine, reservation.Bytes))
		} else {
			err = s.Quota.Release(settle, *reservation)
		}
		if err != nil {
			logError(ctx, "files.quota.settle_failed", err)
		}
	}
	if fileOp == "download" || fileOp == "archive" || fileOp == "upload" {
		if e := s.Sharing.UsedLink(context.WithoutCancel(ctx), l.ID); e != nil {
			logError(ctx, "files.link.count_failed", e)
		}
	}
}

// serveSharedPage answers /s/<token> with the Files web app, which shows the public page for that link.
func (s *Server) serveSharedPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, filepath.Join(s.StaticDir, "index.html"))
}

// retargetPlan works out, before the operation runs, which logical path a rename, move, trash or delete changes
// ([from, to]; to empty when the item goes away). Nil for other operations or without favorites and sharing.
func (s *Server) retargetPlan(r *http.Request, e *Engine) *[2]string {
	op := path.Base(r.URL.Path)
	if r.Method != "POST" || (s.Favorites == nil && s.Sharing == nil) || (op != "rename" && op != "move" && op != "trash" && op != "remove") {
		return nil
	}
	p, err := e.logical(r.URL.Query().Get("path"))
	if err != nil || p == "." {
		return nil
	}
	if op == "trash" || op == "remove" {
		return &[2]string{p, ""}
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var body struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
	}
	_ = json.Unmarshal(raw, &body) // ServeEngine validates strictly
	to := path.Join(path.Dir(p), body.Name)
	if op == "move" {
		if to, err = e.logical(body.Destination); err != nil {
			return nil
		}
	}
	return &[2]string{p, to}
}

// retarget applies a finished rename, move or removal to favorites and shares. Paths of a scoped engine (an item
// shared with the actor) are the owner's paths plus the scope; the actor's favorites of that share use its own paths.
func (s *Server) retarget(ctx context.Context, root, owner string, isShare bool, e *Engine, change *[2]string) {
	ctx = context.WithoutCancel(ctx)
	full := func(p string) string {
		if p == "" {
			return ""
		}
		return path.Join(e.Base, p)
	}
	var err error
	switch {
	case s.Sharing != nil && isShare:
		err = s.Sharing.Retarget(ctx, "", root, change[0], change[1])
	case s.Sharing != nil:
		// In a shared root the change is made in the owner home: its paths are the owner's, with the shared folder.
		err = s.Sharing.Retarget(ctx, owner, "personal", full(change[0]), full(change[1]))
	case s.Favorites != nil:
		err = s.Favorites.Retarget(ctx, owner, root, change[0], change[1])
	}
	if err != nil {
		logError(ctx, "files.retarget_failed", err)
	}
}
