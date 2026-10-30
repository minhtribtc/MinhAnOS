package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minhanos.local/platform/internal/requestid"
)

func outboxCount(t *testing.T, s *Service, kind, recipient string) int {
	t.Helper()
	var n int
	if e := s.db.QueryRow(context.Background(), "SELECT count(*) FROM identity_notification_outbox WHERE kind=$1 AND recipient_id::text=$2", kind, recipient).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestSecurityLogAppendOnlyRetentionAndRetiredLedger(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	if _, e := s.db.Exec(ctx, "UPDATE identity_admin_events SET action='tampered'"); e == nil {
		t.Fatal("audit rows must not be updated")
	}
	if _, e := s.db.Exec(ctx, "DELETE FROM identity_admin_events"); e == nil {
		t.Fatal("audit rows must not be deleted")
	}
	if _, e := s.db.Exec(ctx, "INSERT INTO identity_auth_events(user_id,action) VALUES($1,'password.changed')", strings.Repeat("a", 64)); e == nil {
		t.Fatal("retired ledger accepted a write")
	}
	if _, e := s.db.Exec(ctx, "INSERT INTO identity_admin_events(action,outcome,request_id,occurred_at) VALUES('old.event','success','r1',now()-interval '400 days'),('kept.event','success','r2',now()-interval '300 days')"); e != nil {
		t.Fatal(e)
	}
	policyUser(t, s, token, "retention") // any policy mutation runs the bounded retention batch
	var old, kept int
	s.db.QueryRow(ctx, "SELECT count(*) FILTER (WHERE action='old.event'),count(*) FILTER (WHERE action='kept.event') FROM identity_admin_events").Scan(&old, &kept)
	if old != 0 || kept != 1 {
		t.Fatal("retention", old, kept)
	}
	// Outside the retention routine even expired rows stay.
	s.db.Exec(ctx, "INSERT INTO identity_admin_events(action,outcome,request_id,occurred_at) VALUES('old.event','success','r3',now()-interval '400 days')")
	if _, e := s.db.Exec(ctx, "DELETE FROM identity_admin_events WHERE action='old.event'"); e == nil {
		t.Fatal("delete without retention flag accepted")
	}
}

func TestSignInUsesRequestAndCorrelationIDs(t *testing.T) {
	s, _, admin := policyFixture(t)
	var ids requestid.IDs
	h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = requestid.From(r.Context())
		if _, _, e := s.Auth.Login(r.Context(), DesktopRealm, "legacy", testPassword, ""); e != nil {
			t.Fatal(e)
		}
	}))
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.Header.Set("X-Correlation-ID", "sign-in-flow-01")
	h.ServeHTTP(httptest.NewRecorder(), r)
	var request, correlation string
	if e := s.db.QueryRow(context.Background(), "SELECT request_id,correlation_id FROM identity_admin_events WHERE action='auth.login' AND target_id=$1 ORDER BY occurred_at DESC LIMIT 1", admin.ID).Scan(&request, &correlation); e != nil {
		t.Fatal(e)
	}
	if request != ids.Request || correlation != "sign-in-flow-01" {
		t.Fatal("IDs not recorded", request, correlation, ids)
	}
}

func TestSecurityNoticesQueuedWithTheChange(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "notified")
	if _, e := resetPassword(s, token, u, "replacement-password-1"); e != nil {
		t.Fatal(e)
	}
	if outboxCount(t, s, "password.reset", u.ID) != 1 {
		t.Fatal("reset notice missing")
	}
	fresh, e := (Repository{s.db}).PublicUser(ctx, mustStorage(t, s, u.ID))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = policyChange(s, token, fresh, "user-admin"); e != nil {
		t.Fatal(e)
	}
	if outboxCount(t, s, "role.changed", u.ID) != 1 {
		t.Fatal("role notice missing")
	}
	for i := 0; i < 10; i++ {
		if _, _, e = s.Auth.Login(ctx, DesktopRealm, "notified", "wrong-password-000", ""); !errors.Is(e, ErrUnauthorized) {
			t.Fatal(e)
		}
	}
	if outboxCount(t, s, "login.failures", u.ID) != 1 {
		t.Fatal("repeated failure notice missing")
	}
	// A rolled-back change leaves neither audit nor notice behind.
	stale := fresh
	stale.Version += 9
	if _, e = resetPassword(s, token, stale, "replacement-password-2"); e == nil {
		t.Fatal("stale reset accepted")
	}
	if outboxCount(t, s, "password.reset", u.ID) != 1 {
		t.Fatal("rolled-back reset queued a notice")
	}
	var leaked int
	s.db.QueryRow(ctx, "SELECT count(*) FROM identity_notification_outbox WHERE details::text ~* 'password|token|hash'").Scan(&leaked)
	if leaked != 0 {
		t.Fatal("notice details must not carry secrets")
	}
}
