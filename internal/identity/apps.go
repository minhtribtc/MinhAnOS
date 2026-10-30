package identity

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CheckApp authorizes one request of a server-side app context (Files calls it per request):
// active user AND app grant AND no deny rule AND entitlement. Incomplete UUID backfill fails closed.
func (s *Service) CheckApp(ctx context.Context, storageID, app, permission string) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	a, e := actor(ctx, tx, storageID)
	if e != nil {
		return e
	}
	if a.PublicID == "" {
		return ErrNotReady
	}
	if e = authorizeApp(ctx, tx, a, app, permission); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type AppAccessInput struct {
	Subject struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"subject"`
}

const appAccessColumns = `jsonb_build_object('id',id,'appId',app_id,'effect',effect,'subject',jsonb_build_object('kind',CASE WHEN user_id IS NOT NULL THEN 'user' WHEN group_id IS NOT NULL THEN 'group' ELSE 'role' END,'id',coalesce(user_id,group_id,role_id)),'version',version)`

// applications lists first-party apps with whether the caller may use them.
func (s *Service) applications(ctx context.Context, tx pgx.Tx, a Actor) (Result, error) {
	items := []map[string]any{}
	for _, app := range []struct{ id, name, permission string }{{AccountApp, "Account Center", "identity.self.read"}, {FilesApp, governedApps[FilesApp], "files.read"}} {
		e := authorizeApp(ctx, tx, a, app.id, app.permission)
		var f *Fault
		if e != nil && !errors.As(e, &f) {
			return Result{}, e
		}
		items = append(items, map[string]any{"appId": app.id, "displayName": app.name, "allowed": e == nil})
	}
	return result(map[string]any{"items": items, "nextCursor": nil}, 200, 0), tx.Commit(ctx)
}

// appAccess lists the deny rules of one governed app (at most 500; rules are per subject).
func (s *Service) appAccess(ctx context.Context, tx pgx.Tx, app string) (Result, error) {
	if _, ok := governedApps[app]; !ok {
		return Result{}, fault(404, "IDENTITY_NOT_FOUND")
	}
	rows, e := tx.Query(ctx, "SELECT "+appAccessColumns+" FROM identity_app_access WHERE app_id=$1 AND effect='deny' ORDER BY id LIMIT 500", app)
	if e != nil {
		return Result{}, e
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if e = rows.Scan(&raw); e != nil {
			return Result{}, e
		}
		items = append(items, raw)
	}
	if e = rows.Err(); e != nil {
		return Result{}, e
	}
	rows.Close()
	return result(map[string]any{"items": items, "nextCursor": nil}, 200, 0), tx.Commit(ctx)
}

// AppAccess adds or removes a deny rule for a user, group or role on a governed app. System Admin
// only, recent reauthentication and idempotency; Account Center itself can never be denied this way.
func (s *Service) AppAccess(ctx context.Context, realm Realm, token, app, ruleID, key, requestID string, input AppAccessInput, raw []byte, remove bool) (Result, error) {
	route := "POST /api/v1/identity/applications/" + app + "/access"
	if remove {
		route = "DELETE /api/v1/identity/applications/" + app + "/access/" + ruleID
	}
	return s.mutate(ctx, realm, token, "identity.apps.write", requestID, key, route, raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if e := managementActor(a); e != nil {
			return Result{}, e
		}
		if app == AccountApp {
			return Result{}, fault(403, "IDENTITY_PROTECTED_POLICY")
		}
		if _, ok := governedApps[app]; !ok {
			return Result{}, fault(404, "IDENTITY_NOT_FOUND")
		}
		if remove {
			if !uuidPattern.MatchString(ruleID) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			tag, e := tx.Exec(ctx, "DELETE FROM identity_app_access WHERE id=$1 AND app_id=$2", ruleID, app)
			if e != nil {
				return Result{}, e
			}
			if tag.RowsAffected() != 1 {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			return result(map[string]bool{"accepted": true}, 200, 0), nil
		}
		column := map[string]string{"user": "user_id", "group": "group_id", "role": "role_id"}[input.Subject.Kind]
		if column == "" || !uuidPattern.MatchString(input.Subject.ID) {
			return Result{}, fault(422, "IDENTITY_INVALID_SUBJECT")
		}
		var exists bool
		check := map[string]string{"user": "SELECT EXISTS(SELECT 1 FROM users WHERE public_id=$1 AND status<>'deleted')", "group": "SELECT EXISTS(SELECT 1 FROM identity_groups WHERE id=$1)", "role": "SELECT EXISTS(SELECT 1 FROM identity_roles WHERE id=$1)"}[input.Subject.Kind]
		if e := tx.QueryRow(ctx, check, input.Subject.ID).Scan(&exists); e != nil {
			return Result{}, e
		}
		if !exists {
			return Result{}, fault(404, "IDENTITY_NOT_FOUND")
		}
		var rule json.RawMessage
		e := tx.QueryRow(ctx, "INSERT INTO identity_app_access(app_id,"+column+",effect) VALUES($1,$2,'deny') ON CONFLICT DO NOTHING RETURNING "+appAccessColumns, app, input.Subject.ID).Scan(&rule)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(409, "IDENTITY_BINDING_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		return Result{Body: canonicalJSON(rule), Status: 201}, nil
	})
}
