package drive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Activity levels (ADR 0012), from least to most recorded; the System Admin picks one in Settings.
const (
	LevelMinimal = "minimal" // writes only; no path or name in homes and Shared Folders
	LevelWrites  = "writes"  // writes with paths and names
	LevelAccess  = "access"  // also reads, and denied or failed attempts
	LevelFull    = "full"    // also client, browser, duration, bytes, and what is open right now
)

var levelRank = map[string]int{LevelMinimal: 0, LevelWrites: 1, LevelAccess: 2, LevelFull: 3}

// ActivityStore keeps the level; nil means minimal (tests, standalone without a database).
type ActivityStore interface {
	Level(ctx context.Context) (string, int, error)
	// SetLevel changes the level when version is the current one (false otherwise).
	SetLevel(ctx context.Context, level string, version int, actor string) (bool, error)
}

type SQLActivity struct{ DB *pgxpool.Pool }

func (s SQLActivity) Level(ctx context.Context) (string, int, error) {
	var level string
	var version int
	err := s.DB.QueryRow(ctx, "SELECT audit_level,version FROM files_settings WHERE id=1").Scan(&level, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return LevelMinimal, 1, nil
	}
	return level, version, err
}

func (s SQLActivity) SetLevel(ctx context.Context, level string, version int, actor string) (bool, error) {
	tag, err := s.DB.Exec(ctx, `UPDATE files_settings SET audit_level=$1,version=version+1,updated_at=now(),
		updated_by=(SELECT public_id FROM users WHERE id=$3) WHERE id=1 AND version=$2`, level, version, actor)
	return err == nil && tag.RowsAffected() == 1, err
}

// activity holds the cached level and what people have open now (memory only, gone on restart).
type activity struct {
	mu        sync.Mutex
	level     string
	loaded    time.Time
	views     map[string]*liveEntry
	transfers map[uint64]*liveEntry
	next      uint64
}

// liveEntry is one person looking at a folder or file, or one transfer in progress.
type liveEntry struct {
	UserID      string    `json:"-"`
	User        string    `json:"user"`
	DisplayName string    `json:"displayName"`
	Kind        string    `json:"kind"` // view, open (a file in the editor), download, archive, upload
	Root        string    `json:"root"`
	RootName    string    `json:"rootName,omitempty"`
	Path        string    `json:"path"`
	File        string    `json:"file,omitempty"`
	Since       time.Time `json:"since"`
	Seen        time.Time `json:"seen"`
	Client      string    `json:"client"`
	Agent       string    `json:"agent"`
	Bytes       int64     `json:"bytes"`
	Total       int64     `json:"total,omitempty"`
	counter     *atomic.Int64
}

const (
	liveTTL      = 75 * time.Second // a view not refreshed (heartbeat every 30 s) is gone
	maxLiveViews = 5000
)

// activityLevel is the current level, re-read at most every 15 seconds.
func (s *Server) activityLevel(ctx context.Context) string {
	s.act.mu.Lock()
	defer s.act.mu.Unlock()
	if s.Activity == nil {
		return LevelMinimal
	}
	if s.act.level == "" || time.Since(s.act.loaded) > 15*time.Second {
		level, _, err := s.Activity.Level(ctx)
		if err != nil {
			logError(ctx, "files.activity.level_failed", err)
			if s.act.level == "" {
				return LevelMinimal
			}
			return s.act.level
		}
		s.act.level, s.act.loaded = level, time.Now()
	}
	return s.act.level
}

func (s *Server) records(ctx context.Context, level string) bool {
	return levelRank[s.activityLevel(ctx)] >= levelRank[level]
}

// client is the address of the person: the Desktop proxy appends it to X-Forwarded-For (its last entry); a direct
// request has its own remote address.
func client(r *http.Request) (addr, forwarded string) {
	addr, _, _ = net.SplitHostPort(r.RemoteAddr)
	if r.Header.Get("X-Drive-Auth-Mode") == "platform" {
		if list := strings.Split(r.Header.Get("X-Forwarded-For"), ","); len(list) > 0 {
			if last := strings.TrimSpace(list[len(list)-1]); net.ParseIP(last) != nil {
				return last, ""
			}
		}
	}
	return addr, strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
}

