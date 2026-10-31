package drive

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/jobcore"
	"minhanos.local/platform/internal/migrate"
	"minhanos.local/platform/internal/requestid"
)

//go:embed migrations/001_files_events.sql
var filesEventsSQL string

//go:embed migrations/002_files_idempotency.sql
var filesIdempotencySQL string

//go:embed migrations/003_files_jobs.sql
var filesJobsSQL string

//go:embed migrations/004_files_removals.sql
var filesRemovalsSQL string

//go:embed migrations/005_files_events_organization.sql
var filesEventsOrganizationSQL string

//go:embed migrations/006_files_jobs_platform.sql
var filesJobsPlatformSQL string

//go:embed migrations/007_files_favorites.sql
var filesFavoritesSQL string

//go:embed migrations/008_files_sharing.sql
var filesSharingSQL string

//go:embed migrations/009_files_activity.sql
var filesActivitySQL string

// Module is the Files schema owned by this package (migration runner, app standard row 18).
func Module() migrate.Module {
	return migrate.Module{ID: "files", Dependencies: map[string]migrate.Range{"jobs": {Min: 1, Max: 100}}, Steps: []migrate.Step{{ID: 1, Name: "files_events", SQL: filesEventsSQL}, {ID: 2, Name: "files_idempotency", SQL: filesIdempotencySQL}, {ID: 3, Name: "files_jobs", SQL: filesJobsSQL}, {ID: 4, Name: "files_removals", SQL: filesRemovalsSQL}, {ID: 5, Name: "files_events_organization", SQL: filesEventsOrganizationSQL}, {ID: 6, Name: "files_jobs_platform", SQL: filesJobsPlatformSQL}, {ID: 7, Name: "files_favorites", SQL: filesFavoritesSQL}, {ID: 8, Name: "files_sharing", SQL: filesSharingSQL}, {ID: 9, Name: "files_activity", SQL: filesActivitySQL}}}
}

func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, jobcore.Module(), Module())
}

// AuditRetentionDays bounds files_events; Log Center reports it.
const AuditRetentionDays = 365

// AuditEvent is one Files event. Paths and names go in only when the root is the host system or the activity level
// (ADR 0012) asks for them; at the default level homes and Shared Folders record none (privacy, row 17).
type AuditEvent struct {
	Action  string         // files.<operation>
	Root    string         // personal, share, system
	Share   string         // share key when Root is share
	Path    string         // logical path, when recorded
	Outcome string         // success (default), denied, failure
	Details map[string]any // extra fields allowed by the level (names, destination, bytes, client)
}

func (s *SQLIdentity) Audit(ctx context.Context, u User, ev AuditEvent) error {
	ids := requestid.From(ctx)
	details := map[string]any{}
	for k, v := range ev.Details {
		details[k] = v
	}
	if ev.Path != "" {
		details["path"] = ev.Path
	}
	if ev.Outcome == "" {
		ev.Outcome = "success"
	}
	var share *string
	if ev.Root == "share" {
		share = &ev.Share
	}
	_, err := s.db.Exec(ctx, `INSERT INTO files_events(actor_id,action,root,share_key,request_id,correlation_id,details,organization_id,outcome)
		VALUES((SELECT public_id FROM users WHERE id=$1),$2,$3,$4,$5,$6,$7,(SELECT organization_id FROM storage_shares WHERE key=$4),$8)`, u.ID, ev.Action, ev.Root, share, ids.Request, ids.Correlation, details, ev.Outcome)
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
