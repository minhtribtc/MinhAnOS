package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
)

var (
	ErrUnauthorized   = errors.New("invalid credentials or session")
	ErrThrottled      = errors.New("authentication temporarily throttled")
	ErrBusy           = errors.New("authentication capacity unavailable")
	ErrPasswordPolicy = errors.New("password must contain 12..256 bytes")
)

type Realm string

const (
	DesktopRealm Realm = "desktop"
	DriveRealm   Realm = "files"
)

func table(realm Realm) (string, error) {
	switch realm {
	case DesktopRealm:
		return "sessions", nil
	case DriveRealm:
		return "drive_sessions", nil
	}
	return "", errors.New("unknown session realm")
}

// Principal is internal compatibility data. Never serialize this as a v1 User.
type Principal struct{ StorageID, Username, DisplayName, Role string }
type AuthSession struct {
	Principal Principal
	CSRF      string
}
type OwnSession struct {
	ID        string     `json:"id"`
	CreatedAt *time.Time `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	Current   bool       `json:"current"`
}

// AuthCore owns credentials/session mechanics, not RBAC or HTTP policy.
// No method accepts a client-supplied subject as proof of authority.
type AuthCore struct {
	db    *pgxpool.Pool
	dummy string
	slots chan struct{}
}

func NewAuthCore(db *pgxpool.Pool) (*AuthCore, error) {
	dummy, e := authn.Hash(authn.Token())
	if e != nil {
		return nil, e
	}
	return &AuthCore{db: db, dummy: dummy, slots: make(chan struct{}, 2)}, nil
}
func (a *AuthCore) acquire() (func(), error) {
	select {
	case a.slots <- struct{}{}:
		return func() { <-a.slots }, nil
	default:
		return nil, ErrBusy
	}
}
func normalize(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
func validToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	for _, c := range token {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func noRows(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrUnauthorized
	}
	return e
}

// Reserve a bounded per-account attempt before expensive hashing. PostgreSQL shares
// the fixed window across processes/realms. Keys are hashed, never raw usernames.
func (a *AuthCore) attempt(ctx context.Context, name string) error {
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(681025)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `DELETE FROM identity_auth_attempts WHERE key_hash IN (SELECT key_hash FROM identity_auth_attempts WHERE expires_at<=now() ORDER BY expires_at LIMIT 100)`); e != nil {
		return e
	}
	var count int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM identity_auth_attempts").Scan(&count); e != nil {
		return e
	}
	key := authn.Digest(normalize(name))
	if count >= 10000 {
		var exists bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_auth_attempts WHERE key_hash=$1)", key).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return ErrThrottled
		}
	}
	var n int
	e = tx.QueryRow(ctx, `INSERT INTO identity_auth_attempts(key_hash,attempts,expires_at) VALUES($1,1,now()+interval '10 minutes') ON CONFLICT(key_hash) DO UPDATE SET attempts=CASE WHEN identity_auth_attempts.expires_at<=now() THEN 1 ELSE least(identity_auth_attempts.attempts+1,21) END,expires_at=CASE WHEN identity_auth_attempts.expires_at<=now() THEN now()+interval '10 minutes' ELSE identity_auth_attempts.expires_at END RETURNING attempts`, key).Scan(&n)
	if e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	if n > 20 {
		return ErrThrottled
	}
	return nil
}

func (a *AuthCore) Authenticate(ctx context.Context, realm Realm, token string) (AuthSession, error) {
	var s AuthSession
	t, e := table(realm)
	if e != nil {
		return s, e
	}
	if !validToken(token) {
		return s, ErrUnauthorized
	}
	e = a.db.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.role,s.csrf_token FROM `+t+` s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.status='active' AND s.auth_epoch=u.auth_epoch`, authn.Digest(token)).Scan(&s.Principal.StorageID, &s.Principal.Username, &s.Principal.DisplayName, &s.Principal.Role, &s.CSRF)
	return s, noRows(e)
}

