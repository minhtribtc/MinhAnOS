package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
	"minhanos.local/platform/internal/requestid"
)

const AccountApp = "com.minhanos.account-center"
const policyLock = 681026

type Fault struct {
	Status int
	Code   string
}

func (e *Fault) Error() string            { return e.Code }
func fault(status int, code string) error { return &Fault{status, code} }

type Actor struct {
	StorageID, PublicID, Username, Role string
	Epoch                               int64
}
type Service struct {
	db   *pgxpool.Pool
	Auth *AuthCore
	key  []byte
}

func NewService(ctx context.Context, db *pgxpool.Pool, auth *AuthCore) (*Service, error) {
	if _, e := db.Exec(ctx, "INSERT INTO identity_policy_keys(id,cursor_key) VALUES(1,$1) ON CONFLICT DO NOTHING", authn.Token()); e != nil {
		return nil, e
	}
	var secret string
	if e := db.QueryRow(ctx, "SELECT cursor_key FROM identity_policy_keys WHERE id=1").Scan(&secret); e != nil {
		return nil, e
	}
	key, e := hex.DecodeString(secret)
	if e != nil {
		return nil, e
	}
	return &Service{db, auth, key}, nil
}
func (s *Service) CanAccessHost(ctx context.Context, storageID string) (bool, error) {
	var ok bool
	e := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN identity_host_operators o ON o.user_id=u.id WHERE u.id=$1 AND u.status='active' AND u.role='admin' AND o.state='enabled')`, storageID).Scan(&ok)
	return ok, e
}
func actor(ctx context.Context, tx pgx.Tx, id string) (Actor, error) {
	var a Actor
	a.StorageID = id
	e := tx.QueryRow(ctx, "SELECT coalesce(public_id::text,''),username,role,auth_epoch FROM users WHERE id=$1 AND status='active'", id).Scan(&a.PublicID, &a.Username, &a.Role, &a.Epoch)
	return a, noRows(e)
}

// Only registered first-party Account Center endpoints supply this app context.
// Same-origin JavaScript is trusted code, not sandboxed by an appId header.
func authorize(ctx context.Context, tx pgx.Tx, a Actor, permission string) error {
	var grant bool
	e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_app_grants WHERE app_id=$1 AND permission_key=$2 AND state='enabled' AND scope='{"kind":"system","id":null}'::jsonb)`, AccountApp, permission).Scan(&grant)
	if e != nil {
		return e
	}
	if !grant {
		return fault(403, "APP_PERMISSION_DENIED")
	}
	var deny bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_app_access x WHERE app_id=$1 AND effect='deny' AND (user_id::text=$2 OR group_id IN (SELECT group_id FROM identity_group_members WHERE user_id::text=$2) OR role_id IN (SELECT r.id FROM identity_roles r WHERE r.key=CASE WHEN $3='admin' THEN 'system-admin' ELSE 'standard-user' END) OR role_id IN (SELECT role_id FROM identity_role_bindings WHERE user_id::text=$2 OR group_id IN (SELECT group_id FROM identity_group_members WHERE user_id::text=$2))))`, AccountApp, a.PublicID, a.Role).Scan(&deny)
	if e != nil {
		return e
	}
	if deny {
		return fault(403, "APP_ACCESS_DENIED")
	}
	if a.Role == "admin" {
		return nil
	}
	switch permission {
	case "identity.self.read", "identity.self.write", "identity.sessions.read", "identity.sessions.revoke", "identity.password.change":
		return nil
	}
	// Initial delegated role is bounded to managing standard users. Unknown scope,
	// custom/reserved privilege inheritance and implicit System Admin groups deny.
	var delegated bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_role_bindings b JOIN identity_roles r ON r.id=b.role_id JOIN identity_role_permissions p ON p.role_id=r.id WHERE (r.key='user-admin' OR NOT r.protected) AND p.permission_key=$2 AND b.scope='{"kind":"system","id":null}'::jsonb AND (b.user_id::text=$1 OR b.group_id IN (SELECT group_id FROM identity_group_members WHERE user_id::text=$1)))`, a.PublicID, permission).Scan(&delegated)
	if e != nil {
		return e
	}
	if !delegated {
		return fault(403, "IDENTITY_PERMISSION_DENIED")
	}
	return nil
}
func fresh(ctx context.Context, tx pgx.Tx, realm Realm, token string) error {
	t, e := table(realm)
	if e != nil {
		return e
	}
	var ok bool
	e = tx.QueryRow(ctx, "SELECT coalesce(reauth_until>now(),false) FROM "+t+" WHERE token_hash=$1", authn.Digest(token)).Scan(&ok)
	if e != nil {
		return noRows(e)
	}
	if !ok {
		return fault(403, "REAUTH_REQUIRED")
	}
	return nil
}