func agent(r *http.Request) string {
	a := r.UserAgent()
	if len(a) > 200 {
		a = a[:200]
	}
	return a
}

// clientDetails adds what level full records about the client.
func clientDetails(r *http.Request, details map[string]any, started time.Time) {
	addr, forwarded := client(r)
	details["client"] = addr
	if forwarded != "" {
		details["forwardedFor"] = forwarded
	}
	details["agent"] = agent(r)
	if !started.IsZero() {
		details["ms"] = time.Since(started).Milliseconds()
	}
}

// activityWriter keeps the status, the bytes sent and the start of an error body (its stable code).
type activityWriter struct {
	http.ResponseWriter
	status int
	bytes  atomic.Int64
	head   []byte
}

func (w *activityWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *activityWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.status >= 400 && len(w.head) < 512 {
		w.head = append(w.head, b[:min(len(b), 512-len(w.head))]...)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes.Add(int64(n))
	return n, err
}

func (w *activityWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *activityWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *activityWriter) outcome() (string, string) {
	status := w.status
	if status == 0 {
		status = 200
	}
	if status < 400 {
		return "success", ""
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.head, &body)
	if status == 401 || status == 403 {
		return "denied", body.Code
	}
	return "failure", body.Code
}

// readOps are the Files reads recorded from level access on; list pages after the first are not.
var readOps = map[string]bool{"list": true, "download": true, "text": true, "archive": true, "search": true, "recent": true, "trash": true}

// liveKey is one browser of one person.
func liveKey(u User, r *http.Request) string {
	addr, _ := client(r)
	h := sha256.Sum256([]byte(u.ID + "\x00" + addr + "\x00" + r.UserAgent()))
	return hex.EncodeToString(h[:12])
}

// see records that u looks at root/path (and file, when one is open), from a folder listing or the app's heartbeat.
func (s *Server) see(u User, r *http.Request, root, rootName, p, file string) {
	now := time.Now()
	key := liveKey(u, r)
	addr, _ := client(r)
	s.act.mu.Lock()
	defer s.act.mu.Unlock()
	if s.act.views == nil {
		s.act.views = map[string]*liveEntry{}
	}
	for k, v := range s.act.views {
		if now.Sub(v.Seen) > liveTTL {
			delete(s.act.views, k)
		}
	}
	e := s.act.views[key]
	if e == nil {
		if len(s.act.views) >= maxLiveViews {
			return
		}
		e = &liveEntry{UserID: u.ID, User: u.Username, DisplayName: u.DisplayName, Since: now, Client: addr, Agent: agent(r)}
		s.act.views[key] = e
	}
	kind := "view"
	if file != "" {
		kind = "open"
	}
	if e.Root != root || e.Path != p || e.File != file {
		e.Since = now
	}
	if rootName != "" || e.Root != root {
		e.RootName = rootName
	}
	e.Kind, e.Root, e.Path, e.File, e.Seen = kind, root, p, file, now
}

// transfer lists a download, ZIP or upload while it runs; the returned function ends it.
func (s *Server) transfer(u User, r *http.Request, kind, root, p string, counter *atomic.Int64) func() {
	addr, _ := client(r)
	s.act.mu.Lock()
	if s.act.transfers == nil {
		s.act.transfers = map[uint64]*liveEntry{}
	}
	s.act.next++
	id := s.act.next
	total, _ := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if kind != "upload" {
		total = 0
	}
	s.act.transfers[id] = &liveEntry{UserID: u.ID, User: u.Username, DisplayName: u.DisplayName, Kind: kind, Root: root, Path: p, Since: time.Now(), Seen: time.Now(), Client: addr, Agent: agent(r), Total: total, counter: counter}
	s.act.mu.Unlock()
	return func() {
		s.act.mu.Lock()
		delete(s.act.transfers, id)
		s.act.mu.Unlock()
	}
}

func (s *Server) live() []liveEntry {
	now := time.Now()
	s.act.mu.Lock()
	defer s.act.mu.Unlock()
	out := []liveEntry{}
	for k, v := range s.act.views {
		if now.Sub(v.Seen) > liveTTL {
			delete(s.act.views, k)
			continue
		}
		out = append(out, *v)
	}
	for _, v := range s.act.transfers {
		e := *v
		if v.counter != nil {
			e.Bytes = v.counter.Load()
		}
		e.Seen = now
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].User != out[j].User {
			return out[i].User < out[j].User
		}
		return out[i].Since.Before(out[j].Since)
	})
	return out
}

