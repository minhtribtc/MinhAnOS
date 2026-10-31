package drive

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"sync"
	"time"

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
	PendingMeasures(ctx context.Context, limit int) ([]string, error)
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

// measurePending measures homes the ledger flagged (first use, admin request). It runs in the background
// worker, never on a request path: a large home can take long to walk. The user's read lock excludes their
// writes (which hold the write lock until they commit), so the measurement and the ledger stay consistent.
func (s *Server) measurePending(ctx context.Context) error {
	homes, e := s.Quota.PendingMeasures(ctx, 10)
	if e != nil {
		return e
	}
	for _, home := range homes {
		if !validID(home) {
			continue
		}
		v, _ := s.locks.LoadOrStore(home, &sync.RWMutex{})
		lock := v.(*sync.RWMutex)
		lock.RLock()
		dir, e := s.homeDir(ctx, home, false)
		var n int64
		if e == nil {
			n, e = measureHome(dir)
		}
		if errors.Is(e, storagecore.ErrNoHomes) {
			lock.RUnlock()
			return nil // nothing to measure until a share hosts homes
		}
		if errors.Is(e, fs.ErrNotExist) {
			n, e = 0, nil
		}
		if e == nil {
			e = s.Quota.SetMeasured(ctx, home, n)
		}
		lock.RUnlock()
		if e != nil {
			return e
		}
	}
	return nil
}

// StartMeasurer starts the background measurer before the server accepts requests. It processes the queue
// every interval and when a request found an unmeasured home. Until a home is measured, writes are checked
// against its last known usage (soft quota).
func (s *Server) StartMeasurer(ctx context.Context, interval time.Duration) {
	if s.Quota == nil {
		return
	}
	s.measureKick = make(chan struct{}, 1)
	go s.runMeasurer(ctx, interval)
}

func (s *Server) runMeasurer(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if e := s.measurePending(ctx); e != nil && ctx.Err() == nil {
			logError(ctx, "files.quota.measure_failed", e)
		}
		if e := s.measureShares(ctx); e != nil && ctx.Err() == nil {
			logError(ctx, "files.share.measure_failed", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.measureKick:
		}
	}
}

func (s *Server) kickMeasurer() {
	select {
	case s.measureKick <- struct{}{}:
	default:
	}
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
	if errors.As(e, &q) && q.Scope == "folder" {
		fail(w, 507, "Thư mục quota đã hết dung lượng được cấp.", "QUOTA_EXCEEDED")
		return
	}
	if errors.As(e, &q) {
		fail(w, 507, "Đã hết dung lượng được cấp.", "QUOTA_EXCEEDED")
		return
	}
	fail(w, 503, "Không kiểm tra được dung lượng.", "SERVICE_UNAVAILABLE")
}
