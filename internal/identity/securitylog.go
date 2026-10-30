package identity

import (
	"context"

	"github.com/jackc/pgx/v5"
	"minhanos.local/platform/internal/requestid"
)

// Retention of the structured security log and of undelivered security notices (migration 007).
const (
	auditRetentionDays  = 365
	noticeRetentionDays = 90
)

// securityEvent records an AuthCore action that does not go through the policy mutate path
// (self-service primitives, local recovery) in the same transaction as the change.
// selfActor marks the user as the actor; false means a local operator outside the web.
func securityEvent(ctx context.Context, tx pgx.Tx, storageID, action string, selfActor bool, reasonCode string) error {
	ids := requestid.From(ctx)
	details := map[string]any{}
	if reasonCode != "" {
		details["reasonCode"] = reasonCode
	}
	_, e := tx.Exec(ctx, "INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,correlation_id,details) SELECT CASE WHEN $5 THEN public_id END,$2,public_id::text,'success',$3,$4,$6 FROM users WHERE id=$1", storageID, action, ids.Request, ids.Correlation, selfActor, details)
	return e
}

// notify queues a security notice for one user in the caller's transaction. Users without a
// public ID (not yet backfilled) get none; the change itself is still audited.
func notify(ctx context.Context, tx pgx.Tx, storageID, kind, severity string, mandatory bool, details map[string]any) error {
	ids := requestid.From(ctx)
	if details == nil {
		details = map[string]any{}
	}
	_, e := tx.Exec(ctx, "INSERT INTO identity_notification_outbox(recipient_id,kind,severity,mandatory,details,request_id,correlation_id) SELECT public_id,$2,$3,$4,$5,$6,$7 FROM users WHERE id=$1 AND public_id IS NOT NULL", storageID, kind, severity, mandatory, details, ids.Request, ids.Correlation)
	return e
}

// retain removes a bounded batch of expired audit rows and notices. The append-only trigger
// only permits deleting audit rows older than the retention period while this flag is set.
func retain(ctx context.Context, tx pgx.Tx) error {
	if _, e := tx.Exec(ctx, "SELECT set_config('minhanos.audit_retention','on',true)"); e != nil {
		return e
	}
	if _, e := tx.Exec(ctx, "DELETE FROM identity_admin_events WHERE id IN (SELECT id FROM identity_admin_events WHERE occurred_at<now()-make_interval(days=>$1) ORDER BY occurred_at LIMIT 100)", auditRetentionDays); e != nil {
		return e
	}
	if _, e := tx.Exec(ctx, "SELECT set_config('minhanos.audit_retention','off',true)"); e != nil {
		return e
	}
	_, e := tx.Exec(ctx, "DELETE FROM identity_notification_outbox WHERE id IN (SELECT id FROM identity_notification_outbox WHERE created_at<now()-make_interval(days=>$1) LIMIT 100)", noticeRetentionDays)
	return e
}

func storageIDOf(ctx context.Context, tx pgx.Tx, publicID string) (string, error) {
	var id string
	e := tx.QueryRow(ctx, "SELECT id FROM users WHERE public_id=$1", publicID).Scan(&id)
	return id, e
}
