package drive

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/storagecore"
)

// User data ownership (app standard row 17, contracts/data/files.data.json): GET /api/export streams the
// user's personal home as a tar archive (folders and regular files; links and special files are skipped),
// and RemoveDeletedUsers removes the data of accounts deleted in Account Center (files SQL 004).
const RemovalGraceDays = 30

func (s *Server) exportHome(w http.ResponseWriter, r *http.Request, u User) {
	if !validID(u.ID) {
		fail(w, 403, "Định danh người dùng không hợp lệ.", "ACCESS_DENIED")
		return
	}
	dir, err := s.homeDir(r.Context(), u.ID, false)
	if errors.Is(err, storagecore.ErrNoHomes) {
		fail(w, 409, "Chưa có thư mục chứa dữ liệu người dùng; quản trị viên cần tạo trong Storage Manager.", "STORAGE_NOT_READY")
		return
	}
	if err != nil {
		logError(r.Context(), "files.export.home_failed", err)
		fail(w, 503, "Không mở được vùng lưu trữ.", "SERVICE_UNAVAILABLE")
		return
	}
	if err = os.MkdirAll(filepath.Join(dir, "files"), 0750); err != nil {
		handleFileError(w, err)
		return
	}
	rootFS, err := os.OpenRoot(dir)
	if err != nil {
		handleFileError(w, err)
		return
	}
	defer rootFS.Close()
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", `attachment; filename="minhanos-files-`+u.Username+`.tar"`)
	w.WriteHeader(200)
	// The archive may outlast the server's write timeout; each entry extends the deadline instead.
	rc := http.NewResponseController(w)
	tw := tar.NewWriter(w)
	err = fs.WalkDir(rootFS.FS(), "files", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "files" {
			return err
		}
		_ = rc.SetWriteDeadline(time.Now().Add(2 * time.Minute))
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name, header.Uname, header.Gname, header.Uid, header.Gid = strings.TrimPrefix(p, "files/"), "", "", 0, 0
		if d.IsDir() {
			header.Name += "/"
		}
		if err = tw.WriteHeader(header); err != nil || d.IsDir() {
			return err
		}
		f, err := rootFS.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.CopyN(tw, f, header.Size)
		return err
	})
	if err == nil {
		err = tw.Close()
	}
	if err != nil {
		// Headers are sent: the client sees a truncated archive (tar has no end marker), never a partial success.
		logError(r.Context(), "files.export.failed", err, "actor", u.ID)
	}
}

// RemoveDeletedUsers drops the jobs and replay rows of deleted accounts at once and empties their home
// (files, trash, job staging) once the grace period has passed. The unit itself stays for Storage Manager.
func (s *Server) RemoveDeletedUsers(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, "INSERT INTO files_removals(user_id) SELECT id FROM users WHERE status='deleted' ON CONFLICT DO NOTHING"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "DELETE FROM files_idempotency WHERE user_id IN (SELECT user_id FROM files_removals)"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "DELETE FROM platform_jobs WHERE type LIKE 'files.%' AND actor_id IN (SELECT user_id FROM files_removals)"); err != nil {
		return err
	}
	rows, err := db.Query(ctx, "SELECT user_id FROM files_removals WHERE removed_at IS NULL AND seen_at<now()-make_interval(days=>$1) ORDER BY seen_at LIMIT 20", RemovalGraceDays)
	if err != nil {
		return err
	}
	users, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, user := range users {
		if err = s.removeHome(ctx, user); err != nil {
			return err
		}
		if _, err = db.Exec(ctx, "UPDATE files_removals SET removed_at=now() WHERE user_id=$1", user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) removeHome(ctx context.Context, user string) error {
	if !validID(user) {
		return nil
	}
	dir, err := s.homeDir(ctx, user, false)
	if errors.Is(err, storagecore.ErrNoHomes) || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	v, _ := s.locks.LoadOrStore(user, &sync.RWMutex{})
	lock := v.(*sync.RWMutex)
	lock.Lock()
	defer lock.Unlock()
	for _, part := range []string{"files", "trash", "jobs"} {
		if err = os.RemoveAll(filepath.Join(dir, part)); err != nil {
			return err
		}
	}
	return nil
}
