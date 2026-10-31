package drive

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
	"minhanos.local/platform/internal/requestid"
)

//go:embed migrations/001_files_events.sql
var filesEventsSQL string

//go:embed migrations/002_files_idempotency.sql
var filesIdempotencySQL string

// Module is the Files schema owned by this package (migration runner, app standard row 18).
func Module() migrate.Module {
	return migrate.Module{ID: "files", Steps: []migrate.Step{{ID: 1, Name: "files_events", SQL: filesEventsSQL}, {ID: 2, Name: "files_idempotency", SQL: filesIdempotencySQL}}}
}

func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}

// AuditRetentionDays bounds files_events; Log Center reports it.
const AuditRetentionDays = 365

// AuditEvent is one Files write. Path is recorded for the host system root only; personal homes and
// Shared Folders never put paths or file names in the audit (privacy, row 17).
type AuditEvent struct {
	Action string // files.<operation>
	Root   string // personal, share, system
	Share  string // share key when Root is share
	Path   string // system root only
}

func (s *SQLIdentity) Audit(ctx context.Context, u User, ev AuditEvent) error {
	ids := requestid.From(ctx)
	details := map[string]any{}
	if ev.Root == "system" && ev.Path != "" {
		details["path"] = ev.Path
	}
	var share *string
	if ev.Root == "share" {
		share = &ev.Share
	}
	_, err := s.db.Exec(ctx, `INSERT INTO files_events(actor_id,action,root,share_key,request_id,correlation_id,details)
		VALUES((SELECT public_id FROM users WHERE id=$1),$2,$3,$4,$5,$6,$7)`, u.ID, ev.Action, ev.Root, share, ids.Request, ids.Correlation, details)
	return err
}

// RetainAudit removes a bounded batch of expired Files audit rows and expired replay records.
func RetainAudit(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('minhanos.audit_retention','on',true)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM files_events WHERE id IN (SELECT id FROM files_events WHERE occurred_at<now()-make_interval(days=>$1) ORDER BY occurred_at LIMIT 500)", AuditRetentionDays); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM files_idempotency WHERE ctid IN (SELECT ctid FROM files_idempotency WHERE expires_at<now() LIMIT 500)"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
