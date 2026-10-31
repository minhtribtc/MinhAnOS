// Package jobcore is the platform job engine and scheduler (ADR 0010, contracts/jobs/jobs.v1.yaml): one durable
// table for the background work of every app. An app registers a handler for its job types on a Worker and queues
// work with Enqueue; the worker claims with a lease and fencing token, retries with backoff when allowed, honours
// cooperative cancel and records progress. Schedules enqueue jobs at fixed intervals, once across all instances.
package jobcore

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
	"minhanos.local/platform/internal/requestid"
)

//go:embed migrations/001_jobs.sql
var jobsSQL string

func Module() migrate.Module {
	return migrate.Module{ID: "jobs", Steps: []migrate.Step{{ID: 1, Name: "jobs", SQL: jobsSQL}}}
}

func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}

const (
	RetentionDays = 7
	DefaultLease  = 60 * time.Second
	maxBackoff    = 10 * time.Minute
	listLimit     = 100
)

var (
	ErrNotFound  = errors.New("job not found")
	ErrLeaseLost = errors.New("job lease lost")
	ErrCancelled = errors.New("job cancelled")
	ErrInvalid   = errors.New("invalid job")
	typePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	uuidPattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Coded is an error with a stable code: the job fails with that code and is not retried.
type Coded interface{ JobCode() string }

// Job is the contracts/jobs/job.v1 shape.
type Job struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	State       string    `json:"state"`
	Progress    Progress  `json:"progress"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"maxAttempts"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Error       *Error    `json:"error"`
}
type Progress struct {
	Completed int64  `json:"completed"`
	Total     *int64 `json:"total"`
	Unit      string `json:"unit"`
}
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

// Spec describes a job to queue. Actor (an account ID) may see and cancel it; empty means a system job.
// DedupeKey makes Enqueue return the unfinished job of the same type and key instead of queuing another.
type Spec struct {
	Type, Actor, Organization, DedupeKey, Unit string
	Payload                                    any
	MaxAttempts                                int
	Total                                      *int64
	RunAfter                                   time.Time
}

// Querier is a pool or a transaction: Enqueue inside the caller's transaction queues the job iff it commits.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const columns = "id::text,type,state,completed,total,unit,attempt,max_attempts,created_at,updated_at,error_code,request_id"

func scan(row pgx.Row, extra ...any) (Job, error) {
	var j Job
	var code *string
	var request string
	if err := row.Scan(append([]any{&j.ID, &j.Type, &j.State, &j.Progress.Completed, &j.Progress.Total, &j.Progress.Unit, &j.Attempt, &j.MaxAttempts, &j.CreatedAt, &j.UpdatedAt, &code, &request}, extra...)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	if code != nil {
		j.Error = &Error{Code: *code, Message: *code, RequestID: request, Details: map[string]any{}}
	}
	return j, nil
}

// Enqueue queues a job (see Spec).
func Enqueue(ctx context.Context, q Querier, s Spec) (Job, error) {
	if !typePattern.MatchString(s.Type) || len(s.DedupeKey) > 200 {
		return Job{}, ErrInvalid
	}
	if s.MaxAttempts == 0 {
		s.MaxAttempts = 3
	}
	if s.Unit == "" {
		s.Unit = "steps"
	}
	if s.Payload == nil {
		s.Payload = map[string]any{}
	}
	payload, err := json.Marshal(s.Payload)
	if err != nil {
		return Job{}, err
	}
	if s.RunAfter.IsZero() {
		s.RunAfter = time.Now()
	}
	ids := requestid.From(ctx)
	for range 3 {
		j, err := scan(q.QueryRow(ctx, `INSERT INTO platform_jobs(type,actor_id,organization_id,payload,dedupe_key,unit,total,max_attempts,run_after,request_id,correlation_id)
			VALUES($1,nullif($2,''),nullif($3,'')::uuid,$4,nullif($5,''),$6,$7,$8,$9,$10,$11)
			ON CONFLICT(type,dedupe_key) WHERE dedupe_key IS NOT NULL AND state IN ('queued','running') DO NOTHING RETURNING `+columns,
			s.Type, s.Actor, s.Organization, payload, s.DedupeKey, s.Unit, s.Total, s.MaxAttempts, s.RunAfter, ids.Request, ids.Correlation))
		if !errors.Is(err, ErrNotFound) {
			return j, err
		}
		// The same work is already queued: hand that job back (unless it just finished; then queue again).
		j, err = scan(q.QueryRow(ctx, "SELECT "+columns+" FROM platform_jobs WHERE type=$1 AND dedupe_key=$2 AND state IN ('queued','running')", s.Type, s.DedupeKey))
		if !errors.Is(err, ErrNotFound) {
			return j, err
		}
	}
	return Job{}, fmt.Errorf("enqueue %s: dedupe contention", s.Type)
}

// Store reads and cancels jobs on behalf of their actor.
type Store struct{ DB *pgxpool.Pool }

func (s Store) Get(ctx context.Context, actor, id string) (Job, error) {
	if actor == "" || !uuidPattern.MatchString(id) {
		return Job{}, ErrNotFound
	}
	return scan(s.DB.QueryRow(ctx, "SELECT "+columns+" FROM platform_jobs WHERE id=$1 AND actor_id=$2", id, actor))
}

// List returns the actor's most recent jobs of the given types (all types when none are given).
func (s Store) List(ctx context.Context, actor string, types []string, limit int) ([]Job, error) {
	if limit < 1 || limit > listLimit {
		limit = listLimit
	}
	rows, err := s.DB.Query(ctx, "SELECT "+columns+" FROM platform_jobs WHERE actor_id=$1 AND (cardinality($2::text[])=0 OR type=ANY($2)) ORDER BY created_at DESC LIMIT $3", actor, types, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		j, err := scan(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// Cancel is cooperative and idempotent: a queued job is cancelled at once, a running one when its handler next
// reports progress; a finished job is returned unchanged.
func (s Store) Cancel(ctx context.Context, actor, id string) (Job, error) {
	if actor == "" || !uuidPattern.MatchString(id) {
		return Job{}, ErrNotFound
	}
	return scan(s.DB.QueryRow(ctx, cancelSQL+" WHERE id=$1 AND actor_id=$2 RETURNING "+columns, id, actor))
}

// CancelActor requests cancel of every unfinished job of an actor (account disabled or deleted).
func CancelActor(ctx context.Context, q Querier, actor string) error {
	_, err := q.Exec(ctx, cancelSQL+" WHERE actor_id=$1 AND state IN ('queued','running')", actor)
	return err
}

const cancelSQL = `UPDATE platform_jobs SET cancel_requested=state IN ('queued','running') OR cancel_requested,
	state=CASE WHEN state='queued' THEN 'cancelled' ELSE state END,
	finished_at=CASE WHEN state='queued' THEN now() ELSE finished_at END,
	updated_at=CASE WHEN state IN ('queued','running') THEN now() ELSE updated_at END`

// Run is one claimed execution of a job, handed to its handler.
type Run struct {
	Job
	Actor, Organization string
	Payload             json.RawMessage
	Fencing             int64
	cancelRequested     bool
	db                  *pgxpool.Pool
	lease               time.Duration
}

// Decode reads the payload into v.
func (r *Run) Decode(v any) error { return json.Unmarshal(r.Payload, v) }

// Report records progress, extends the lease and tells whether cancel was requested; it fails with ErrLeaseLost
// when another worker took the job over (the caller must stop without side effects).
func (r *Run) Report(ctx context.Context, completed int64, total *int64) (cancel bool, err error) {
	err = r.db.QueryRow(ctx, "UPDATE platform_jobs SET completed=$3,total=coalesce($4,total),lease_until=now()+make_interval(secs=>$5),updated_at=now() WHERE id=$1 AND fencing=$2 AND state='running' RETURNING cancel_requested",
		r.ID, r.Fencing, completed, total, r.lease.Seconds()).Scan(&cancel)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrLeaseLost
	}
	return cancel, err
}

// Progress is Report for handlers that stop on cancel: it returns ErrCancelled when cancel was requested.
func (r *Run) Progress(ctx context.Context, completed int64, total *int64) error {
	cancel, err := r.Report(ctx, completed, total)
	if err == nil && cancel {
		return ErrCancelled
	}
	return err
}

// Handler runs one job. nil → succeeded; ErrCancelled → cancelled; a Coded error → failed with its code; any other
// error → retried with backoff when the handler allows it, else failed JOB_FAILED.
type Handler func(ctx context.Context, run *Run) error

// Options of a handler. Retry is only for work that is safe to repeat (staged output, idempotent effects).
type Options struct{ Retry bool }

type registration struct {
	handler Handler
	Options
}

type schedule struct {
	key, jobType string
	every        time.Duration
}

// Worker runs the handlers registered on it and the schedules of their types. Several services (and instances)
// share the table; each claims only its own types.
type Worker struct {
	DB        *pgxpool.Pool
	Lease     time.Duration
	handlers  map[string]registration
	schedules []schedule
	kick      chan struct{}
}

func NewWorker(db *pgxpool.Pool) *Worker {
	w := &Worker{DB: db, Lease: DefaultLease, handlers: map[string]registration{}, kick: make(chan struct{}, 1)}
	w.Handle("jobs.retention", w.retain, Options{})
	return w
}

// Handle registers the handler of a job type; call before Start.
func (w *Worker) Handle(jobType string, h Handler, o Options) {
	if !typePattern.MatchString(jobType) {
		panic("jobcore: invalid job type " + jobType)
	}
	w.handlers[jobType] = registration{h, o}
}

// Every queues a job of jobType every interval (at least a minute), once across instances; call before Start.
func (w *Worker) Every(key, jobType string, every time.Duration) {
	if !typePattern.MatchString(key) || every < time.Minute {
		panic("jobcore: invalid schedule " + key)
	}
	w.schedules = append(w.schedules, schedule{key, jobType, every})
}

func (w *Worker) types() []string {
	out := make([]string, 0, len(w.handlers))
	for t := range w.handlers {
		out = append(out, t)
	}
	return out
}

// Start registers the schedules and runs the worker until ctx ends; it polls every interval and wakes on Kick.
func (w *Worker) Start(ctx context.Context, interval time.Duration) error {
	if err := w.register(ctx); err != nil {
		return err
	}
	go w.loop(ctx, interval)
	return nil
}

// register records the schedules of this worker (keeping their next run).
func (w *Worker) register(ctx context.Context) error {
	if _, ok := w.handlers["jobs.retention"]; ok {
		w.schedules = append(w.schedules, schedule{"jobs.retention", "jobs.retention", 6 * time.Hour})
	}
	for _, s := range w.schedules {
		if _, ok := w.handlers[s.jobType]; !ok {
			return fmt.Errorf("jobcore: schedule %s has no handler for %s", s.key, s.jobType)
		}
		if _, err := w.DB.Exec(ctx, "INSERT INTO platform_schedules(key,job_type,every_seconds) VALUES($1,$2,$3) ON CONFLICT(key) DO UPDATE SET job_type=EXCLUDED.job_type,every_seconds=EXCLUDED.every_seconds",
			s.key, s.jobType, int(s.every.Seconds())); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) loop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.due(ctx); err != nil && ctx.Err() == nil {
			slog.Error("jobs.schedule_failed", "error", err)
		}
		for w.RunOne(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-w.kick:
		}
	}
}

// Kick wakes the worker after a job was queued.
func (w *Worker) Kick() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// due queues the jobs of due schedules and advances them in one transaction (FOR UPDATE SKIP LOCKED: one instance
// wins); the dedupe key stops a slow run from piling up behind itself.
func (w *Worker) due(ctx context.Context) error {
	tx, err := w.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT key,job_type,every_seconds FROM platform_schedules WHERE job_type=ANY($1) AND next_run_at<=now() FOR UPDATE SKIP LOCKED", w.types())
	if err != nil {
		return err
	}
	var due []schedule
	for rows.Next() {
		var s schedule
		var seconds int
		if err = rows.Scan(&s.key, &s.jobType, &seconds); err != nil {
			rows.Close()
			return err
		}
		s.every = time.Duration(seconds) * time.Second
		due = append(due, s)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, s := range due {
		job, err := Enqueue(ctx, tx, Spec{Type: s.jobType, DedupeKey: "schedule:" + s.key, MaxAttempts: 1})
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE platform_schedules SET next_run_at=now()+make_interval(secs=>$2),last_job_id=$3::uuid WHERE key=$1", s.key, s.every.Seconds(), job.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// RunOne claims and runs one job of the registered types; it reports whether a job was found.
func (w *Worker) RunOne(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	run := &Run{db: w.DB, lease: w.Lease}
	var actor, org *string
	j, err := scan(w.DB.QueryRow(ctx, `UPDATE platform_jobs SET state='running',attempt=attempt+1,fencing=fencing+1,lease_until=now()+make_interval(secs=>$2),updated_at=now()
		WHERE id=(SELECT id FROM platform_jobs WHERE type=ANY($1) AND ((state='queued' AND run_after<=now()) OR (state='running' AND lease_until<now())) ORDER BY run_after LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING `+columns+",actor_id,organization_id::text,payload,fencing,cancel_requested", w.types(), w.Lease.Seconds()), &actor, &org, &run.Payload, &run.Fencing, &run.cancelRequested)
	if errors.Is(err, ErrNotFound) {
		return false
	}
	if err != nil {
		slog.Error("jobs.claim_failed", "error", err)
		return false
	}
	run.Job = j
	if actor != nil {
		run.Actor = *actor
	}
	if org != nil {
		run.Organization = *org
	}
	reg := w.handlers[j.Type]
	ctx = requestid.With(ctx, requestid.IDs{Request: "job-" + j.ID, Correlation: "job-" + j.ID})
	state, code, retry := "succeeded", "", false
	switch {
	case run.cancelRequested:
		state = "cancelled"
	case j.Attempt > j.MaxAttempts:
		state, code = "failed", "JOB_INTERRUPTED"
	default:
		err = safeRun(ctx, reg.handler, run)
		var coded Coded
		switch {
		case err == nil:
		case errors.Is(err, ErrLeaseLost):
			return true // another worker owns the job now
		case errors.Is(err, ErrCancelled):
			state = "cancelled"
		case errors.As(err, &coded):
			state, code = "failed", coded.JobCode()
		default:
			slog.Error("jobs.run_failed", "type", j.Type, "job", j.ID, "error", err)
			state, code, retry = "failed", "JOB_FAILED", reg.Retry
		}
	}
	if err = w.finish(ctx, run, state, code, retry); err != nil {
		slog.Error("jobs.finish_failed", "type", j.Type, "job", j.ID, "error", err)
	}
	return true
}

func safeRun(ctx context.Context, h Handler, run *Run) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("handler panic: %v", p)
		}
	}()
	return h(ctx, run)
}

// finish ends a run: a final state, or back to queued after an exponential backoff with jitter when retry is set
// and attempts remain.
func (w *Worker) finish(ctx context.Context, run *Run, state, code string, retry bool) error {
	var errorCode *string
	if code != "" {
		errorCode = &code
	}
	backoff := min(5*time.Second<<min(run.Attempt, 10), maxBackoff)
	backoff = backoff/2 + rand.N(backoff/2+1)
	_, err := w.DB.Exec(ctx, `UPDATE platform_jobs SET
		state=CASE WHEN $5 AND attempt<max_attempts AND NOT cancel_requested THEN 'queued' ELSE $3 END,
		run_after=CASE WHEN $5 AND attempt<max_attempts AND NOT cancel_requested THEN now()+make_interval(secs=>$6) ELSE run_after END,
		finished_at=CASE WHEN $5 AND attempt<max_attempts AND NOT cancel_requested THEN NULL ELSE now() END,
		error_code=$4,lease_until=NULL,updated_at=now()
		WHERE id=$1 AND fencing=$2 AND state='running'`, run.ID, run.Fencing, state, errorCode, retry, backoff.Seconds())
	return err
}

// retain removes finished jobs older than RetentionDays in bounded batches.
func (w *Worker) retain(ctx context.Context, _ *Run) error {
	for {
		tag, err := w.DB.Exec(ctx, "DELETE FROM platform_jobs WHERE id IN (SELECT id FROM platform_jobs WHERE finished_at<now()-make_interval(days=>$1) LIMIT 500)", RetentionDays)
		if err != nil || tag.RowsAffected() < 500 {
			return err
		}
	}
}
