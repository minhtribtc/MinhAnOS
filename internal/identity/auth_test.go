package identity

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
)

const testPassword = "original-password-123"

func authFixture(t *testing.T) (*AuthCore, *pgxpool.Pool) {
	t.Helper()
	db := fixture(t)
	ctx := context.Background()
	hash, e := authn.Hash(testPassword)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "UPDATE users SET password_hash=$1", hash); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	a, e := NewAuthCore(db)
	if e != nil {
		t.Fatal(e)
	}
	return a, db
}
func loginTest(t *testing.T, a *AuthCore, realm Realm, password string) string {
	t.Helper()
	_, token, e := a.Login(context.Background(), realm, " LEGACY ", password, "")
	if e != nil {
		t.Fatal(e)
	}
	return token
}
func denied(t *testing.T, a *AuthCore, realm Realm, token string) {
	t.Helper()
	if _, e := a.Authenticate(context.Background(), realm, token); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("session not denied", e)
	}
}
func TestSharedAuthLegacyRotationAndOwnership(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	old := loginTest(t, a, DesktopRealm, testPassword)
	_, fresh, e := a.Login(ctx, DesktopRealm, "legacy", testPassword, old)
	if e != nil || fresh == old {
		t.Fatal("rotation", e)
	}
	denied(t, a, DesktopRealm, old)
	denied(t, a, DriveRealm, fresh)
	drive := loginTest(t, a, DriveRealm, testPassword)
	_, driveNew, e := a.Login(ctx, DriveRealm, "legacy", testPassword, drive)
	if e != nil {
		t.Fatal(e)
	}
	denied(t, a, DriveRealm, drive)
	sessions, e := a.OwnSessions(ctx, DesktopRealm, fresh)
	if e != nil || len(sessions) != 2 {
		t.Fatal(sessions, e)
	}
	var driveHandle string
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, "files:") {
			driveHandle = s.ID
		}
		if strings.Contains(s.ID, fresh) || strings.Contains(s.ID, authn.Digest(fresh)) {
			t.Fatal("secret in handle")
		}
	}
	hash, _ := authn.Hash(testPassword)
	_, e = db.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,'other','Other','user',$2)", strings.Repeat("f", 64), hash)
	if e != nil {
		t.Fatal(e)
	}
	_, other, e := a.Login(ctx, DriveRealm, "other", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.RevokeOwnSession(ctx, DriveRealm, other, driveHandle); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("cross-user revoke", e)
	}
	if e = a.RevokeOwnSession(ctx, DesktopRealm, fresh, driveHandle); e != nil {
		t.Fatal(e)
	}
	denied(t, a, DriveRealm, driveNew)
	if e = a.RevokeOwnSession(ctx, DesktopRealm, fresh, "sessions;DROP TABLE users:00000000-0000-0000-0000-000000000000"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
}
func TestSharedAuthPasswordAndAllRealms(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	desktop := loginTest(t, a, DesktopRealm, testPassword)
	drive := loginTest(t, a, DriveRealm, testPassword)
	if e := a.ChangePassword(ctx, DesktopRealm, desktop, "wrong", "replacement-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
	if e := a.ChangePassword(ctx, DesktopRealm, desktop, testPassword, "short"); !errors.Is(e, ErrPasswordPolicy) {
		t.Fatal(e)
	}
	if e := a.ChangePassword(ctx, DesktopRealm, desktop, testPassword, "replacement-password-123"); e != nil {
		t.Fatal(e)
	}
	denied(t, a, DesktopRealm, desktop)
	denied(t, a, DriveRealm, drive)
	if e := a.ChangePassword(ctx, DesktopRealm, desktop, testPassword, "replayed-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("password replay", e)
	}
	if _, _, e := a.Login(ctx, DriveRealm, "legacy", testPassword, ""); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("old password", e)
	}
	fresh := loginTest(t, a, DriveRealm, "replacement-password-123")
	if e := a.RevokeAll(ctx, DriveRealm, fresh); e != nil {
		t.Fatal(e)
	}
	denied(t, a, DriveRealm, fresh)
	var count int
	db.QueryRow(ctx, "SELECT count(*) FROM identity_auth_events WHERE action='password.changed'").Scan(&count)
	if count != 1 {
		t.Fatal("audit", count)
	}
}
func TestSharedAuthStatusEpochAndLegacySessions(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	raw := authn.Token()
	_, e := db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", authn.Digest(raw), strings.Repeat("a", 64), authn.Token())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.Authenticate(ctx, DesktopRealm, raw); e != nil {
		t.Fatal("legacy cookie lost", e)
	}
	_, e = db.Exec(ctx, "UPDATE users SET status='disabled'")
	if e != nil {
		t.Fatal(e)
	}
	denied(t, a, DesktopRealm, raw)
	if _, _, e = a.Login(ctx, DriveRealm, "legacy", testPassword, ""); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("disabled login", e)
	}
	if _, e = a.IssueRecoveryLocal(ctx, "legacy"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("recovery enabled disabled user", e)
	}
	db.Exec(ctx, "UPDATE users SET status='active'")
	denied(t, a, DesktopRealm, raw)
	fresh := loginTest(t, a, DesktopRealm, testPassword)
	hash, _ := authn.Hash("direct-writer-password-123")
	if _, e = db.Exec(ctx, "UPDATE users SET password_hash=$1", hash); e != nil {
		t.Fatal(e)
	}
	denied(t, a, DesktopRealm, fresh)
	if _, e = db.Exec(ctx, "UPDATE users SET auth_epoch=1"); e == nil {
		t.Fatal("epoch decreased")
	}
}
func TestLocalRecoveryExpiryReplayAndConcurrentConsume(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	desktop := loginTest(t, a, DesktopRealm, testPassword)
	drive := loginTest(t, a, DriveRealm, testPassword)
	raw, e := a.IssueRecoveryLocal(ctx, "legacy")
	if e != nil {
		t.Fatal(e)
	}
	// Issuance alone does not change credentials or revoke sessions.
	if _, e = a.Authenticate(ctx, DesktopRealm, desktop); e != nil {
		t.Fatal(e)
	}
	var stored string
	db.QueryRow(ctx, "SELECT recovery_token_hash FROM users").Scan(&stored)
	if stored == raw || stored != authn.Digest(raw) {
		t.Fatal("recovery not hashed")
	}
	db.Exec(ctx, "UPDATE users SET recovery_expires_at=now()-interval '1 second'")
	if e = a.ConsumeRecoveryLocal(ctx, "legacy", raw, "recovered-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("expired token", e)
	}
	raw, e = a.IssueRecoveryLocal(ctx, "legacy")
	if e != nil {
		t.Fatal(e)
	}
	previous := raw
	raw, e = a.IssueRecoveryLocal(ctx, "legacy")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.ConsumeRecoveryLocal(ctx, "legacy", previous, "recovered-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("superseded token", e)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- a.ConsumeRecoveryLocal(ctx, "legacy", raw, "recovered-password-123")
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else if !errors.Is(e, ErrUnauthorized) {
			t.Fatal(e)
		}
	}
	if successes != 1 {
		t.Fatal("single use", successes)
	}
	denied(t, a, DesktopRealm, desktop)
	denied(t, a, DriveRealm, drive)
	if e = a.ConsumeRecoveryLocal(ctx, "legacy", raw, "recovered-again-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("recovery replay", e)
	}
	loginTest(t, a, DriveRealm, "recovered-password-123")
}
func TestAuthThrottleSharedBoundedAndExpiry(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	second, e := NewAuthCore(db)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 20; i++ {
		core := a
		if i%2 == 1 {
			core = second
		}
		if e = core.attempt(ctx, "unknown-user"); e != nil {
			t.Fatal(e)
		}
	}
	if e = second.attempt(ctx, " UNKNOWN-USER "); !errors.Is(e, ErrThrottled) {
		t.Fatal("cross-instance throttle", e)
	}
	db.Exec(ctx, "UPDATE identity_auth_attempts SET expires_at=now()-interval '1 second'")
	if e = a.attempt(ctx, "unknown-user"); e != nil {
		t.Fatal("window did not expire", e)
	}
	release, e := a.acquire()
	if e != nil {
		t.Fatal(e)
	}
	release2, e := a.acquire()
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = a.Login(ctx, DesktopRealm, "legacy", testPassword, ""); !errors.Is(e, ErrBusy) {
		t.Fatal("KDF concurrency unbounded", e)
	}
	release()
	release2()
	if _, _, e = a.Login(ctx, DesktopRealm, "missing-user", testPassword, ""); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("unknown user", e)
	}
	if _, _, e = a.Login(ctx, DesktopRealm, "legacy", "incorrect-password", ""); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("wrong password", e)
	}
}