func (a *AuthCore) Login(ctx context.Context, realm Realm, name, password, oldToken string) (AuthSession, string, error) {
	var s AuthSession
	t, e := table(realm)
	if e != nil {
		return s, "", e
	}
	name = normalize(name)
	if len(name) > 64 || len(password) > 256 {
		return s, "", ErrUnauthorized
	}
	release, e := a.acquire()
	if e != nil {
		return s, "", e
	}
	defer release()
	if e = a.attempt(ctx, name); e != nil {
		return s, "", e
	}
	hash, status := a.dummy, ""
	var epoch int64
	e = a.db.QueryRow(ctx, `SELECT id,username,display_name,role,password_hash,status,auth_epoch FROM users WHERE username=$1`, name).Scan(&s.Principal.StorageID, &s.Principal.Username, &s.Principal.DisplayName, &s.Principal.Role, &hash, &status, &epoch)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return AuthSession{}, "", e
	}
	valid := authn.Verify(password, hash)
	if e != nil || !valid || status != "active" {
		return AuthSession{}, "", ErrUnauthorized
	}
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return AuthSession{}, "", e
	}
	defer tx.Rollback(ctx)
	// Lock after hashing, then recheck the exact credential/epoch. This serializes
	// issuance with password changes/revocation without holding a lock during KDF.
	var currentHash, currentStatus string
	var currentEpoch int64
	e = tx.QueryRow(ctx, "SELECT password_hash,status,auth_epoch,display_name,role FROM users WHERE id=$1 FOR UPDATE", s.Principal.StorageID).Scan(&currentHash, &currentStatus, &currentEpoch, &s.Principal.DisplayName, &s.Principal.Role)
	if e != nil {
		return AuthSession{}, "", noRows(e)
	}
	if currentHash != hash || currentStatus != "active" || currentEpoch != epoch {
		return AuthSession{}, "", ErrUnauthorized
	}
	raw := authn.Token()
	s.CSRF = authn.Token()
	// Lazy, single-row expansion after verified login; never rekeys storage or
	// runs an unbounded startup backfill. Assigned public references stay immutable.
	if _, e = tx.Exec(ctx, "UPDATE users SET public_id=coalesce(public_id,gen_random_uuid()),password_credential_id=coalesce(password_credential_id,gen_random_uuid()) WHERE id=$1 AND (public_id IS NULL OR password_credential_id IS NULL)", s.Principal.StorageID); e != nil {
		return AuthSession{}, "", e
	}
	if validToken(oldToken) {
		if _, e = tx.Exec(ctx, "DELETE FROM "+t+" WHERE token_hash=$1", authn.Digest(oldToken)); e != nil {
			return AuthSession{}, "", e
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO "+t+"(token_hash,user_id,csrf_token,expires_at,auth_epoch) VALUES($1,$2,$3,now()+interval '24 hours',$4)", authn.Digest(raw), s.Principal.StorageID, s.CSRF, epoch); e != nil {
		return AuthSession{}, "", e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM identity_auth_attempts WHERE key_hash=$1", authn.Digest(name)); e != nil {
		return AuthSession{}, "", e
	}
	e = tx.Commit(ctx)
	if e != nil {
		return AuthSession{}, "", e
	}
	return s, raw, nil
}

func (a *AuthCore) Logout(ctx context.Context, realm Realm, token string) error {
	t, e := table(realm)
	if e != nil {
		return e
	}
	if !validToken(token) {
		return ErrUnauthorized
	}
	_, e = a.db.Exec(ctx, "DELETE FROM "+t+" WHERE token_hash=$1", authn.Digest(token))
	return e
}

