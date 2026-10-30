package identity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"minhanos.local/platform/internal/authn"
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
	permission := map[string]string{"me": "identity.self.read", "user": "identity.users.read", "users": "identity.users.read", "roles": "identity.roles.read", "permissions": "identity.roles.read", "audit-events": "identity.audit.read", "me/sessions": "identity.sessions.read", "me/credentials": "identity.self.read", "groups": "identity.groups.read", "members": "identity.groups.read", "role-bindings": "identity.roles.read", "me/access": "identity.self.read", "user/access": "identity.users.read"}[kind]
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
	if kind == "me/access" {
		return s.ownAccess(ctx, tx, a)
	}
	if kind == "user/access" {
		u, role, e := userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if e = targetAllowed(ctx, tx, a, u, role); e != nil {
			return Result{}, e
		}
		subject := Actor{PublicID: u.ID, Username: u.Username, Role: role}
		if e = tx.QueryRow(ctx, "SELECT id,auth_epoch FROM users WHERE public_id=$1", target).Scan(&subject.StorageID, &subject.Epoch); e != nil {
			return Result{}, e
		}
		return s.ownAccess(ctx, tx, subject)
	}
	if kind == "members" {
		if _, e = readGroup(ctx, tx, target); e != nil {
			return Result{}, e
		}
	}
	cursorKind := kind
	if kind == "members" {
		cursorKind += "/" + target
	}
	after, e := s.decodeCursor(page, a, cursorKind, search)
	if e != nil {
		return Result{}, e
	}
	var rows pgx.Rows
	switch kind {
	case "me/sessions":
		rows, e = tx.Query(ctx, `SELECT handle,jsonb_build_object('id',handle,'userId',$2::uuid,'realm',realm,'current',(realm=$3 AND token_hash=$4),'createdAt',created_at,'lastSeenAt',NULL,'expiresAt',expires_at,'deviceHint',NULL) FROM (SELECT 'desktop' realm,'desktop:'||public_id::text handle,s.* FROM sessions s UNION ALL SELECT 'files' realm,'files:'||public_id::text handle,s.* FROM drive_sessions s) s WHERE user_id=$1 AND auth_epoch=$5 AND expires_at>now() AND handle>$6 ORDER BY handle LIMIT $7`, a.StorageID, a.PublicID, string(realm), authn.Digest(token), a.Epoch, after, limit+1)
	case "me/credentials":
		rows, e = tx.Query(ctx, `SELECT password_credential_id::text,jsonb_build_object('id',password_credential_id,'userId',public_id,'kind','password','state','active','lastChangedAt',password_changed_at) FROM users WHERE id=$1 AND password_credential_id::text>$2 ORDER BY password_credential_id::text LIMIT $3`, a.StorageID, after, limit+1)
	case "groups":
		rows, e = tx.Query(ctx, `SELECT id::text,jsonb_build_object('id',id,'key',key,'name',name,'kind',kind,'version',version) FROM identity_groups WHERE id::text>$1 ORDER BY id::text LIMIT $2`, after, limit+1)
	case "members":
		rows, e = tx.Query(ctx, `SELECT user_id::text,jsonb_build_object('userId',user_id) FROM identity_group_members WHERE group_id=$1 AND user_id::text>$2 ORDER BY user_id::text LIMIT $3`, target, after, limit+1)
	case "role-bindings":
		rows, e = tx.Query(ctx, `SELECT id::text,jsonb_build_object('id',id,'roleId',role_id,'subject',jsonb_build_object('kind',CASE WHEN user_id IS NOT NULL THEN 'user' ELSE 'group' END,'id',coalesce(user_id,group_id)),'scope',scope,'version',version) FROM identity_role_bindings WHERE id::text>$1 ORDER BY id::text LIMIT $2`, after, limit+1)
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
		next = s.encodeCursor(cursor{a.PublicID, cursorKind, search, last, a.Epoch, time.Now().Add(15 * time.Minute).Unix()})
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

func (s *Service) ownAccess(ctx context.Context, tx pgx.Tx, a Actor) (Result, error) {
	permissions := []string{}
	var active bool
	if e := tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1", a.StorageID).Scan(&active); e != nil {
		return Result{}, e
	}
	rows, e := tx.Query(ctx, "SELECT key FROM identity_permissions ORDER BY key")
	if e != nil {
		return Result{}, e
	}
	keys := []string{}
	for rows.Next() {
		var k string
		if e = rows.Scan(&k); e != nil {
			rows.Close()
			return Result{}, e
		}
		keys = append(keys, k)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return Result{}, e
	}
	for _, k := range keys {
		if !active {
			break
		}
		if e = authorize(ctx, tx, a, k); e == nil {
			permissions = append(permissions, k)
		} else {
			var f *Fault
			if !errors.As(e, &f) {
				return Result{}, e
			}
		}
	}
	baseRole := "standard-user"
	if a.Role == "admin" {
		baseRole = "system-admin"
	} else {
		var delegated bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_role_bindings b JOIN identity_roles r ON r.id=b.role_id WHERE r.key='user-admin' AND b.scope='{"kind":"system","id":null}' AND (b.user_id::text=$1 OR b.group_id IN (SELECT group_id FROM identity_group_members WHERE user_id::text=$1)))`, a.PublicID).Scan(&delegated); e != nil {
			return Result{}, e
		}
		if delegated {
			baseRole = "user-admin"
		}
	}
	var host bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_host_operators WHERE user_id=$1 AND state='enabled')", a.StorageID).Scan(&host); e != nil {
		return Result{}, e
	}
	return result(map[string]any{"userId": a.PublicID, "baseRole": baseRole, "permissions": permissions, "hostOperator": host && active && a.Role == "admin"}, 200, 0), tx.Commit(ctx)
}
