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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/eventcore"
	"minhanos.local/platform/internal/jobcore"
	"minhanos.local/platform/internal/storagecore"
)

// Durable Files jobs on the platform job engine (ADR 0010, contracts/jobs). Copying a folder, or a file larger
// than a request may carry, is queued as a files.copy job; the worker claims it with a lease and fencing token,
// copies into a staging folder of the unit (jobs/<id>), reports progress, honours cancel and renames the staging
// folder into place at the end. A lost lease lets another run reclaim the job within maxAttempts; the fencing
// token stops the old run, and staging makes a retry start clean.
const (
	jobProgressEvery = 4 << 20
	jobListLimit     = 20
)

var errJobNotFound = problem(404, "FILES_JOB_NOT_FOUND", "Không tìm thấy tác vụ.")

type (
	Job         = jobcore.Job
	JobProgress = jobcore.Progress
	JobError    = jobcore.Error
)

// copyPayload is the files.copy job payload: paths in the actor's home or a Shared Folder they may write.
type copyPayload struct {
	Root        string `json:"root"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// JobCode makes file errors final job failures with their stable code.
func (e *FileError) JobCode() string { return e.Code }

// RegisterJobs puts the Files handlers and schedules on the worker; Server.Jobs must be that worker.
func (s *Server) RegisterJobs(w *jobcore.Worker, db *pgxpool.Pool) {
	w.Handle("files.copy", s.copyTree, jobcore.Options{Retry: true})
	w.Handle("files.audit-retention", func(ctx context.Context, _ *jobcore.Run) error { return RetainAudit(ctx, db) }, jobcore.Options{})
	w.Handle("files.user-removal", func(ctx context.Context, _ *jobcore.Run) error { return s.RemoveDeletedUsers(ctx, db) }, jobcore.Options{})
	w.Every("files.audit-retention", "files.audit-retention", 6*time.Hour)
	w.Every("files.user-removal", "files.user-removal", time.Hour)
}

// RegisterEvents subscribes Files to platform events: a disabled account's unfinished jobs are cancelled.
func (s *Server) RegisterEvents(b *eventcore.Bus) {
	b.Subscribe("files.jobs", []string{"identity.user.disabled.v1"}, func(ctx context.Context, tx pgx.Tx, e eventcore.Event) error {
		var actor string
		err := tx.QueryRow(ctx, "SELECT id FROM users WHERE public_id::text=$1", e.Subject).Scan(&actor)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return jobcore.CancelActor(ctx, tx, actor)
	})
}

func (s *Server) copyJob(r *http.Request, e *Engine, u User, root string) (Job, bool, error) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return Job{}, false, problem(400, "INVALID_REQUEST", "Dữ liệu yêu cầu không hợp lệ.")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	source, err := e.logical(r.URL.Query().Get("path"))
	if err != nil {
		return Job{}, false, err
	}
	info, err := e.Root.Lstat(e.disk(source))
	if err != nil {
		return Job{}, false, err
	}
	if !info.IsDir() && info.Size() <= MaxUpload {
		return Job{}, false, nil
	}
	if s.Jobs == nil {
		return Job{}, false, problem(501, "FILES_JOBS_UNAVAILABLE", "Tác vụ nền chưa bật.")
	}
	var body struct {
		Destination string `json:"destination"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil {
		return Job{}, false, problem(400, "INVALID_REQUEST", "Dữ liệu yêu cầu không hợp lệ.")
	}
	dest, err := e.logical(body.Destination)
	if err != nil || dest == "." || source == "." || dest == source || strings.HasPrefix(dest, source+"/") {
		return Job{}, false, problem(400, "FILES_DESTINATION_INVALID", "Đích sao chép không hợp lệ.")
	}
	if _, err = e.Root.Lstat(e.disk(dest)); err == nil {
		return Job{}, false, problem(409, "FILES_EXISTS", "Đích đã tồn tại.")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Job{}, false, err
	}
	if _, err = e.Root.Lstat(e.disk(path.Dir(dest))); err != nil {
		return Job{}, false, err
	}
	job, err := jobcore.Enqueue(r.Context(), s.Jobs.DB, jobcore.Spec{Type: "files.copy", Actor: u.ID, Unit: "bytes", Payload: copyPayload{root, source, dest}})
	if err == nil {
		s.Jobs.Kick()
	}
	return job, true, err
}

// serveJobs answers /api/jobs (the user's recent Files jobs), /api/jobs/{id} and POST /api/jobs/{id}/cancel.
func (s *Server) serveJobs(w http.ResponseWriter, r *http.Request, u User) {
	if s.Jobs == nil {
		fail(w, 501, "Tác vụ nền chưa bật.", "FILES_JOBS_UNAVAILABLE")
		return
	}
	store := jobcore.Store{DB: s.Jobs.DB}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/jobs"), "/")
	parts := strings.Split(rest, "/")
	var out any
	var err error
	switch {
	case rest == "" && r.Method == "GET":
		var jobs []Job
		jobs, err = store.List(r.Context(), u.ID, []string{"files.copy"}, jobListLimit)
		out = map[string]any{"items": jobs}
	case len(parts) == 1 && r.Method == "GET":
		out, err = store.Get(r.Context(), u.ID, parts[0])
	case len(parts) == 2 && parts[1] == "cancel" && r.Method == "POST":
		out, err = store.Cancel(r.Context(), u.ID, parts[0])
	default:
		fail(w, 404, "API không tồn tại.", "NOT_FOUND")
		return
	}
	if errors.Is(err, jobcore.ErrNotFound) {
		err = errJobNotFound
	}
	var fe *FileError
	switch {
	case errors.As(err, &fe):
		fail(w, fe.Status, fe.Message, fe.Code)
	case err != nil:
		logError(r.Context(), "files.jobs.lookup_failed", err)
		fail(w, 503, "Không đọc được tác vụ.", "SERVICE_UNAVAILABLE")
	default:
		jsonResponse(w, 200, out)
	}
}

