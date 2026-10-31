package drive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
	"minhanos.local/platform/internal/authn"
)

const MaxUpload = 128 << 20
const MaxText = 256 << 10

// FileError carries a stable code (app standard row 8); Message is a developer hint, never shown by the UI.
type FileError struct {
	Status  int
	Code    string
	Message string
}

func (e *FileError) Error() string { return e.Message }
func problem(status int, code, message string) error {
	return &FileError{status, code, message}
}

type Entry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Kind     string    `json:"kind"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Mode     string    `json:"mode"`
	UID      uint32    `json:"uid"`
	GID      uint32    `json:"gid"`
	ETag     string    `json:"etag"`
}
type TrashItem struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Deleted time.Time `json:"deleted"`
}

// Engine owns one rooted namespace. Public paths never address its private trash journal.
// A shared item (ADR 0011) is served by an engine scoped to it: Base is the shared folder (or the folder holding the
// shared file) inside the owner's files, Only limits a file share to that one name. Scoped engines have no trash
// view; items they trash land in the owner's trash under the owner's path.
type Engine struct {
	Root *os.Root
	Host bool
	Base string
	Only string
}

func clean(value string) (string, error) {
	if len(value) > 4096 || strings.ContainsAny(value, "\x00\\") {
		return "", problem(400, "FILES_PATH_INVALID", "Đường dẫn không hợp lệ.")
	}
	for _, s := range strings.Split(value, "/") {
		if s == ".." {
			return "", problem(400, "FILES_PATH_OUTSIDE", "Không được đi ra ngoài không gian lưu trữ.")
		}
	}
	p := strings.TrimPrefix(path.Clean("/"+value), "/")
	if p == "" {
		p = "."
	}
	return p, nil
}
func validName(name string) bool {
	return utf8.ValidString(name) && len(name) > 0 && len(name) <= 255 && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00\n\r")
}
func (e *Engine) logical(value string) (string, error) {
	p, err := clean(value)
	if err != nil {
		return "", err
	}
	if e.Only != "" && p != "." && p != e.Only && !strings.HasPrefix(p, e.Only+"/") {
		return "", problem(404, "FILES_NOT_FOUND", "Không tìm thấy tệp hoặc thư mục.")
	}
	if e.Host {
		first := strings.Split(p, "/")[0]
		if first == "proc" || first == "sys" || first == "dev" || first == "run" {
			return "", problem(403, "FILES_VIRTUAL_FS", "Filesystem ảo và thiết bị không được quản lý trong File Station.")
		}
	}
	return p, nil
}
func (e *Engine) disk(p string) string {
	if e.Host {
		return p
	}
	return path.Join("files", e.Base, p)
}

// scoped engines (a shared item) never show or change the owner's trash.
func (e *Engine) scoped() bool { return e.Base != "" || e.Only != "" }

// Each parent is opened relative to a rooted handle. Reject symlinks instead of following
// them into unexpected namespaces, and use O_NOFOLLOW on the final directory/file.
func (e *Engine) parent(value string) (*os.File, string, string, error) {
	p, err := e.logical(value)
	if err != nil {
		return nil, "", "", err
	}
	disk := e.disk(p)
	current, err := e.Root.Open(".")
	if err != nil {
		return nil, "", "", err
	}
	parts := strings.Split(path.Dir(disk), "/")
	for _, part := range parts {
		if part == "." || part == "" {
			continue
		}
		fd, er := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		current.Close()
		if er != nil {
			return nil, "", "", er
		}
		current = os.NewFile(uintptr(fd), part)
	}
	return current, path.Base(disk), p, nil
}
func (e *Engine) open(value string, flags int) (*os.File, string, error) {
	parent, name, p, err := e.parent(value)
	if err != nil {
		return nil, "", err
	}
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, "", err
	}
	return os.NewFile(uintptr(fd), name), p, nil
}
func etag(fi os.FileInfo) string {
	st, _ := fi.Sys().(*syscall.Stat_t)
	inode := uint64(0)
	if st != nil {
		inode = st.Ino
	}
	return fmt.Sprintf("%x-%x-%x", inode, fi.Size(), fi.ModTime().UnixNano())
}
func entry(fi os.FileInfo, p string) Entry {
	kind := "special"
	if fi.Mode().IsRegular() {
		kind = "file"
	} else if fi.IsDir() {
		kind = "folder"
	} else if fi.Mode()&os.ModeSymlink != 0 {
		kind = "symlink"
	}
	v := Entry{Name: fi.Name(), Path: p, Kind: kind, Size: fi.Size(), Modified: fi.ModTime(), Mode: fi.Mode().String(), ETag: etag(fi)}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		v.UID = st.Uid
		v.GID = st.Gid
	}
	return v
}
func (e *Engine) List(value string) ([]Entry, error) {
	f, p, err := e.open(value, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(10001)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > 10000 {
		return nil, problem(422, "FILES_FOLDER_LIMIT", "Thư mục vượt giới hạn 10.000 mục của bản đầu.")
	}
	out := []Entry{}
	for _, d := range entries {
		fd, er := unix.Openat(int(f.Fd()), d.Name(), unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if er != nil {
			continue
		}
		child := os.NewFile(uintptr(fd), d.Name())
		fi, er := child.Stat()
		child.Close()
		if er != nil {
			continue
		}
		if e.Only != "" && p == "." && d.Name() != e.Only {
			continue
		}
		out = append(out, entry(fi, path.Join(p, d.Name())))
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Kind == "folder") != (out[j].Kind == "folder") {
			return out[i].Kind == "folder"
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}
func (e *Engine) Read(value string) (*os.File, os.FileInfo, error) {
	f, _, err := e.open(value, unix.O_RDONLY)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = problem(400, "FILES_NOT_REGULAR", "Chỉ hỗ trợ tệp thông thường; không mở symlink, thiết bị hoặc socket.")
		}
		return nil, nil, err
	}
	return f, fi, nil
}
func (e *Engine) Mkdir(value, name string) error {
	if !validName(name) {
		return problem(400, "FILES_NAME_INVALID", "Tên thư mục không hợp lệ.")
	}
	p, err := e.logical(path.Join(value, "."))
	if err != nil {
		return err
	}
	f, _, err := e.open(p, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return err
	}
	defer f.Close()
	return unix.Mkdirat(int(f.Fd()), name, 0750)
}
func (e *Engine) Rename(source, destination string) error {
	src, sn, sp, err := e.parent(source)
	if err != nil {
		return err
	}
	defer src.Close()
	if sp == "." {
		return problem(400, "FILES_ROOT_PROTECTED", "Không thể di chuyển thư mục gốc.")
	}
	dst, dn, dp, err := e.parent(destination)
	if err != nil {
		return err
	}
	defer dst.Close()
	if dp == "." {
		return problem(400, "FILES_DESTINATION_INVALID", "Đích không hợp lệ.")
	}
	var st unix.Stat_t
	if err = unix.Fstatat(int(src.Fd()), sn, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG && st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return problem(400, "FILES_NOT_REGULAR", "Không di chuyển tệp đặc biệt hoặc symlink.")
	}
	return renameNoReplace(int(src.Fd()), sn, int(dst.Fd()), dn)
}
func (e *Engine) Upload(value, name string, body io.Reader) (Entry, error) {
	if !validName(name) {
		return Entry{}, problem(400, "FILES_NAME_INVALID", "Tên tệp không hợp lệ.")
	}
	dir, p, err := e.open(value, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return Entry{}, err
	}
	defer dir.Close()
	temp := ".upload-" + authn.Token()
	fd, err := unix.Openat(int(dir.Fd()), temp, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC, 0640)
	if err != nil {
		return Entry{}, err
	}
	f := os.NewFile(uintptr(fd), temp)
	defer func() { f.Close(); _ = unix.Unlinkat(int(dir.Fd()), temp, 0) }()
	count, err := io.Copy(f, io.LimitReader(body, MaxUpload+1))
	if err != nil {
		return Entry{}, err
	}
	if count > MaxUpload {
		return Entry{}, problem(413, "FILES_UPLOAD_TOO_LARGE", "Tệp vượt giới hạn 128 MB của bản đầu.")
	}
	if err = f.Sync(); err != nil {
		return Entry{}, err
	}
	if err = renameNoReplace(int(dir.Fd()), temp, int(dir.Fd()), name); err != nil {
		return Entry{}, err
	}
	_ = dir.Sync()
	fi, err := f.Stat()
	if err != nil {
		return Entry{}, err
	}
	v := entry(fi, path.Join(p, name))
	v.Name = name
	return v, nil
}
func (e *Engine) Text(value string) (string, string, error) {
	f, fi, err := e.Read(value)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	if fi.Size() > MaxText {
		return "", "", problem(413, "FILES_TEXT_TOO_LARGE", "Xem/sửa văn bản giới hạn 256 KB.")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxText+1))
	if err != nil {
		return "", "", err
	}
	if len(b) > MaxText || !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return "", "", problem(415, "FILES_NOT_TEXT", "Tệp không phải văn bản UTF-8 được hỗ trợ.")
	}
	return string(b), etag(fi), nil
}

func (e *Engine) Copy(source, destination string) (Entry, error) {
	src, fi, err := e.Read(source)
	if err != nil {
		return Entry{}, err
	}
	defer src.Close()
	if fi.Size() > MaxUpload {
		return Entry{}, problem(413, "FILES_COPY_TOO_LARGE", "Sao chép tệp giới hạn 128 MB trong bản này.")
	}
	dest, err := e.logical(destination)
	if err != nil {
		return Entry{}, err
	}
	if dest == "." {
		return Entry{}, problem(400, "FILES_DESTINATION_INVALID", "Đích sao chép không hợp lệ.")
	}
	return e.Upload(path.Dir(dest), path.Base(dest), src)
}

// SaveText checks optimistic concurrency and keeps a backup before atomic replacement.
func (e *Engine) SaveText(value, content, expected string) (string, error) {
	if len(content) > MaxText || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return "", problem(413, "FILES_TEXT_TOO_LARGE", "Văn bản không hợp lệ hoặc quá 256 KB.")
	}
	old, fi, err := e.Read(value)
	if err != nil {
		return "", err
	}
	defer old.Close()
	if expected == "" || etag(fi) != expected {
		return "", problem(409, "FILES_CHANGED", "Tệp đã thay đổi. Nạp lại trước khi lưu.")
	}
	if fi.Size() > MaxText {
		return "", problem(413, "FILES_TEXT_TOO_LARGE", "Không sửa tệp lớn hơn 256 KB.")
	}
	dir, name, _, err := e.parent(value)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	// Hard links and config mountpoints require semantics that this editor does not promise.
	st := fi.Sys().(*syscall.Stat_t)
	if st.Nlink != 1 {
		return "", problem(409, "FILES_EDIT_UNSUPPORTED", "Tệp có hard link; không hỗ trợ thay thế trong trình sửa này.")
	}
	if fi.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return "", problem(409, "FILES_EDIT_UNSUPPORTED", "Tệp có mode đặc biệt; không thay thế trong trình sửa này.")
	}
	if e.Host {
		n, er := unix.Flistxattr(int(old.Fd()), nil)
		if er != nil && !errors.Is(er, unix.ENOTSUP) {
			return "", er
		}
		if n > 0 {
			return "", problem(409, "FILES_EDIT_UNSUPPORTED", "Tệp có ACL/xattr; bản đầu không thay thế để tránh mất thuộc tính.")
		}
	}
	backup := name + ".minhanos-backup-" + time.Now().UTC().Format("20060102T150405") + "-" + authn.Token()[:8]
	bfd, err := unix.Openat(int(dir.Fd()), backup, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC, 0600)
	if err != nil {
		return "", err
	}
	bf := os.NewFile(uintptr(bfd), backup)
	_, err = io.Copy(bf, old)
	if err == nil {
		err = bf.Sync()
	}
	bf.Close()
	if err != nil {
		return "", err
	}
	temp := ".edit-" + authn.Token()
	fd, err := unix.Openat(int(dir.Fd()), temp, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC, 0600)
	if err != nil {
		return "", err
	}
	f := os.NewFile(uintptr(fd), temp)
	defer func() { f.Close(); _ = unix.Unlinkat(int(dir.Fd()), temp, 0) }()
	if _, err = f.WriteString(content); err != nil {
		return "", err
	}
	if e.Host {
		if err = f.Chown(int(st.Uid), int(st.Gid)); err != nil {
			return "", err
		}
	}
	if err = f.Chmod(fi.Mode().Perm()); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	var latest unix.Stat_t
	if err = unix.Fstatat(int(dir.Fd()), name, &latest, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return "", err
	}
	if latest.Ino != st.Ino || latest.Mtim.Sec != st.Mtim.Sec || latest.Mtim.Nsec != st.Mtim.Nsec || latest.Size != st.Size {
		return "", problem(409, "FILES_CHANGED", "Tệp đã được thay đổi trong lúc lưu.")
	}
	if err = unix.Renameat(int(dir.Fd()), temp, int(dir.Fd()), name); err != nil {
		return "", err
	}
	_ = dir.Sync()
	return backup, nil
}
func (e *Engine) Trash(value string) (TrashItem, error) {
	if e.Host {
		return TrashItem{}, problem(400, "FILES_NO_SYSTEM_TRASH", "Tệp hệ thống không có thùng rác; dùng thao tác xóa riêng.")
	}
	p, err := e.logical(value)
	if err != nil {
		return TrashItem{}, err
	}
	if p == "." {
		return TrashItem{}, problem(400, "FILES_ROOT_PROTECTED", "Không xóa thư mục gốc.")
	}
	item := TrashItem{ID: authn.Token(), Path: path.Join(e.Base, p), Name: path.Base(p), Deleted: time.Now().UTC()}
	dir := path.Join("trash", item.ID)
	if err = e.Root.Mkdir(dir, 0700); err != nil {
		return item, err
	}
	metadata, err := e.Root.OpenFile(path.Join(dir, "meta.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return item, err
	}
	err = json.NewEncoder(metadata).Encode(item)
	if err == nil {
		err = metadata.Sync()
	}
	metadata.Close()
	if err != nil {
		return item, err
	}
	// Journal is written first. Recovery lists an item only when its content exists.
	src, sn, _, err := e.parent(p)
	if err != nil {
		return item, err
	}
	defer src.Close()
	dest, err := e.Root.Open(dir)
	if err != nil {
		return item, err
	}
	defer dest.Close()
	err = renameNoReplace(int(src.Fd()), sn, int(dest.Fd()), "content")
	return item, err
}
func (e *Engine) TrashList() ([]TrashItem, error) {
	if e.scoped() {
		return nil, problem(403, "FILES_SHARED_NO_TRASH", "Mục được chia sẻ không có thùng rác riêng.")
	}
	if e.Host {
		return nil, problem(400, "FILES_NO_SYSTEM_TRASH", "Không có thùng rác hệ thống.")
	}
	f, err := e.Root.Open("trash")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dirs, err := f.ReadDir(10001)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(dirs) > 10000 {
		return nil, problem(422, "FILES_TRASH_LIMIT", "Thùng rác vượt giới hạn.")
	}
	items := []TrashItem{}
	for _, d := range dirs {
		if !validID(d.Name()) {
			continue
		}
		if _, err = e.Root.Lstat(path.Join("trash", d.Name(), "content")); err != nil {
			continue
		}
		meta, er := e.Root.Open(path.Join("trash", d.Name(), "meta.json"))
		if er != nil {
			continue
		}
		var v TrashItem
		er = json.NewDecoder(io.LimitReader(meta, 8192)).Decode(&v)
		meta.Close()
		if er == nil && v.ID == d.Name() {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Deleted.After(items[j].Deleted) })
	return items, nil
}
func (e *Engine) Restore(id string) error {
	if e.scoped() {
		return problem(403, "FILES_SHARED_NO_TRASH", "Mục được chia sẻ không có thùng rác riêng.")
	}
	if e.Host || !validID(id) {
		return problem(400, "FILES_TRASH_ITEM_INVALID", "Mục khôi phục không hợp lệ.")
	}
	meta, err := e.Root.Open(path.Join("trash", id, "meta.json"))
	if err != nil {
		return err
	}
	var v TrashItem
	err = json.NewDecoder(io.LimitReader(meta, 8192)).Decode(&v)
	meta.Close()
	if err != nil {
		return err
	}
	dst, dn, _, err := e.parent(v.Path)
	if err != nil {
		return err
	}
	defer dst.Close()
	src, err := e.Root.Open(path.Join("trash", id))
	if err != nil {
		return err
	}
	defer src.Close()
	if err = renameNoReplace(int(src.Fd()), "content", int(dst.Fd()), dn); err != nil {
		return err
	}
	_ = e.Root.Remove(path.Join("trash", id, "meta.json"))
	_ = e.Root.Remove(path.Join("trash", id))
	return nil
}
func (e *Engine) Remove(value string) error {
	if !e.Host {
		return problem(400, "FILES_USE_TRASH", "Dùng Thùng rác cho tệp cá nhân.")
	}
	dir, name, p, err := e.parent(value)
	if err != nil {
		return err
	}
	defer dir.Close()
	if p == "." {
		return problem(400, "FILES_ROOT_PROTECTED", "Không được xóa thư mục gốc.")
	}
	var st unix.Stat_t
	if err = unix.Fstatat(int(dir.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	flags := 0
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		flags = unix.AT_REMOVEDIR
	} else if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return problem(400, "FILES_NOT_REGULAR", "Không xóa tệp đặc biệt hoặc symlink.")
	}
	return unix.Unlinkat(int(dir.Fd()), name, flags)
}
func sendFile(w http.ResponseWriter, r *http.Request, e *Engine, p string) error {
	f, fi, err := e.Read(p)
	if err != nil {
		return err
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fi.Name()}))
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	return nil
}
func validID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func handleFileError(w http.ResponseWriter, err error) {
	var fe *FileError
	if errors.As(err, &fe) {
		fail(w, fe.Status, fe.Message, fe.Code)
		return
	}
	status, code := 500, "INTERNAL_ERROR"
	message := "Không thể hoàn thành thao tác tệp."
	switch {
	case errors.Is(err, os.ErrNotExist):
		status = 404
		code = "FILES_NOT_FOUND"
		message = "Không tìm thấy tệp hoặc thư mục."
	case errors.Is(err, os.ErrExist):
		status = 409
		code = "FILES_EXISTS"
		message = "Tên đã tồn tại; không ghi đè tự động."
	case errors.Is(err, os.ErrPermission):
		status = 403
		code = "FILES_PERMISSION"
		message = "Không có quyền truy cập tệp."
	case errors.Is(err, unix.ELOOP):
		status = 403
		code = "FILES_SYMLINK"
		message = "Không theo symlink trong bản đầu."
	case errors.Is(err, unix.ENOTDIR):
		status = 400
		code = "FILES_NOT_DIRECTORY"
		message = "Đường dẫn không phải thư mục hoặc chứa symlink."
	case errors.Is(err, unix.ENOTEMPTY):
		status = 409
		code = "FILES_NOT_EMPTY"
		message = "Chỉ xóa thư mục hệ thống rỗng."
	case errors.Is(err, unix.EXDEV):
		status = 409
		code = "FILES_CROSS_DEVICE"
		message = "Bản đầu chưa hỗ trợ di chuyển qua filesystem khác."
	case errors.Is(err, unix.ENOSPC):
		status = 507
		code = "STORAGE_FULL"
		message = "Không đủ dung lượng lưu trữ."
	}
	fail(w, status, message, code)
}
func checksum(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

// renameNoReplace renames without ever replacing the target. Filesystems without RENAME_NOREPLACE (OpenZFS before
// 2.2 answers EINVAL) fall back to link+unlink for files, which also fails with EEXIST, and to check-then-rename for
// directories (hard links are impossible there), where a directory created between the check and the rename could
// be replaced only if it is empty (rename(2) refuses a non-empty target).
var renameat2 = unix.Renameat2 // replaced in tests to exercise the fallback

func renameNoReplace(srcDir int, src string, dstDir int, dst string) error {
	err := renameat2(srcDir, src, dstDir, dst, unix.RENAME_NOREPLACE)
	if err != unix.EINVAL && err != unix.ENOSYS && err != unix.EOPNOTSUPP {
		return err
	}
	var st unix.Stat_t
	if err = unix.Fstatat(srcDir, src, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		if err = unix.Linkat(srcDir, src, dstDir, dst, 0); err != nil {
			return err
		}
		return unix.Unlinkat(srcDir, src, 0)
	}
	if err = unix.Fstatat(dstDir, dst, &st, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return unix.EEXIST
	} else if err != unix.ENOENT {
		return err
	}
	return unix.Renameat(srcDir, src, dstDir, dst)
}
