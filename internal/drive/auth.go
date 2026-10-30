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

	"github.com/jackc/pgx/v5"
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

var ErrUnauthorized = errors.New("unauthorized")

type SQLIdentity struct {
	db    *pgxpool.Pool
	dummy string
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
	dummy, err := authn.Hash(authn.Token())
	if err != nil {
		return nil, err
	}
	return &SQLIdentity{db: db, dummy: dummy}, nil
}
func (s *SQLIdentity) Session(ctx context.Context, token string) (Session, error) {
	var v Session
	if len(token) != 64 {
		return v, ErrUnauthorized
	}
	err := s.db.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.role,s.csrf_token FROM drive_sessions s JOIN users u ON u.id=s.user_id WHERE token_hash=$1 AND expires_at>now()`, authn.Digest(token)).Scan(&v.User.ID, &v.User.Username, &v.User.DisplayName, &v.User.Role, &v.CSRF)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrUnauthorized
	}
	return v, err
}

// PlatformSession is optional and reads an authenticated session rather than browser-supplied identity claims.
func (s *SQLIdentity) PlatformSession(ctx context.Context, token string) (Session, error) {
	var v Session
	if len(token) != 64 {
		return v, ErrUnauthorized
	}
	err := s.db.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.role,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id WHERE token_hash=$1 AND expires_at>now()`, authn.Digest(token)).Scan(&v.User.ID, &v.User.Username, &v.User.DisplayName, &v.User.Role, &v.CSRF)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrUnauthorized
	}
	return v, err
}
func (s *SQLIdentity) Login(ctx context.Context, name, password string) (Session, string, error) {
	var v Session
	hash := s.dummy
	err := s.db.QueryRow(ctx, "SELECT id,username,display_name,role,password_hash FROM users WHERE username=$1", strings.ToLower(strings.TrimSpace(name))).Scan(&v.User.ID, &v.User.Username, &v.User.DisplayName, &v.User.Role, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, "", err
	}
	valid := authn.Verify(password, hash)
	if err != nil || !valid {
		return Session{}, "", ErrUnauthorized
	}
	token := authn.Token()
	v.CSRF = authn.Token()
	_, err = s.db.Exec(ctx, "INSERT INTO drive_sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)", authn.Digest(token), v.User.ID, v.CSRF, time.Now().Add(24*time.Hour))
	return v, token, err
}
func (s *SQLIdentity) Logout(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM drive_sessions WHERE token_hash=$1", authn.Digest(token))
	return err
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
