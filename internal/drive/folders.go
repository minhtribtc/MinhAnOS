package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"minhanos.local/platform/internal/storagecore"
)

// errProtectedFolder: a quota folder (or a folder containing one) cannot go to the trash; the admin removes
// the quota folder in Storage Manager first. Rename and move keep the registration (paths are rewritten).
var errProtectedFolder = errors.New("quota folder protected")

// folderPlan is how one Shared Folder write touches quota folders (storage Q3). Paths are logical ("." root).
type folderPlan struct {
	write      []string // folders a new file lands in (upload, copy, save-text): reserved with the share
	from, to   []string // move: folders the item leaves / enters (moves inside a share keep its total)
	size       int64    // bytes moved, trashed or restored
	rewrite    [2]string
	restore    bool
	trash      bool
	moveHold   *storagecore.Reservation
	folderList []storagecore.FolderScope
}

// within lists the quota folders strictly enclosing p (p itself excluded: a folder does not hold itself).
func within(folders []storagecore.FolderScope, p string) []string {
	result := []string{}
	for _, f := range folders {
		if strings.HasPrefix(p, f.Path+"/") {
			result = append(result, f.ID)
		}
	}
	return result
}

// holds reports whether p is a quota folder or contains one.
func holds(folders []storagecore.FolderScope, p string) bool {
	for _, f := range folders {
		if p == "." || f.Path == p || strings.HasPrefix(f.Path, p+"/") {
			return true
		}
	}
	return false
}

func minus(a, b []string) []string {
	result := []string{}
	for _, x := range a {
		found := false
		for _, y := range b {
			found = found || x == y
		}
		if !found {
			result = append(result, x)
		}
	}
	return result
}

// treeSize sums regular files at rel (a file or a folder) inside the unit, without following links.
func treeSize(root *os.Root, rel string) (int64, error) {
	var total int64
	e := fs.WalkDir(root.FS(), rel, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, e
}

// planFolders reads the operation (and its JSON body, which is put back for ServeEngine) and works out the
// quota folders it touches. Only called for mutations on a Shared Folder that has quota folders.
func (s *Server) planFolders(r *http.Request, e *Engine, folders []storagecore.FolderScope) (*folderPlan, error) {
	plan := &folderPlan{folderList: folders}
	op := path.Base(r.URL.Path)
	p, err := clean(r.URL.Query().Get("path"))
	if err != nil {
		return nil, err
	}
	var body struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		ID          string `json:"id"`
	}
	if op != "upload" {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, problem(400, "INVALID_REQUEST", "Dữ liệu yêu cầu không hợp lệ.")
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		_ = json.Unmarshal(raw, &body) // ServeEngine validates strictly
	}
	switch op {
	case "upload":
		plan.write = within(folders, path.Join(p, r.URL.Query().Get("name")))
	case "save-text":
		plan.write = within(folders, p)
	case "copy":
		if d, err := clean(body.Destination); err == nil {
			plan.write = within(folders, d)
		}
	case "rename", "move":
		d := path.Join(path.Dir(p), body.Name)
		if op == "move" {
			if d, err = clean(body.Destination); err != nil {
				return plan, nil // ServeEngine reports it
			}
		}
		if holds(folders, p) && p != "." {
			plan.rewrite = [2]string{p, d}
		}
		if op == "move" {
			plan.from, plan.to = minus(within(folders, p), within(folders, d)), minus(within(folders, d), within(folders, p))
			if len(plan.from)+len(plan.to) > 0 {
				if plan.size, err = treeSize(e.Root, e.disk(p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return nil, err
				}
			}
		}
	case "trash":
		if holds(folders, p) {
			return nil, errProtectedFolder
		}
		if plan.from = within(folders, p); len(plan.from) > 0 {
			plan.trash = true
			if plan.size, err = treeSize(e.Root, e.disk(p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	case "restore":
		if !validID(body.ID) {
			return plan, nil
		}
		items, err := e.TrashList()
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.ID == body.ID {
				if plan.to = within(folders, item.Path); len(plan.to) > 0 {
					plan.restore = true
					if plan.size, err = treeSize(e.Root, path.Join("trash", item.ID, "content")); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return plan, nil
}

// settleFolders applies a finished operation to the quota folders: rewritten paths, usage that left or
// entered folders. Uses a fresh context like the share ledger: the client may be gone.
func (s *Server) settleFolders(shareID string, plan *folderPlan, ok bool) {
	if plan == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var err error
	if plan.moveHold != nil {
		if ok {
			err = s.Quota.Commit(ctx, *plan.moveHold, plan.size)
		} else {
			err = s.Quota.Release(ctx, *plan.moveHold)
		}
	}
	if ok && err == nil && plan.rewrite[0] != "" {
		err = s.Shares.MoveFolders(ctx, shareID, plan.rewrite[0], plan.rewrite[1])
	}
	if ok && err == nil && (plan.trash || len(plan.from) > 0) {
		err = s.Shares.AdjustFolders(ctx, plan.from, -plan.size)
	}
	if ok && err == nil && plan.restore {
		err = s.Shares.AdjustFolders(ctx, plan.to, plan.size)
	}
	if err != nil {
		logError(context.Background(), "files.quota_folder.settle_failed", err, "share", shareID)
	}
}

// measureShares creates/adopts and measures pending quota folders, then re-measures shares an admin asked
// for. It holds the share's read lock (writes hold the write lock until they settle), like homes.
func (s *Server) measureShares(ctx context.Context) error {
	if s.Shares == nil {
		return nil
	}
	folders, e := s.Shares.PendingFolderMeasures(ctx, 10)
	if e != nil {
		return e
	}
	for _, f := range folders {
		rel := path.Join("files", f.Path)
		n, e := s.withShare(ctx, f.ShareID, f.Volume, f.ShareKey, func(root *os.Root) (int64, error) {
			if e := root.MkdirAll(rel, 0750); e != nil {
				return 0, e
			}
			return treeSize(root, rel)
		})
		if e == nil {
			e = s.Shares.SetFolderMeasured(ctx, f.ID, n)
		}
		if e != nil {
			logError(ctx, "files.quota_folder.measure_failed", e, "folder", f.ID) // retried on the next pass
		}
	}
	shares, e := s.Shares.PendingShareMeasures(ctx, 5)
	if e != nil {
		return e
	}
	for _, share := range shares {
		n, e := s.withShare(ctx, share.ID, share.Volume, share.Key, func(root *os.Root) (int64, error) { return treeSize(root, ".") })
		if e == nil {
			e = s.Shares.SetShareMeasured(ctx, share.ID, n)
		}
		if e != nil {
			logError(ctx, "files.share.measure_failed", e, "share", share.Key)
		}
	}
	return nil
}

func (s *Server) withShare(ctx context.Context, id, volume, key string, measure func(*os.Root) (int64, error)) (int64, error) {
	v, _ := s.locks.LoadOrStore("share:"+id, &sync.RWMutex{})
	lock := v.(*sync.RWMutex)
	lock.RLock()
	defer lock.RUnlock()
	dir, e := s.unitPath(ctx, volume, "shares", key, false)
	if e != nil {
		return 0, e
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return 0, e
	}
	defer root.Close()
	return measure(root)
}