// countingBody counts the bytes of an upload as they arrive.
type countingBody struct {
	r interface {
		Read([]byte) (int, error)
		Close() error
	}
	n *atomic.Int64
}

func (b countingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n.Add(int64(n))
	return n, err
}
func (b countingBody) Close() error { return b.r.Close() }

// serveActivity: POST /api/activity/presence {root, rootName, path, file} — the Files app reports what is open
// (stored at level full only); GET /api/activity/live — who has what open and the transfers running (System Admin);
// GET|PUT /api/activity/settings — the level (System Admin; PUT needs If-Match with the version).
func (s *Server) serveActivity(w http.ResponseWriter, r *http.Request, u User) {
	admin := u.Role == "admin"
	switch {
	case r.URL.Path == "/api/activity/presence" && r.Method == "POST":
		var b struct {
			Root     string `json:"root"`
			RootName string `json:"rootName"`
			Path     string `json:"path"`
			File     string `json:"file"`
		}
		if err := decodeBody(r, &b); err != nil {
			handleFileError(w, err)
			return
		}
		if len(b.Root) > 200 || len(b.RootName) > 200 || len(b.Path) > 4096 || len(b.File) > 4096 {
			fail(w, 400, "Dữ liệu yêu cầu không hợp lệ.", "INVALID_REQUEST")
			return
		}
		stored := s.records(r.Context(), LevelFull)
		if stored {
			s.see(u, r, b.Root, b.RootName, b.Path, b.File)
		}
		jsonResponse(w, 200, map[string]bool{"stored": stored})
	case r.URL.Path == "/api/activity/live" && r.Method == "GET":
		if !admin {
			fail(w, 403, "Chỉ System Admin xem được ai đang truy cập.", "ACCESS_DENIED")
			return
		}
		level := s.activityLevel(r.Context())
		items := []liveEntry{}
		if level == LevelFull {
			items = s.live()
		}
		jsonResponse(w, 200, map[string]any{"level": level, "items": items})
	case r.URL.Path == "/api/activity/settings" && (r.Method == "GET" || r.Method == "PUT"):
		if !admin {
			fail(w, 403, "Chỉ System Admin đổi được mức nhật ký.", "ACCESS_DENIED")
			return
		}
		if s.Activity == nil {
			fail(w, 503, "Nhật ký hoạt động chưa được bật.", "SERVICE_UNAVAILABLE")
			return
		}
		level, version, err := s.Activity.Level(r.Context())
		if err != nil {
			logError(r.Context(), "files.activity.level_failed", err)
			fail(w, 503, "Không đọc được cấu hình nhật ký.", "SERVICE_UNAVAILABLE")
			return
		}
		if r.Method == "GET" {
			w.Header().Set("ETag", `"`+strconv.Itoa(version)+`"`)
			jsonResponse(w, 200, map[string]any{"level": level, "version": version, "retentionDays": AuditRetentionDays})
			return
		}
		match := strings.Trim(r.Header.Get("If-Match"), `"`)
		if match == "" {
			fail(w, 428, "Thiếu If-Match.", "PRECONDITION_REQUIRED")
			return
		}
		var b struct {
			Level string `json:"level"`
		}
		if err = decodeBody(r, &b); err != nil {
			handleFileError(w, err)
			return
		}
		if _, ok := levelRank[b.Level]; !ok {
			fail(w, 400, "Mức nhật ký không hợp lệ.", "FILES_ACTIVITY_LEVEL_INVALID")
			return
		}
		want, err := strconv.Atoi(match)
		if err != nil || want != version {
			fail(w, 412, "Cấu hình đã đổi ở nơi khác; tải lại rồi thử lại.", "VERSION_CONFLICT")
			return
		}
		// The change itself is always recorded, before it takes effect.
		if err = s.Identity.Audit(r.Context(), u, AuditEvent{Action: "files.activity_level", Root: "personal", Details: map[string]any{"from": level, "to": b.Level}}); err != nil {
			fail(w, 503, "Chưa ghi được nhật ký; thao tác chưa thực hiện.", "AUDIT_UNAVAILABLE")
			return
		}
		ok, err := s.Activity.SetLevel(r.Context(), b.Level, version, u.ID)
		if err != nil {
			logError(r.Context(), "files.activity.set_failed", err)
			fail(w, 503, "Không lưu được cấu hình nhật ký.", "SERVICE_UNAVAILABLE")
			return
		}
		if !ok {
			fail(w, 412, "Cấu hình đã đổi ở nơi khác; tải lại rồi thử lại.", "VERSION_CONFLICT")
			return
		}
		s.act.mu.Lock()
		s.act.level, s.act.loaded = b.Level, time.Now()
		if b.Level != LevelFull {
			s.act.views = nil
		}
		s.act.mu.Unlock()
		w.Header().Set("ETag", `"`+strconv.Itoa(version+1)+`"`)
		jsonResponse(w, 200, map[string]any{"level": b.Level, "version": version + 1, "retentionDays": AuditRetentionDays})
	default:
		fail(w, 404, "API không tồn tại.", "NOT_FOUND")
	}
}