// Reauthenticate grants only a short-lived, epoch-bound server-side freshness
// marker on the verified session. It never accepts a frontend subject/app claim.
func (a *AuthCore) Reauthenticate(ctx context.Context, realm Realm, token, password string) error {
	if len(password) > 256 {
		return ErrUnauthorized
	}
	release, e := a.acquire()
	if e != nil {
		return e
	}
	defer release()
	session, e := a.Authenticate(ctx, realm, token)
	if e != nil {
		return e
	}
	if e = a.attempt(ctx, session.Principal.Username); e != nil {
		return e
	}
	var hash string
	if e = a.db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", session.Principal.StorageID).Scan(&hash); e != nil {
		return noRows(e)
	}
	if !authn.Verify(password, hash) {
		return ErrUnauthorized
	}
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	var fresh string
	if e = tx.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", id).Scan(&fresh); e != nil {
		return e
	}
	if fresh != hash {
		return ErrUnauthorized
	}
	t, _ := table(realm)
	if _, e = tx.Exec(ctx, "UPDATE "+t+" SET reauth_until=now()+interval '5 minutes' WHERE token_hash=$1", authn.Digest(token)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// PasswordHash bounds KDF work for shared user creation. Callers still enforce
// their business permission inside the committing transaction.
func (a *AuthCore) PasswordHash(password string) (string, error) {
	if len(password) < 12 || len(password) > 256 {
		return "", ErrPasswordPolicy
	}
	release, e := a.acquire()
	if e != nil {
		return "", e
	}
	defer release()
	return authn.Hash(password)
}

// lockSession acquires users before session rows in all mutations. The input
// authority is the bearer token in a fixed realm, never an unverified user ID.
func lockSession(ctx context.Context, tx pgx.Tx, realm Realm, token string) (string, error) {
	t, e := table(realm)
	if e != nil {
		return "", e
	}
	if !validToken(token) {
		return "", ErrUnauthorized
	}
	var id string
	e = tx.QueryRow(ctx, "SELECT user_id FROM "+t+" WHERE token_hash=$1", authn.Digest(token)).Scan(&id)
	if e != nil {
		return "", noRows(e)
	}
	var active bool
	e = tx.QueryRow(ctx, "SELECT status='active' FROM users WHERE id=$1 FOR UPDATE", id).Scan(&active)
	if e != nil {
		return "", noRows(e)
	}
	if !active {
		return "", ErrUnauthorized
	}
	var ok bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+t+` s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.user_id=$2 AND s.expires_at>now() AND s.auth_epoch=u.auth_epoch)`, authn.Digest(token), id).Scan(&ok)
	if e != nil {
		return "", e
	}
	if !ok {
		return "", ErrUnauthorized
	}
	return id, nil
}
func revoke(ctx context.Context, tx pgx.Tx, id, action string) error {
	for _, t := range []string{"sessions", "drive_sessions"} {
		if _, e := tx.Exec(ctx, "DELETE FROM "+t+" WHERE user_id=$1", id); e != nil {
			return e
		}
	}
	if _, e := tx.Exec(ctx, "DELETE FROM identity_auth_attempts WHERE key_hash=(SELECT encode(sha256(convert_to(username,'UTF8')),'hex') FROM users WHERE id=$1)", id); e != nil {
		return e
	}
	_, e := tx.Exec(ctx, "INSERT INTO identity_auth_events(user_id,action) VALUES($1,$2)", id, action)
	return e
}

