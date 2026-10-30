package identity

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Read(ctx context.Context, realm Realm, token, kind, target, search, page string, limit int) (Result, error) {
	if limit < 1 || limit > 100 || len(search) > 100 {
		return Result{}, fault(400, "INVALID_REQUEST")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return Result{}, e
	}
	a, e := actor(ctx, tx, id)
	if e != nil {
		return Result{}, e
	}
	if a.PublicID == "" {
		return Result{}, ErrNotReady
	}
	permission := map[string]string{"me": "identity.self.read", "user": "identity.users.read", "users": "identity.users.read", "roles": "identity.roles.read", "permissions": "identity.roles.read", "audit-events": "identity.audit.read"}[kind]
	if permission == "" {
		return Result{}, fault(404, "IDENTITY_NOT_FOUND")
	}
	if e = authorize(ctx, tx, a, permission); e != nil {
		return Result{}, e
	}
	if kind == "me" || kind == "user" {
		if kind == "me" {
			target = a.PublicID
		}
		u, role, e := userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if kind == "user" {
			if e = targetAllowed(ctx, tx, a, u, role); e != nil {
				return Result{}, e
			}
		}
		return result(u, 200, u.Version), tx.Commit(ctx)
	}
	after, e := s.decodeCursor(page, a, kind, search)
	if e != nil {
		return Result{}, e
	}
	var rows pgx.Rows
	switch kind {
	case "users":
		rows, e = tx.Query(ctx, `SELECT public_id::text,jsonb_build_object('id',public_id,'username',username,'displayName',display_name,'status',status,'version',version) FROM users u WHERE public_id IS NOT NULL AND public_id::text>$1 AND position($2 in username)>0 AND ($3 OR (role='user' AND NOT EXISTS(SELECT 1 FROM identity_role_bindings b WHERE b.user_id=u.public_id OR b.group_id IN (SELECT group_id FROM identity_group_members WHERE user_id=u.public_id)))) ORDER BY public_id::text LIMIT $4`, after, normalize(search), a.Role == "admin", limit+1)
	case "roles":
		rows, e = tx.Query(ctx, `SELECT r.id::text,jsonb_build_object('id',r.id,'key',r.key,'name',r.name,'protected',r.protected,'permissions',(SELECT coalesce(jsonb_agg(permission_key ORDER BY permission_key),'[]'::jsonb) FROM identity_role_permissions WHERE role_id=r.id),'version',r.version) FROM identity_roles r WHERE id::text>$1 ORDER BY id::text LIMIT $2`, after, limit+1)
	case "permissions":
		rows, e = tx.Query(ctx, `SELECT key,jsonb_build_object('key',key,'descriptionKey','accounts.permissions.'||key,'resourceTypes',jsonb_build_array('identity')) FROM identity_permissions WHERE key>$1 ORDER BY key LIMIT $2`, after, limit+1)
	case "audit-events":
		rows, e = tx.Query(ctx, `SELECT id::text,jsonb_build_object('id',id,'actorId',actor_id,'action',action,'targetId',target_id,'outcome',outcome,'occurredAt',occurred_at,'requestId',request_id,'details',details) FROM identity_admin_events WHERE id::text>$1 ORDER BY id::text LIMIT $2`, after, limit+1)
	}
	if e != nil {
		return Result{}, e
	}
	defer rows.Close()
	items := []json.RawMessage{}
	last := ""
	more := false
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if e = rows.Scan(&key, &raw); e != nil {
			return Result{}, e
		}
		if len(items) == limit {
			more = true
			break
		}
		items = append(items, raw)
		last = key
	}
	if e = rows.Err(); e != nil {
		return Result{}, e
	}
	rows.Close()
	var next any
	if more {
		next = s.encodeCursor(cursor{a.PublicID, kind, search, last, a.Epoch, time.Now().Add(15 * time.Minute).Unix()})
	}
	return result(map[string]any{"items": items, "nextCursor": next}, 200, 0), tx.Commit(ctx)
}

func (s *Service) RecordDenied(ctx context.Context, storageID, requestID, route, code string) {
	// Best-effort failure audit, never include raw body, secret, username or headers.
	if len(code) > 64 || !strings.HasPrefix(code, "IDENTITY_") && code != "APP_ACCESS_DENIED" && code != "APP_PERMISSION_DENIED" && code != "REAUTH_REQUIRED" {
		return
	}
	_, _ = s.db.Exec(ctx, `INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,details) SELECT public_id,'identity.denied',$2,'denied',$3,jsonb_build_object('reasonCode',$4::text) FROM users WHERE id=$1`, storageID, auditTarget(route), requestID, code)
}