// peekBody reads the JSON body of a write (name, destination, trash item) for the record and puts it back.
func peekBody(r *http.Request) map[string]any {
	if r.Body == nil || r.URL.Path == "/api/files/upload" {
		return nil
	}
	raw, err := readAllLimited(r)
	if err != nil {
		return nil
	}
	var b struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		ID          string `json:"id"`
	}
	if json.Unmarshal(raw, &b) != nil {
		return nil
	}
	out := map[string]any{}
	if b.Name != "" {
		out["name"] = b.Name
	}
	if b.Destination != "" {
		out["destination"] = b.Destination
	}
	if b.ID != "" {
		out["trashItem"] = b.ID
	}
	return out
}

func readAllLimited(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, MaxText*6+8192))
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return raw, err
}

// filesRecord is what one /api/files request records, filled in as the handler learns it.
type filesRecord struct {
	op       string
	mutation bool
	root     string // event root: personal, share, system
	share    string
	rootID   string // as the client named it (personal, share:<key>, shared:<id>, system)
	path     string
	details  map[string]any
	audited  bool // the write was recorded before it ran
	started  time.Time
}

// recordFiles records, after the response, what the level asks for: failures of recorded writes (writes), refused
// writes and every read (access), with the client (full). A failed record is logged, never shown to the user.
func (s *Server) recordFiles(r *http.Request, u User, w *activityWriter, rec *filesRecord) {
	ctx := r.Context()
	rank := levelRank[s.activityLevel(ctx)]
	outcome, code := w.outcome()
	write := func(action string) {
		details := map[string]any{}
		for k, v := range rec.details {
			details[k] = v
		}
		if code != "" {
			details["code"] = code
		}
		if rank >= levelRank[LevelFull] {
			clientDetails(r, details, rec.started)
		}
		ev := AuditEvent{Action: action, Root: rec.root, Share: rec.share, Outcome: outcome, Details: details}
		if rank >= levelRank[LevelWrites] || rec.root == "system" {
			ev.Path = rec.path
		}
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.Identity.Audit(c, u, ev); err != nil {
			logError(ctx, "files.activity.record_failed", err, "action", action)
		}
	}
	action := "files." + strings.ReplaceAll(rec.op, "-", "_")
	if rec.mutation {
		switch {
		case rec.audited && outcome != "success" && rank >= levelRank[LevelWrites]:
			write(action)
		case !rec.audited && outcome != "success" && rank >= levelRank[LevelAccess]:
			write(action)
		}
		return
	}
	if !readOps[rec.op] || rank < levelRank[LevelAccess] || (rec.op == "list" && r.URL.Query().Get("cursor") != "") {
		return
	}
	if rec.root == "" {
		rec.root = "personal"
	}
	if rec.details == nil {
		rec.details = map[string]any{}
	}
	switch rec.op {
	case "search":
		rec.details["query"] = r.URL.Query().Get("q")
	case "archive":
		rec.details["items"] = r.URL.Query()["item"]
		rec.details["bytes"] = w.bytes.Load()
	case "download", "text":
		rec.details["bytes"] = w.bytes.Load()
	}
	if rank >= levelRank[LevelFull] && outcome == "success" {
		switch rec.op {
		case "list":
			s.see(u, r, rec.rootID, "", rec.path, "")
		case "text":
			s.see(u, r, rec.rootID, "", parentPath(rec.path), rec.path)
		}
	}
	write(action)
}

func parentPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// grantPath is where p of a shared root lies in the owner's home.
func grantPath(g Grant, p string) string {
	base := g.Path
	if !g.Dir {
		base = parentPath(g.Path)
	}
	if p == "" || p == "." {
		return base
	}
	if base == "" {
		return p
	}
	return base + "/" + p
}

// publicDetails describes a public-link request: the link always; its item and names from level writes on; the
// visitor's address and browser at full.
func (s *Server) publicDetails(r *http.Request, l Link) map[string]any {
	ctx := r.Context()
	d := map[string]any{"via": "link"}
	if l.ID != "" {
		d["link"] = l.ID
	}
	if s.records(ctx, LevelWrites) && l.ID != "" {
		d["owner"], d["linkPath"], d["permission"] = l.Owner, l.Path, l.Permission
		if n := r.URL.Query().Get("name"); n != "" {
			d["name"] = n
		}
		if items := r.URL.Query()["item"]; len(items) > 0 {
			d["items"] = items
		}
	}
	if s.records(ctx, LevelFull) {
		clientDetails(r, d, time.Time{})
	}
	return d
}

// recordPublic records a visitor's request from level access on (writes that ran were recorded before).
func (s *Server) recordPublic(r *http.Request, l *Link, op string, w *activityWriter, started time.Time, audited *bool) {
	ctx := r.Context()
	if !s.records(ctx, LevelAccess) {
		return
	}
	outcome, code := w.outcome()
	if *audited && outcome == "success" {
		return
	}
	name := strings.TrimPrefix(op, "files/")
	switch {
	case op == "":
		name = "open"
	case !actionName.MatchString(name):
		name = "other"
	}
	d := s.publicDetails(r, *l)
	if code != "" {
		d["code"] = code
	}
	if outcome == "success" && (name == "download" || name == "archive") {
		d["bytes"] = w.bytes.Load()
	}
	if s.records(ctx, LevelFull) {
		d["ms"] = time.Since(started).Milliseconds()
		if name == "list" && outcome == "success" && l.ID != "" {
			s.see(User{ID: "link:" + l.ID}, r, "link:"+l.ID, l.Name, r.URL.Query().Get("path"), "")
		}
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Identity.Audit(c, User{}, AuditEvent{Action: "files.public_" + name, Root: "personal", Path: r.URL.Query().Get("path"), Outcome: outcome, Details: d}); err != nil {
		logError(ctx, "files.activity.record_failed", err, "action", "files.public_"+name)
	}
}

var actionName = regexp.MustCompile(`^[a-z]{1,20}$`)