// copyTree runs one copy job with a quota reservation for its size; see the staging note below.
func (s *Server) copyTree(ctx context.Context, run *jobcore.Run) error {
	var job copyPayload
	if err := run.Decode(&job); err != nil {
		return problem(422, "JOB_FAILED", "Tác vụ không hợp lệ.")
	}
	user := run.Actor
	var dir, lockKey, shareID string
	var folders []storagecore.FolderScope
	var err error
	if key, isShare := strings.CutPrefix(job.Root, "share:"); isShare {
		share, ok, e := s.userShare(ctx, user, key)
		if e != nil {
			return e
		}
		if !ok {
			return problem(404, "SHARE_NOT_FOUND", "Không tìm thấy thư mục chia sẻ.")
		}
		if share.Access != "write" {
			return problem(403, "SHARE_READ_ONLY", "Thư mục chia sẻ này chỉ cho phép xem.")
		}
		if dir, err = s.unitPath(ctx, share.Volume, "shares", share.Key, false); err != nil {
			return err
		}
		lockKey, shareID = "share:"+share.ID, share.ID
		if s.Shares != nil {
			if folders, err = s.Shares.ShareFolders(ctx, share.ID); err != nil {
				return err
			}
		}
	} else {
		if dir, err = s.homeDir(ctx, user, true); err != nil {
			return err
		}
		lockKey = user
	}
	rootFS, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer rootFS.Close()
	e := &Engine{Root: rootFS}
	src, dst := e.disk(job.Source), e.disk(job.Destination)
	// Staging lives in the unit beside files/ and trash/ (same file system, never listed); a previous
	// attempt may have left a partial copy there.
	staging := "jobs/" + run.ID
	_ = rootFS.RemoveAll(staging)
	if err = rootFS.MkdirAll("jobs", 0700); err != nil {
		return err
	}
	if _, err = rootFS.Lstat(dst); err == nil {
		return problem(409, "FILES_EXISTS", "Đích đã tồn tại.")
	}
	total, err := treeSize(rootFS, src)
	if err != nil {
		return err
	}
	if _, err = run.Report(ctx, 0, &total); err != nil {
		return err
	}
	var reservation *storagecore.Reservation
	if s.Quota != nil {
		var res storagecore.Reservation
		if shareID != "" && s.Shares != nil {
			res, err = s.Shares.ReserveShare(ctx, shareID, within(folders, job.Destination), total)
		} else if shareID == "" {
			res, err = s.Quota.Reserve(ctx, user, total)
		}
		if err != nil {
			var exceeded storagecore.QuotaExceeded
			if errors.As(err, &exceeded) {
				return problem(507, "QUOTA_EXCEEDED", "Vượt quota.")
			}
			return err
		}
		if res.ID != "" {
			reservation = &res
		}
	}
	var copied, reported int64
	err = fs.WalkDir(rootFS.FS(), src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := path.Join(staging, strings.TrimPrefix(p, src))
		switch {
		case d.IsDir():
			return rootFS.Mkdir(target, 0750)
		case !d.Type().IsRegular():
			return problem(422, "FILES_COPY_UNSUPPORTED_ENTRY", "Thư mục có liên kết hoặc tệp đặc biệt; không sao chép được.")
		}
		in, err := rootFS.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := rootFS.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if err != nil {
			return err
		}
		buf := make([]byte, 1<<20)
		for {
			n, rerr := in.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					out.Close()
					return werr
				}
				copied += int64(n)
				if copied-reported >= jobProgressEvery {
					reported = copied
					cancel, perr := run.Report(ctx, copied, nil)
					if perr != nil {
						out.Close()
						return perr
					}
					if cancel {
						out.Close()
						return jobcore.ErrCancelled
					}
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				out.Close()
				return rerr
			}
		}
		if err = out.Sync(); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
	if err == nil {
		var cancel bool
		if cancel, err = run.Report(ctx, copied, nil); err == nil && cancel {
			err = jobcore.ErrCancelled
		}
	}
	if err == nil {
		// Only the final step takes the home or share write lock, so a long copy never blocks the user.
		v, _ := s.locks.LoadOrStore(lockKey, &sync.RWMutex{})
		lock := v.(*sync.RWMutex)
		lock.Lock()
		if _, err = rootFS.Lstat(dst); err == nil {
			err = problem(409, "FILES_EXISTS", "Đích đã tồn tại.")
		} else if errors.Is(err, fs.ErrNotExist) {
			err = rootFS.Rename(staging, dst)
		}
		lock.Unlock()
	}
	if err != nil {
		_ = rootFS.RemoveAll(staging)
	}
	if reservation != nil {
		settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		var qerr error
		if err == nil {
			qerr = s.Quota.Commit(settle, *reservation, copied)
		} else {
			qerr = s.Quota.Release(settle, *reservation)
		}
		if qerr != nil {
			logError(ctx, "files.jobs.quota_settle_failed", qerr, "job", run.ID)
		}
	}
	return err
}
