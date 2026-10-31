package drive

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// File management beyond single items (Files roadmap phase 1): search and recent files over a whole subtree,
// several items as one streamed ZIP, and emptying the trash. Walks never follow links, skip special files and the
// host's virtual filesystems, and stop after walkLimit entries (the answer then says it is truncated).
const (
	walkLimit    = 100000
	SearchLimit  = 200
	RecentLimit  = 50
	ArchiveItems = 1000
)

// walk visits the subtree under the logical path p (p itself excluded) until fn returns errStopWalk or the
// entry budget is spent; it reports whether the walk was cut short.
func (e *Engine) walk(p string, fn func(logical string, d fs.DirEntry) error) (bool, error) {
	p, err := e.logical(p)
	if err != nil {
		return false, err
	}
	base := e.disk(p)
	if info, err := e.Root.Lstat(base); err != nil {
		return false, err
	} else if !info.IsDir() {
		return false, problem(400, "FILES_NOT_FOLDER", "Đường dẫn không phải thư mục.")
	}
	seen, truncated := 0, false
	err = fs.WalkDir(e.Root.FS(), base, func(disk string, d fs.DirEntry, err error) error {
		if err != nil {
			if disk == base {
				return err
			}
			return nil // unreadable subfolder: skip it, keep the rest
		}
		if disk == base {
			return nil
		}
		logical := path.Join(p, strings.TrimPrefix(strings.TrimPrefix(disk, base), "/"))
		if e.Host && d.IsDir() && !strings.Contains(logical, "/") {
			if _, err := e.logical(logical); err != nil {
				return fs.SkipDir
			}
		}
		if seen++; seen > walkLimit {
			truncated = true
			return fs.SkipAll
		}
		return fn(logical, d)
	})
	if errors.Is(err, errStopWalk) {
		return true, nil
	}
	return truncated, err
}

var errStopWalk = errors.New("stop walk")

// Search finds files and folders whose name contains q (case-insensitive) anywhere under p.
func (e *Engine) Search(p, q string) ([]Entry, bool, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" || len(q) > 255 {
		return nil, false, problem(400, "FILES_SEARCH_INVALID", "Từ khóa tìm kiếm không hợp lệ.")
	}
	out := []Entry{}
	truncated, err := e.walk(p, func(logical string, d fs.DirEntry) error {
		if !strings.Contains(strings.ToLower(d.Name()), q) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, entry(info, logical))
		if len(out) >= SearchLimit {
			return errStopWalk
		}
		return nil
	})
	return out, truncated, err
}

// Recent lists the most recently modified regular files under p, newest first.
func (e *Engine) Recent(p string) ([]Entry, bool, error) {
	out := []Entry{}
	truncated, err := e.walk(p, func(logical string, d fs.DirEntry) error {
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, entry(info, logical))
		// Keep memory bounded: trim to the newest RecentLimit now and then.
		if len(out) >= RecentLimit*20 {
			sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
			out = out[:RecentLimit]
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out[:min(len(out), RecentLimit)], truncated, err
}

// ArchiveCheck validates the items of an archive before the response starts (afterwards errors cannot be reported).
func (e *Engine) ArchiveCheck(items []string) ([]string, error) {
	if len(items) == 0 || len(items) > ArchiveItems {
		return nil, problem(400, "FILES_ARCHIVE_INVALID", "Danh sách tải xuống không hợp lệ.")
	}
	out, seen := make([]string, 0, len(items)), map[string]bool{}
	for _, item := range items {
		p, err := e.logical(item)
		if err != nil {
			return nil, err
		}
		if seen[p] {
			continue // a repeated item would be walked again
		}
		seen[p] = true
		if p == "." {
			return nil, problem(400, "FILES_ARCHIVE_INVALID", "Danh sách tải xuống không hợp lệ.")
		}
		if _, err = e.Root.Lstat(e.disk(p)); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Archive writes the checked items as one ZIP: folders recursively, regular files only (links and special files are
// skipped). Each item is stored under its own name; items from different folders with the same name get " (2)",
// " (3)"… so none is lost.
func (e *Engine) Archive(w io.Writer, items []string) error {
	z := zip.NewWriter(w)
	tops := map[string]bool{}
	for _, p := range items {
		base, top := path.Base(p), path.Base(p)
		for n := 2; tops[top]; n++ {
			ext := path.Ext(base)
			top = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(base, ext), n, ext)
		}
		tops[top] = true
		add := func(logical string, d fs.DirEntry) error {
			name := top + strings.TrimPrefix(logical, p)
			info, err := d.Info()
			if err != nil {
				return nil
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = name
			if d.IsDir() {
				header.Name += "/"
				_, err = z.CreateHeader(header)
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			header.Method = zip.Deflate
			out, err := z.CreateHeader(header)
			if err != nil {
				return err
			}
			f, err := e.Root.Open(e.disk(logical))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(out, f)
			return err
		}
		info, err := e.Root.Lstat(e.disk(p))
		if err != nil {
			return err
		}
		if err = add(p, fs.FileInfoToDirEntry(info)); err != nil {
			return err
		}
		if info.IsDir() {
			if _, err = e.walk(p, add); err != nil {
				return err
			}
		}
	}
	return z.Close()
}

// EmptyTrash permanently removes every trash item (id empty) or one item, and returns the bytes freed.
func (e *Engine) EmptyTrash(id string) (int64, error) {
	if e.Host {
		return 0, problem(400, "FILES_NO_SYSTEM_TRASH", "Không có thùng rác hệ thống.")
	}
	ids := []string{id}
	if id == "" {
		items, err := e.TrashList()
		if err != nil {
			return 0, err
		}
		ids = ids[:0]
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	} else if !validID(id) {
		return 0, problem(400, "FILES_TRASH_ITEM_INVALID", "Mục trong thùng rác không hợp lệ.")
	}
	var freed int64
	for _, id := range ids {
		dir := path.Join("trash", id)
		if _, err := e.Root.Lstat(dir); err != nil {
			return freed, err
		}
		_ = fs.WalkDir(e.Root.FS(), dir, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				if info, err := d.Info(); err == nil {
					freed += info.Size()
				}
			}
			return nil
		})
		if err := e.Root.RemoveAll(dir); err != nil {
			return freed, err
		}
	}
	return freed, nil
}