// RevokeAll invalidates both realms including the current session. Any in-flight
// request already authenticated needs its own commit-time resource authorization.
func (a *AuthCore) RevokeAll(ctx context.Context, realm Realm, token string) error {
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE users SET auth_epoch=auth_epoch+1,recovery_token_hash=NULL,recovery_expires_at=NULL,recovery_auth_epoch=NULL WHERE id=$1", id); e != nil {
		return e
	}
	if e = revoke(ctx, tx, id, "sessions.revoked"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (a *AuthCore) ChangePassword(ctx context.Context, realm Realm, token, current, next string) error {
	if len(next) < 12 || len(next) > 256 {
		return ErrPasswordPolicy
	}
	if len(current) > 256 {
		return ErrUnauthorized
	}
	release, e := a.acquire()
	if e != nil {
		return e
	}
	defer release()
	s, e := a.Authenticate(ctx, realm, token)
	if e != nil {
		return e
	}
	if e = a.attempt(ctx, s.Principal.Username); e != nil {
		return e
	}
	var hash string
	if e = a.db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", s.Principal.StorageID).Scan(&hash); e != nil {
		return noRows(e)
	}
	if !authn.Verify(current, hash) {
		return ErrUnauthorized
	}
	nextHash, e := authn.Hash(next)
	if e != nil {
		return e
	}
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	var fresh string
	if e = tx.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", id).Scan(&fresh); e != nil {
		return e
	}
	if fresh != hash {
		return ErrUnauthorized
	}
	if _, e = tx.Exec(ctx, "UPDATE users SET password_hash=$2,auth_epoch=auth_epoch+1 WHERE id=$1", id, nextHash); e != nil {
		return e
	}
	if e = revoke(ctx, tx, id, "password.changed"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// OwnSessions is an internal bounded primitive, not the paginated HTTP API.
func (a *AuthCore) OwnSessions(ctx context.Context, realm Realm, token string) ([]OwnSession, error) {
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `SELECT realm||':'||s.public_id::text,s.created_at,s.expires_at,(realm=$2 AND token_hash=$3) FROM (SELECT 'desktop' realm,s.* FROM sessions s UNION ALL SELECT 'files' realm,s.* FROM drive_sessions s) s JOIN users u ON u.id=s.user_id WHERE user_id=$1 AND s.auth_epoch=u.auth_epoch AND expires_at>now() ORDER BY s.created_at DESC,realm,s.public_id LIMIT 100`, id, string(realm), authn.Digest(token))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []OwnSession{}
	for rows.Next() {
		var v OwnSession
		if e = rows.Scan(&v.ID, &v.CreatedAt, &v.ExpiresAt, &v.Current); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (a *AuthCore) RevokeOwnSession(ctx context.Context, realm Realm, token, handle string) error {
	p := strings.Split(handle, ":")
	if len(p) != 2 || len(p[1]) != 36 {
		return ErrUnauthorized
	}
	t, e := table(Realm(p[0]))
	if e != nil {
		return ErrUnauthorized
	}
	// Use text comparison rather than letting malformed UUIDs reach SQL casts.
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, "DELETE FROM "+t+" WHERE public_id::text=$1 AND user_id=$2", p[1], id)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthorized
	}
	if _, e = tx.Exec(ctx, "INSERT INTO identity_auth_events(user_id,action) VALUES($1,'sessions.revoked')", id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// IssueRecoveryLocal/ConsumeRecoveryLocal are operator-only primitives. Never
// expose them through HTTP. The CLI must enforce OS-root and private file IO.
func (a *AuthCore) IssueRecoveryLocal(ctx context.Context, name string) (string, error) {
	raw := authn.Token()
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, "SELECT id FROM users WHERE username=$1 AND status='active' FOR UPDATE", normalize(name)).Scan(&id)
	if e != nil {
		return "", noRows(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE users SET recovery_token_hash=$2,recovery_expires_at=now()+interval '15 minutes',recovery_auth_epoch=auth_epoch WHERE id=$1", id, authn.Digest(raw)); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO identity_auth_events(user_id,action) VALUES($1,'recovery.issued')", id); e != nil {
		return "", e
	}
	if e = tx.Commit(ctx); e != nil {
		return "", e
	}
	return raw, nil
}
func (a *AuthCore) ConsumeRecoveryLocal(ctx context.Context, name, raw, next string) error {
	if !validToken(raw) {
		return ErrUnauthorized
	}
	if len(next) < 12 || len(next) > 256 {
		return ErrPasswordPolicy
	}
	release, e := a.acquire()
	if e != nil {
		return e
	}
	defer release()
	// Invalid tokens do not spend a password KDF and do not lock out the user.
	var exists bool
	e = a.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username=$1 AND status='active' AND recovery_token_hash=$2 AND recovery_expires_at>now() AND recovery_auth_epoch=auth_epoch)", normalize(name), authn.Digest(raw)).Scan(&exists)
	if e != nil {
		return e
	}
	if !exists {
		return ErrUnauthorized
	}
	hash, e := authn.Hash(next)
	if e != nil {
		return e
	}
	tx, e := a.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, "SELECT id FROM users WHERE username=$1 AND status='active' FOR UPDATE", normalize(name)).Scan(&id)
	if e != nil {
		return noRows(e)
	}
	tag, e := tx.Exec(ctx, "UPDATE users SET password_hash=$3,auth_epoch=auth_epoch+1,recovery_token_hash=NULL,recovery_expires_at=NULL,recovery_auth_epoch=NULL WHERE id=$1 AND recovery_token_hash=$2 AND recovery_expires_at>now() AND recovery_auth_epoch=auth_epoch", id, authn.Digest(raw), hash)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthorized
	}
	if e = revoke(ctx, tx, id, "recovery.consumed"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