type Result struct {
	Body   json.RawMessage
	Status int
	ETag   string
}

func result(body any, status int, version int64) Result {
	raw, _ := json.Marshal(body)
	raw = canonicalJSON(raw)
	etag := ""
	if version > 0 {
		etag = fmt.Sprintf("\"%d\"", version)
	}
	return Result{raw, status, etag}
}
func (s *Service) mac(value string) string {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// All policy mutations take the common lock before any user row, including the
// legacy adapter. Replays reauthenticate/reauthorize before reading cached DTOs.
func (s *Service) mutate(ctx context.Context, realm Realm, token, permission, requestID, key, route string, body []byte, recent bool, fn func(pgx.Tx, Actor) (Result, error)) (Result, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", policyLock); e != nil {
		return Result{}, e
	}
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return Result{}, e
	}
	a, e := actor(ctx, tx, id)
	if e != nil {
		return Result{}, e
	}
	if e = authorize(ctx, tx, a, permission); e != nil {
		return Result{}, e
	}
	if recent {
		if e = fresh(ctx, tx, realm, token); e != nil {
			return Result{}, e
		}
	}
	var keyHash, fingerprint string
	if key != "" {
		if len(key) < 8 || len(key) > 128 {
			return Result{}, fault(400, "INVALID_IDEMPOTENCY_KEY")
		}
		keyHash = authn.Digest(key)
		fingerprint = s.mac(route + "\x00" + string(body))
		if _, e = tx.Exec(ctx, `DELETE FROM identity_mutations WHERE (actor_id,key_hash) IN (SELECT actor_id,key_hash FROM identity_mutations WHERE expires_at<=now() LIMIT 100)`); e != nil {
			return Result{}, e
		}
		var previous string
		var cached Result
		e = tx.QueryRow(ctx, "SELECT request_hash,response,status,etag FROM identity_mutations WHERE actor_id=$1 AND key_hash=$2 AND expires_at>now()", id, keyHash).Scan(&previous, &cached.Body, &cached.Status, &cached.ETag)
		if e == nil {
			if previous != fingerprint {
				return Result{}, fault(409, "IDEMPOTENCY_CONFLICT")
			}
			cached.Body = canonicalJSON(cached.Body)
			return cached, tx.Commit(ctx)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return Result{}, e
		}
		var count int
		if e = tx.QueryRow(ctx, "SELECT count(*) FROM identity_mutations WHERE actor_id=$1", id).Scan(&count); e != nil {
			return Result{}, e
		}
		if count >= 1000 {
			return Result{}, fault(429, "IDEMPOTENCY_LIMIT")
		}
		// Remove this actor's expired key even if bounded global cleanup missed it.
		if _, e = tx.Exec(ctx, "DELETE FROM identity_mutations WHERE actor_id=$1 AND key_hash=$2 AND expires_at<=now()", id, keyHash); e != nil {
			return Result{}, e
		}
	}
	if e = retain(ctx, tx); e != nil {
		return Result{}, e
	}
	out, e := fn(tx, a)
	if e != nil {
		return Result{}, e
	}
	target := auditTarget(route)
	if target == nil {
		var u struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(out.Body, &u)
		if uuidPattern.MatchString(u.ID) {
			target = u.ID
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,correlation_id) VALUES(nullif($1,'')::uuid,$2,$3,'success',$4,$5)", a.PublicID, actionName(route), target, requestID, requestid.From(ctx).Correlation); e != nil {
		return Result{}, e
	}
	if key != "" {
		if _, e = tx.Exec(ctx, "INSERT INTO identity_mutations(actor_id,key_hash,request_hash,response,status,etag,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '24 hours')", id, keyHash, fingerprint, out.Body, out.Status, out.ETag); e != nil {
			return Result{}, e
		}
	}
	return out, tx.Commit(ctx)
}
func actionName(route string) string {
	switch {
	case strings.Contains(route, "/users/") && strings.HasSuffix(route, "/sessions/revoke"):
		return "user.sessions.revoked"
	case strings.Contains(route, "/users/") && strings.HasSuffix(route, "/password"):
		return "user.password.reset"
	case strings.Contains(route, "/me/password"):
		return "password.changed"
	case strings.Contains(route, "/me/sessions"):
		return "sessions.revoked"
	case strings.Contains(route, "/role-bindings"):
		if strings.HasPrefix(route, "DELETE ") {
			return "role.binding.removed"
		}
		return "role.binding.created"
	case strings.Contains(route, "/groups"):
		if strings.HasPrefix(route, "DELETE ") && !strings.Contains(route, "/members") {
			return "group.deleted"
		}
		if strings.Contains(route, "/members") {
			if strings.HasPrefix(route, "DELETE ") {
				return "group.member.removed"
			}
			return "group.member.added"
		}
		if strings.HasPrefix(route, "PATCH ") {
			return "group.updated"
		}
		return "group.created"
	case strings.HasSuffix(route, "/roles"):
		return "role.created"
	case strings.HasSuffix(route, "/disable"):
		return "user.disabled"
	case strings.HasSuffix(route, "/enable"):
		return "user.enabled"
	case strings.HasSuffix(route, "/role"):
		return "user.role.changed"
	case strings.HasPrefix(route, "DELETE "):
		return "user.deleted"
	case strings.HasPrefix(route, "PATCH "):
		return "user.updated"
	default:
		return "user.created"
	}
}
func auditTarget(route string) any {
	p := strings.Split(route, "/")
	for _, v := range p {
		if uuidPattern.MatchString(v) {
			return v
		}
	}
	return nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

func userRow(ctx context.Context, tx pgx.Tx, id string) (User, string, error) {
	var u User
	var role string
	if !uuidPattern.MatchString(id) {
		return u, "", fault(404, "IDENTITY_NOT_FOUND")
	}
	e := tx.QueryRow(ctx, "SELECT public_id::text,username,display_name,status,version,role,password_change_required FROM users WHERE public_id=$1", id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Status, &u.Version, &role, &u.PasswordChangeRequired)
	if errors.Is(e, pgx.ErrNoRows) {
		return u, "", fault(404, "IDENTITY_NOT_FOUND")
	}
	return u, role, e
}
func targetAllowed(ctx context.Context, tx pgx.Tx, a Actor, u User, role string) error {
	if a.Role == "admin" {
		return nil
	}
	var privileged bool
	e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_role_bindings WHERE user_id=$1 OR group_id IN (SELECT group_id FROM identity_group_members WHERE user_id=$1))", u.ID).Scan(&privileged)
	if e != nil {
		return e
	}
	if role != "user" || privileged {
		return fault(403, "IDENTITY_PROTECTED_USER")
	}
	return nil
}
func lastAdmin(ctx context.Context, tx pgx.Tx, u User, role string) error {
	if role != "admin" || u.Status != "active" {
		return nil
	}
	var n int
	e := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='admin' AND status='active'").Scan(&n)
	if e != nil {
		return e
	}
	if n <= 1 {
		return fault(409, "LAST_ADMIN_REQUIRED")
	}
	return nil
}
func requireVersion(version int64, match string) error {
	if match == "" {
		return fault(428, "PRECONDITION_REQUIRED")
	}
	if match != fmt.Sprintf("\"%d\"", version) {
		return fault(412, "VERSION_CONFLICT")
	}
	return nil
}

type CreateUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

// LegacyCreate preserves the old DTO while routing creation through the same
// committing service. Creating another admin never creates a host-operator row.
func (s *Service) Create(ctx context.Context, realm Realm, token string, input CreateUser, role, key, requestID string, raw []byte, legacy bool) (Result, error) {
	input.Username = normalize(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !legacy && len([]rune(input.Password)) < 12 {
		return Result{}, ErrPasswordPolicy
	}
	if !namePattern.MatchString(input.Username) || len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 100 || (role != "admin" && role != "user") {
		return Result{}, fault(422, "IDENTITY_INVALID_USER")
	}
	if e := s.preflight(ctx, realm, token, "identity.users.write", !legacy || role == "admin"); e != nil {
		return Result{}, e
	}
	hash, e := s.Auth.PasswordHash(input.Password)
	if e != nil {
		return Result{}, e
	}
	return s.mutate(ctx, realm, token, "identity.users.write", requestID, key, "POST /api/v1/identity/users", raw, !legacy || role == "admin", func(tx pgx.Tx, a Actor) (Result, error) {
		if role == "admin" {
			if a.Role != "admin" {
				return Result{}, fault(403, "IDENTITY_PERMISSION_DENIED")
			}
			if e := authorize(ctx, tx, a, "identity.roles.write"); e != nil {
				return Result{}, e
			}
		}
		var u User
		storage := authn.Token()
		e := tx.QueryRow(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,$2,$3,$4,$5) ON CONFLICT(username) DO NOTHING RETURNING public_id::text,username,display_name,status,version", storage, input.Username, input.DisplayName, role, hash).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Status, &u.Version)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(409, "IDENTITY_USERNAME_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		if legacy {
			var exists bool
			if e = tx.QueryRow(ctx, "SELECT to_regclass('audit') IS NOT NULL").Scan(&exists); e != nil {
				return Result{}, e
			}
			if exists {
				if _, e = tx.Exec(ctx, "INSERT INTO audit(user_id,actor,action,detail) VALUES($1,$2,'user.create',$3)", a.StorageID, a.Username, "Created account "+u.Username+" ("+role+")"); e != nil {
					return Result{}, e
				}
			}
		}
		if legacy {
			return result(map[string]string{"id": storage, "username": u.Username, "displayName": u.DisplayName, "role": role}, 201, 0), nil
		}
		return result(u, 201, u.Version), nil
	})
}
func (s *Service) ChangeUser(ctx context.Context, realm Realm, token, target, operation, name, match, key, requestID string, raw []byte) (Result, error) {
	permission := "identity.users.write"
	recent := true
	if operation == "system-admin" || operation == "user-admin" || operation == "standard-user" {
		permission = "identity.roles.write"
	}
	if operation == "self" {
		permission = "identity.self.write"
		recent = false
	}
	route := map[string]string{"patch": "PATCH /api/v1/identity/users/", "self": "PATCH /api/v1/identity/users/", "disable": "POST /api/v1/identity/users/", "enable": "POST /api/v1/identity/users/", "delete": "DELETE /api/v1/identity/users/", "system-admin": "PUT /api/v1/identity/users/", "user-admin": "PUT /api/v1/identity/users/", "standard-user": "PUT /api/v1/identity/users/"}[operation] + target
	if operation == "disable" || operation == "enable" {
		route += "/" + operation
	}
	if operation == "system-admin" || operation == "user-admin" || operation == "standard-user" {
		route += "/role"
	}
	return s.mutate(ctx, realm, token, permission, requestID, key, route, append([]byte(match+"\x00"), raw...), recent, func(tx pgx.Tx, a Actor) (Result, error) {
		if operation == "self" && target != a.PublicID {
			return Result{}, fault(403, "IDENTITY_PERMISSION_DENIED")
		}
		u, role, e := userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if operation != "self" {
			if e = targetAllowed(ctx, tx, a, u, role); e != nil {
				return Result{}, e
			}
		}
		if e = requireVersion(u.Version, match); e != nil {
			return Result{}, e
		}
		if u.Status == "deleted" {
			return Result{}, fault(409, "IDENTITY_USER_DELETED")
		}
		switch operation {
		case "self", "patch":
			name = strings.TrimSpace(name)
			if len([]rune(name)) < 1 || len([]rune(name)) > 100 {
				return Result{}, fault(422, "IDENTITY_INVALID_USER")
			}
			_, e = tx.Exec(ctx, "UPDATE users SET display_name=$2,version=version+1 WHERE public_id=$1", target, name)
		case "disable", "delete":
			if e = lastAdmin(ctx, tx, u, role); e != nil {
				return Result{}, e
			}
			if operation == "delete" && u.Status != "disabled" {
				return Result{}, fault(409, "IDENTITY_DISABLE_FIRST")
			}
			status := "disabled"
			if operation == "delete" {
				status = "deleted"
			}
			_, e = tx.Exec(ctx, "UPDATE users SET status=$2,version=version+1 WHERE public_id=$1", target, status)
			if e == nil {
				_, e = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE public_id=$1);", target)
			}
			if e == nil {
				_, e = tx.Exec(ctx, "DELETE FROM drive_sessions WHERE user_id=(SELECT id FROM users WHERE public_id=$1)", target)
			}
			if e == nil && operation == "delete" {
				_, e = tx.Exec(ctx, "UPDATE identity_host_operators SET state='revoked' WHERE user_id=(SELECT id FROM users WHERE public_id=$1)", target)
			}
		case "enable":
			_, e = tx.Exec(ctx, "UPDATE users SET status='active',version=version+1 WHERE public_id=$1", target)
		case "system-admin", "user-admin", "standard-user":
			if a.Role != "admin" {
				return Result{}, fault(403, "IDENTITY_PERMISSION_DENIED")
			}
			if operation == "standard-user" {
				var inherited bool
				if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_group_members m JOIN identity_role_bindings b ON b.group_id=m.group_id JOIN identity_roles r ON r.id=b.role_id WHERE m.user_id=$1)", target).Scan(&inherited); e != nil {
					return Result{}, e
				}
				if inherited {
					return Result{}, fault(409, "IDENTITY_GROUP_ROLE_CONFLICT")
				}
			}
			if operation != "system-admin" {
				if e = lastAdmin(ctx, tx, u, role); e != nil {
					return Result{}, e
				}
			}
			next := "user"
			if operation == "system-admin" {
				next = "admin"
			}
			_, e = tx.Exec(ctx, "UPDATE users SET role=$2,version=version+1,auth_epoch=auth_epoch+1,recovery_token_hash=NULL,recovery_expires_at=NULL,recovery_auth_epoch=NULL WHERE public_id=$1", target, next)
			if e == nil {
				_, e = tx.Exec(ctx, "DELETE FROM identity_role_bindings WHERE user_id=$1", target)
			}
			if e == nil && operation == "user-admin" {
				_, e = tx.Exec(ctx, `INSERT INTO identity_role_bindings(role_id,user_id,scope) SELECT id,$1,'{"kind":"system","id":null}'::jsonb FROM identity_roles WHERE key='user-admin'`, target)
			}
			if e == nil && next != "admin" {
				_, e = tx.Exec(ctx, "UPDATE identity_host_operators SET state='revoked' WHERE user_id=(SELECT id FROM users WHERE public_id=$1)", target)
			}
			if e == nil {
				_, e = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE public_id=$1)", target)
			}
			if e == nil {
				_, e = tx.Exec(ctx, "DELETE FROM drive_sessions WHERE user_id=(SELECT id FROM users WHERE public_id=$1)", target)
			}
			if e == nil {
				var storage string
				if storage, e = storageIDOf(ctx, tx, target); e == nil {
					e = notify(ctx, tx, storage, "role.changed", "warning", true, map[string]any{"role": operation})
				}
			}
		default:
			return Result{}, fault(400, "INVALID_REQUEST")
		}
		if e != nil {
			return Result{}, e
		}
		u, _, e = userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if operation == "delete" {
			return result(map[string]bool{"accepted": true}, 200, 0), nil
		}
		return result(u, 200, u.Version), nil
	})
}

