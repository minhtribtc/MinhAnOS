package drive

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"

	"minhanos.local/platform/internal/storagecore"
)

// QuotaLedger is the application-level (soft) quota of personal homes (storagecore.Store in production).
// It is enforced while Files is the only write path; the host system root is outside quota.
type QuotaLedger interface {
	Reserve(ctx context.Context, home string, bytes int64) (storagecore.Reservation, error)
	Commit(ctx context.Context, r storagecore.Reservation, actual int64) error
	Release(ctx context.Context, r storagecore.Reservation) error
	NeedsMeasure(ctx context.Context, home string) (bool, error)
	SetMeasured(ctx context.Context, home string, bytes int64) error
	HomeUsage(ctx context.Context, home string) (storagecore.Usage, error)
}

// measureHome sums regular files (files and trash) without following links. It repairs the ledger; it
// is not the enforcement mechanism.
func measureHome(dir string) (int64, error) {
	var total int64
	e := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
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

// syncMeasure re-measures a home when the ledger asks for it. Callers hold the user's write lock.
func (s *Server) syncMeasure(ctx context.Context, home, dir string) error {
	need, e := s.Quota.NeedsMeasure(ctx, home)
	if e != nil || !need {
		return e
	}
	n, e := measureHome(dir)
	if e != nil {
		return e
	}
	return s.Quota.SetMeasured(ctx, home, n)
}

// quotaSize is how many bytes a personal write may add: the raw upload body, the copied file, or the new
// text. Other operations (mkdir, rename, move, trash, restore) do not add data.
func quotaSize(r *http.Request, e *Engine) int64 {
	switch path.Base(r.URL.Path) {
	case "upload":
		if r.ContentLength >= 0 {
			return min(r.ContentLength, MaxUpload)
		}
		return MaxUpload
	case "copy":
		f, info, err := e.Read(r.URL.Query().Get("path"))
		if err != nil {
			return 0 // the operation itself reports the error
		}
		f.Close()
		return info.Size()
	case "save-text":
		return int64(MaxText)
	}
	return 0
}

// quotaActual is what the finished write added. Upload bodies are raw file bytes; text saves keep the old
// version as a backup, so the new file size is added.
func quotaActual(r *http.Request, e *Engine, reserved int64) int64 {
	if path.Base(r.URL.Path) == "save-text" {
		f, info, err := e.Read(r.URL.Query().Get("path"))
		if err != nil {
			return reserved
		}
		f.Close()
		return info.Size()
	}
	return reserved
}

func quotaFailure(w http.ResponseWriter, e error) {
	var q storagecore.QuotaExceeded
	if errors.As(e, &q) {
		fail(w, 507, "Đã hết dung lượng được cấp.", "QUOTA_EXCEEDED")
		return
	}
	fail(w, 503, "Không kiểm tra được dung lượng.", "SERVICE_UNAVAILABLE")
}
