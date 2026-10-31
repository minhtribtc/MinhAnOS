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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"minhanos.local/platform/internal/requestid"
	"minhanos.local/platform/internal/storagecore"
)

// Durable Files jobs (app standard row 20, files SQL 003, contracts/jobs). Copying a folder, or a file larger
// than a request may carry, is queued as a job; a worker in Drive claims it with a lease and fencing token,
// copies into a staging folder of the unit (jobs/<id>), reports progress, honours cancel and renames the
// staging folder into place at the end. A lost lease (crash, restart) lets another run reclaim the job
// within maxAttempts; the fencing token stops the old run, and staging makes a retry start clean.
const (
	jobLease         = 60 * time.Second
	jobProgressEvery = 4 << 20
	JobRetentionDays = 7
	jobListLimit     = 20
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	errJobNotFound  = problem(404, "FILES_JOB_NOT_FOUND", "Không tìm thấy tác vụ.")
	errJobLeaseLost = errors.New("job lease lost")
	errJobCancelled = errors.New("job cancelled")
)

// Job is the contracts/jobs/job.v1 shape.
type Job struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	State       string      `json:"state"`
	Progress    JobProgress `json:"progress"`
	Attempt     int         `json:"attempt"`
	MaxAttempts int         `json:"maxAttempts"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Error       *JobError   `json:"error"`
}
type JobProgress struct {
	Completed int64  `json:"completed"`
	Total     *int64 `json:"total"`
	Unit      string `json:"unit"`
}
type JobError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

// claimedJob is what the worker needs to run a job.
type claimedJob struct {
	Job
	UserID, Root, Source, Destination string
	Fencing                           int64
}

// JobStore keeps Files jobs; SQLIdentity implements it. Without it, folder copies are refused.
type JobStore interface {
	CreateJob(ctx context.Context, user, root, source, destination string) (Job, error)
	UserJob(ctx context.Context, user, id string) (Job, error)
	UserJobs(ctx context.Context, user string) ([]Job, error)
	CancelJob(ctx context.Context, user, id string) (Job, error)
	ClaimJob(ctx context.Context) (claimedJob, bool, error)
	ReportJob(ctx context.Context, id string, fencing, completed int64, total *int64) (cancel bool, err error)
	FinishJob(ctx context.Context, id string, fencing int64, state, code string, retry bool) error
}

const jobColumns = "id::text,type,state,completed,total,attempt,max_attempts,created_at,updated_at,error_code,request_id"

func scanJob(row pgx.Row, extra ...any) (Job, error) {
	var j Job
	var code *string
	var request string
	dest := append([]any{&j.ID, &j.Type, &j.State, &j.Progress.Completed, &j.Progress.Total, &j.Attempt, &j.MaxAttempts, &j.CreatedAt, &j.UpdatedAt, &code, &request}, extra...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, errJobNotFound
		}
		return Job{}, err
	}
	j.Progress.Unit = "bytes"
	if code != nil {
		j.Error = &JobError{Code: *code, Message: *code, RequestID: request, Details: map[string]any{}}
	}
	return j, nil
}

func (s *SQLIdentity) CreateJob(ctx context.Context, user, root, source, destination string) (Job, error) {
	return scanJob(s.db.QueryRow(ctx, "INSERT INTO files_jobs(user_id,type,root,source,destination,request_id) VALUES($1,'files.copy',$2,$3,$4,$5) RETURNING "+jobColumns,
		user, root, source, destination, requestid.From(ctx).Request))
}

func (s *SQLIdentity) UserJob(ctx context.Context, user, id string) (Job, error) {
	if !uuidPattern.MatchString(id) {
		return Job{}, errJobNotFound
	}
	return scanJob(s.db.QueryRow(ctx, "SELECT "+jobColumns+" FROM files_jobs WHERE id=$1 AND user_id=$2", id, user))
}

func (s *SQLIdentity) UserJobs(ctx context.Context, user string) ([]Job, error) {
	rows, err := s.db.Query(ctx, "SELECT "+jobColumns+" FROM files_jobs WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2", user, jobListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// CancelJob is cooperative and idempotent: a queued job is cancelled at once, a running one when the worker
// next reports progress; a finished job is returned unchanged.
func (s *SQLIdentity) CancelJob(ctx context.Context, user, id string) (Job, error) {
	if !uuidPattern.MatchString(id) {
		return Job{}, errJobNotFound
	}
	return scanJob(s.db.QueryRow(ctx, `UPDATE files_jobs SET cancel_requested=state IN ('queued','running') OR cancel_requested,
		state=CASE WHEN state='queued' THEN 'cancelled' ELSE state END,
		finished_at=CASE WHEN state='queued' THEN now() ELSE finished_at END,
		updated_at=CASE WHEN state IN ('queued','running') THEN now() ELSE updated_at END
		WHERE id=$1 AND user_id=$2 RETURNING `+jobColumns, id, user))
}

// ClaimJob takes the oldest queued job, or a running one whose lease expired, and fences out older workers.
func (s *SQLIdentity) ClaimJob(ctx context.Context) (claimedJob, bool, error) {
	var c claimedJob
	j, err := scanJob(s.db.QueryRow(ctx, `UPDATE files_jobs SET state='running',attempt=attempt+1,fencing=fencing+1,lease_until=now()+make_interval(secs=>$1),updated_at=now()
		WHERE id=(SELECT id FROM files_jobs WHERE state='queued' OR (state='running' AND lease_until<now()) ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING `+jobColumns+",user_id,root,source,destination,fencing", jobLease.Seconds()), &c.UserID, &c.Root, &c.Source, &c.Destination, &c.Fencing)
	if errors.Is(err, errJobNotFound) {
		return claimedJob{}, false, nil
	}
	c.Job = j
	return c, err == nil, err
}

// ReportJob records progress and extends the lease; it fails with errJobLeaseLost when another worker took over.
func (s *SQLIdentity) ReportJob(ctx context.Context, id string, fencing, completed int64, total *int64) (bool, error) {
	var cancel bool
	err := s.db.QueryRow(ctx, "UPDATE files_jobs SET completed=$3,total=coalesce($4,total),lease_until=now()+make_interval(secs=>$5),updated_at=now() WHERE id=$1 AND fencing=$2 AND state='running' RETURNING cancel_requested",
		id, fencing, completed, total, jobLease.Seconds()).Scan(&cancel)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errJobLeaseLost
	}
	return cancel, err
}

// FinishJob ends a run: a final state, or back to queued when retry is set and attempts remain.
func (s *SQLIdentity) FinishJob(ctx context.Context, id string, fencing int64, state, code string, retry bool) error {
	var errorCode *string
	if code != "" {
		errorCode = &code
	}
	_, err := s.db.Exec(ctx, `UPDATE files_jobs SET
		state=CASE WHEN $5 AND attempt<max_attempts AND NOT cancel_requested THEN 'queued' ELSE $3 END,
		finished_at=CASE WHEN $5 AND attempt<max_attempts AND NOT cancel_requested THEN NULL ELSE now() END,
		error_code=$4,lease_until=NULL,updated_at=now()
		WHERE id=$1 AND fencing=$2 AND state='running'`, id, fencing, state, errorCode, retry)
	return err
}

// copyJob queues a copy that cannot run within a request: a folder, or a file over MaxUpload. ok is false
// for small files, which ServeEngine copies directly (the body is put back for it).
func (s *Server) copyJob(r *http.Request, e *Engine, u User, root string) (Job, bool, error) {
	store, ok := s.Identity.(JobStore)
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
	if !ok {
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
	job, err := store.CreateJob(r.Context(), u.ID, root, source, dest)
	if err == nil {
		s.kickJobs()
	}
	return job, true, err
}

// serveJobs answers /api/jobs (the user's recent jobs), /api/jobs/{id} and POST /api/jobs/{id}/cancel.
func (s *Server) serveJobs(w http.ResponseWriter, r *http.Request, u User) {
	store, ok := s.Identity.(JobStore)
	if !ok {
		fail(w, 501, "Tác vụ nền chưa bật.", "FILES_JOBS_UNAVAILABLE")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/jobs"), "/")
	parts := strings.Split(rest, "/")
	var out any
	var err error
	switch {
	case rest == "" && r.Method == "GET":
		var jobs []Job
		jobs, err = store.UserJobs(r.Context(), u.ID)
		out = map[string]any{"items": jobs}
	case len(parts) == 1 && r.Method == "GET":
		out, err = store.UserJob(r.Context(), u.ID, parts[0])
	case len(parts) == 2 && parts[1] == "cancel" && r.Method == "POST":
		out, err = store.CancelJob(r.Context(), u.ID, parts[0])
	default:
		fail(w, 404, "API không tồn tại.", "NOT_FOUND")
		return
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

// StartJobs runs the job worker until ctx ends; it polls every interval and wakes early when a job is queued.
func (s *Server) StartJobs(ctx context.Context, interval time.Duration) {
	s.jobKick = make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			for s.runJob(ctx) {
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-s.jobKick:
			}
		}
	}()
}

func (s *Server) kickJobs() {
	if s.jobKick == nil {
		return
	}
	select {
	case s.jobKick <- struct{}{}:
	default:
	}
}

// runJob claims and runs one job; it reports whether a job was found.
func (s *Server) runJob(ctx context.Context) bool {
	store, ok := s.Identity.(JobStore)
	if !ok || ctx.Err() != nil {
		return false
	}
	job, found, err := store.ClaimJob(ctx)
	if err != nil {
		logError(ctx, "files.jobs.claim_failed", err)
		return false
	}
	if !found {
		return false
	}
	ctx = requestid.With(ctx, requestid.IDs{Request: "job-" + job.ID, Correlation: "job-" + job.ID})
	state, code, retry := "succeeded", "", false
	if job.Attempt > job.MaxAttempts {
		state, code = "failed", "JOB_INTERRUPTED"
	} else if err = s.copyTree(ctx, store, job); err != nil {
		var fe *FileError
		switch {
		case errors.Is(err, errJobLeaseLost):
			return true // another worker owns the job now
		case errors.Is(err, errJobCancelled):
			state = "cancelled"
		case errors.As(err, &fe):
			state, code = "failed", fe.Code
		default:
			logError(ctx, "files.jobs.run_failed", err, "job", job.ID)
			state, code, retry = "failed", "JOB_FAILED", true
		}
	}
	if err = store.FinishJob(ctx, job.ID, job.Fencing, state, code, retry); err != nil {
		logError(ctx, "files.jobs.finish_failed", err, "job", job.ID)
	}
	return true
}

// copyTree runs one copy job with a quota reservation for its size; see the staging note below.
func (s *Server) copyTree(ctx context.Context, store JobStore, job claimedJob) error {
	var dir, lockKey, shareID string
	var folders []storagecore.FolderScope
	var err error
	if key, isShare := strings.CutPrefix(job.Root, "share:"); isShare {
		share, ok, e := s.userShare(ctx, job.UserID, key)
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
		if dir, err = s.homeDir(ctx, job.UserID, true); err != nil {
			return err
		}
		lockKey = job.UserID
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
	staging := "jobs/" + job.ID
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
	if _, err = store.ReportJob(ctx, job.ID, job.Fencing, 0, &total); err != nil {
		return err
	}
	var reservation *storagecore.Reservation
	if s.Quota != nil {
		var res storagecore.Reservation
		if shareID != "" && s.Shares != nil {
			res, err = s.Shares.ReserveShare(ctx, shareID, within(folders, job.Destination), total)
		} else if shareID == "" {
			res, err = s.Quota.Reserve(ctx, job.UserID, total)
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
					cancel, perr := store.ReportJob(ctx, job.ID, job.Fencing, copied, nil)
					if perr != nil {
						out.Close()
						return perr
					}
					if cancel {
						out.Close()
						return errJobCancelled
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
		if cancel, err = store.ReportJob(ctx, job.ID, job.Fencing, copied, nil); err == nil && cancel {
			err = errJobCancelled
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
			logError(ctx, "files.jobs.quota_settle_failed", qerr, "job", job.ID)
		}
	}
	return err
}