type cursor struct {
	Actor, Route, Search, After string
	Epoch, Expires              int64
}

func (s *Service) encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	v := base64.RawURLEncoding.EncodeToString(b)
	return v + "." + s.mac("cursor\x00"+v)
}
func (s *Service) decodeCursor(value string, a Actor, route, search string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 2048 {
		return "", fault(400, "INVALID_CURSOR")
	}
	p := strings.Split(value, ".")
	if len(p) != 2 || !hmac.Equal([]byte(p[1]), []byte(s.mac("cursor\x00"+p[0]))) {
		return "", fault(400, "INVALID_CURSOR")
	}
	b, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return "", fault(400, "INVALID_CURSOR")
	}
	var c cursor
	if json.Unmarshal(b, &c) != nil || c.Actor != a.PublicID || c.Epoch != a.Epoch || c.Route != route || c.Search != search || c.Expires < time.Now().Unix() {
		return "", fault(400, "INVALID_CURSOR")
	}
	return c.After, nil
}

func (s *Service) preflight(ctx context.Context, realm Realm, token, permission string, recent bool) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	a, e := actor(ctx, tx, id)
	if e != nil {
		return e
	}
	if e = authorize(ctx, tx, a, permission); e != nil {
		return e
	}
	if recent {
		if e = fresh(ctx, tx, realm, token); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// PostgreSQL jsonb reorders keys; normalize both original and cached DTOs so an
// idempotent replay returns identical JSON bytes without storing credential input.
func canonicalJSON(raw []byte) json.RawMessage {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return raw
	}
	b, e := json.Marshal(v)
	if e != nil {
		return raw
	}
	return b
}

func (s *Service) Check(ctx context.Context, realm Realm, token, permission string) error {
	return s.preflight(ctx, realm, token, permission, false)
}

// v1 must not silently omit historical users that have not been backfilled.
func (s *Service) Ready(ctx context.Context) error {
	var pending bool
	if e := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE public_id IS NULL OR password_credential_id IS NULL)").Scan(&pending); e != nil {
		return e
	}
	if pending {
		return ErrNotReady
	}
	return nil
}
