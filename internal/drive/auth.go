package drive

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
	"minhanos.local/platform/internal/identity"
)

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}
type Session struct {
	User User   `json:"user"`
	CSRF string `json:"csrfToken"`
}

// IdentityStore is the boundary for replacing shared PostgreSQL identities with OIDC later.
type IdentityStore interface {
	Session(context.Context, string) (Session, error)
	Login(context.Context, string, string) (Session, string, error)
	Logout(context.Context, string) error
	Audit(context.Context, User, string, string) error
}

var ErrUnauthorized = identity.ErrUnauthorized

type SQLIdentity struct {
	db   *pgxpool.Pool
	core *identity.AuthCore
}

func NewSQLIdentity(ctx context.Context, db *pgxpool.Pool, bootstrap string) (*SQLIdentity, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(681021)"); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS users(id text PRIMARY KEY,username text UNIQUE NOT NULL,display_name text NOT NULL,role text NOT NULL CHECK(role IN ('admin','user')),password_hash text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS drive_sessions(token_hash text PRIMARY KEY,user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,csrf_token text NOT NULL,expires_at timestamptz NOT NULL);
 CREATE INDEX IF NOT EXISTS drive_sessions_expiry ON drive_sessions(expires_at);
 CREATE TABLE IF NOT EXISTS drive_audit(id bigserial PRIMARY KEY,user_id text NOT NULL,actor text NOT NULL,action text NOT NULL,target text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());`)
	if err != nil {
		return nil, err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		p, e := os.ReadFile(bootstrap)
		if e != nil {
			return nil, e
		}
		password := strings.TrimSpace(string(p))
		if len(password) < 12 {
			return nil, errors.New("bootstrap password must have at least 12 characters")
		}
		h, e := authn.Hash(password)
		if e != nil {
			return nil, e
		}
		_, err = tx.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,'admin','Minh An','admin',$2) ON CONFLICT(username) DO NOTHING", authn.Token(), h)
		if err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, "DELETE FROM drive_sessions WHERE expires_at<now()"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err = identity.ApplySchema(ctx, db); err != nil {
		return nil, err
	}
	core, err := identity.NewAuthCore(db)
	if err != nil {
		return nil, err
	}
	return &SQLIdentity{db: db, core: core}, nil
}
func driveSession(v identity.AuthSession) Session {
	return Session{User: User{ID: v.Principal.StorageID, Username: v.Principal.Username, DisplayName: v.Principal.DisplayName, Role: v.Principal.Role}, CSRF: v.CSRF}
}
func (s *SQLIdentity) Session(ctx context.Context, token string) (Session, error) {
	v, e := s.core.Authenticate(ctx, identity.DriveRealm, token)
	return driveSession(v), e
}

// PlatformSession verifies the real cookie in its own realm; a header is not authority.
func (s *SQLIdentity) PlatformSession(ctx context.Context, token string) (Session, error) {
	v, e := s.core.Authenticate(ctx, identity.DesktopRealm, token)
	return driveSession(v), e
}
func (s *SQLIdentity) Login(ctx context.Context, name, password string) (Session, string, error) {
	return s.LoginRotating(ctx, name, password, "")
}
func (s *SQLIdentity) LoginRotating(ctx context.Context, name, password, old string) (Session, string, error) {
	v, token, e := s.core.Login(ctx, identity.DriveRealm, name, password, old)
	return driveSession(v), token, e
}
func (s *SQLIdentity) Logout(ctx context.Context, token string) error {
	return s.core.Logout(ctx, identity.DriveRealm, token)
}
func (s *SQLIdentity) Audit(ctx context.Context, u User, action, target string) error {
	_, err := s.db.Exec(ctx, "INSERT INTO drive_audit(user_id,actor,action,target) VALUES($1,$2,$3,$4)", u.ID, u.Username, action, target)
	return err
}

type loginAttempt struct {
	count int
	start time.Time
}
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.attempts == nil {
		l.attempts = map[string]loginAttempt{}
	}
	now := time.Now()
	for k, v := range l.attempts {
		if now.Sub(v.start) > 10*time.Minute {
			delete(l.attempts, k)
		}
	}
	if _, ok := l.attempts[ip]; !ok && len(l.attempts) >= 4096 {
		return false
	}
	v := l.attempts[ip]
	if v.start.IsZero() {
		v.start = now
	}
	v.count++
	l.attempts[ip] = v
	return v.count <= 20
}
func validCSRF(r *http.Request, s Session) bool {
	return s.CSRF != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.CSRF)) == 1
}