func TestLoginCannotRacePastEpochRevocation(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM users FOR UPDATE").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() { _, _, e := a.Login(ctx, DesktopRealm, "legacy", testPassword, ""); result <- e }()
	blocked := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&blocked)
		if e != nil {
			t.Fatal(e)
		}
		if blocked {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("login did not reach locked issuance")
	}
	if _, e = tx.Exec(ctx, "UPDATE users SET auth_epoch=auth_epoch+1"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-result; !errors.Is(e, ErrUnauthorized) {
		t.Fatal("stale verified password issued session", e)
	}
}

func TestCredentialChangeRollsBackIfAuditFails(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	token := loginTest(t, a, DesktopRealm, testPassword)
	if _, e := db.Exec(ctx, "DROP TABLE identity_auth_events"); e != nil {
		t.Fatal(e)
	}
	if e := a.ChangePassword(ctx, DesktopRealm, token, testPassword, "replacement-password-123"); e == nil {
		t.Fatal("mutation ignored audit failure")
	}
	if _, e := a.Authenticate(ctx, DesktopRealm, token); e != nil {
		t.Fatal("rollback lost old session", e)
	}
	loginTest(t, a, DriveRealm, testPassword)
}

func TestThrottleCapacityAndPasswordInvalidatesRecovery(t *testing.T) {
	a, db := authFixture(t)
	ctx := context.Background()
	if _, e := db.Exec(ctx, `INSERT INTO identity_auth_attempts SELECT lpad(to_hex(i),64,'0'),1,now()+interval '10 minutes' FROM generate_series(1,10000) i`); e != nil {
		t.Fatal(e)
	}
	if e := a.attempt(ctx, "new-key"); !errors.Is(e, ErrThrottled) {
		t.Fatal("unbounded throttle table", e)
	}
	db.Exec(ctx, "DELETE FROM identity_auth_attempts")
	token := loginTest(t, a, DesktopRealm, testPassword)
	recovery, e := a.IssueRecoveryLocal(ctx, "legacy")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.ChangePassword(ctx, DesktopRealm, token, testPassword, "replacement-password-123"); e != nil {
		t.Fatal(e)
	}
	if e = a.ConsumeRecoveryLocal(ctx, "legacy", recovery, "attacker-password-123"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("stale recovery after change", e)
	}
}
