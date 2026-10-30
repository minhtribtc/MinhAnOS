package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minhanos.local/platform/internal/authn"
)

func resetPassword(s *Service, token string, u User, next string) (Result, error) {
	input := PasswordReset{next}
	raw, _ := json.Marshal(input)
	return s.ResetPassword(context.Background(), DesktopRealm, token, u.ID, fmt.Sprintf("\"%d\"", u.Version), authn.Token(), authn.Token(), input, raw)
}

func auditCount(t *testing.T, s *Service, action, target string) int {
	t.Helper()
	var n int
	if e := s.db.QueryRow(context.Background(), "SELECT count(*) FROM identity_admin_events WHERE action=$1 AND target_id IS NOT DISTINCT FROM nullif($2,'')", action, target).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestAdminPasswordResetRevokesBothRealmsAndAudits(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "resetme")
	_, desk, e := s.Auth.Login(ctx, DesktopRealm, "resetme", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, drive, e := s.Auth.Login(ctx, DriveRealm, "resetme", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = resetPassword(s, token, u, "short"); !errors.Is(e, ErrPasswordPolicy) {
		t.Fatal("weak password accepted", e)
	}
	stale := u
	stale.Version += 7
	_, e = resetPassword(s, token, stale, "replacement-password-1")
	code(t, e, "VERSION_CONFLICT")
	_, e = resetPassword(s, token, admin, "replacement-password-1")
	code(t, e, "IDENTITY_USE_OWN_PASSWORD")
	out, e := resetPassword(s, token, u, "replacement-password-1")
	if e != nil {
		t.Fatal(e)
	}
	if after := decodeUser(t, out); after.Version <= u.Version || strings.Contains(string(out.Body), "password") {
		t.Fatal("reset must bump version and never echo credentials", string(out.Body))
	}
	if _, e = s.Auth.Authenticate(ctx, DesktopRealm, desk); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("desktop session survived reset")
	}
	if _, e = s.Auth.Authenticate(ctx, DriveRealm, drive); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("files session survived reset")
	}
	if _, _, e = s.Auth.Login(ctx, DesktopRealm, "resetme", testPassword, ""); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("old password still works")
	}
	userToken := ""
	if _, userToken, e = s.Auth.Login(ctx, DesktopRealm, "resetme", "replacement-password-1", ""); e != nil {
		t.Fatal("new password rejected", e)
	}
	if auditCount(t, s, "user.password.reset", u.ID) != 1 {
		t.Fatal("reset not audited")
	}
	// A standard user cannot reset anyone, and an admin needs a fresh reauthentication.
	other := policyUser(t, s, token, "otherone")
	if _, e = resetPassword(s, userToken, other, "replacement-password-2"); e == nil {
		t.Fatal("standard user reset another password")
	}
	_, fresh, e := s.Auth.Login(ctx, DesktopRealm, " LEGACY ", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = resetPassword(s, fresh, other, "replacement-password-2")
	code(t, e, "REAUTH_REQUIRED")
}

func TestPasswordResetCannotTakeOverHostOperator(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	if _, e := s.db.Exec(ctx, "INSERT INTO identity_host_operators(user_id,state,origin) VALUES($1,'enabled','test') ON CONFLICT(user_id) DO UPDATE SET state='enabled'", strings.Repeat("a", 64)); e != nil {
		t.Fatal(e)
	}
	second := policyUser(t, s, token, "secondadmin")
	if _, e := policyChange(s, token, second, "system-admin"); e != nil {
		t.Fatal(e)
	}
	_, other, e := s.Auth.Login(ctx, DesktopRealm, "secondadmin", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Auth.Reauthenticate(ctx, DesktopRealm, other, testPassword); e != nil {
		t.Fatal(e)
	}
	_, e = resetPassword(s, other, admin, "replacement-password-1")
	code(t, e, "IDENTITY_HOST_OPERATOR_PROTECTED")
	loginTest(t, s.Auth, DesktopRealm, testPassword)
}

func TestPasswordResetHTTPRoute(t *testing.T) {
	s, token, _ := policyFixture(t)
	u := policyUser(t, s, token, "httpreset")
	session, e := s.Auth.Authenticate(context.Background(), DesktopRealm, token)
	if e != nil {
		t.Fatal(e)
	}
	send := func(body string, match string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://account.test/api/v1/identity/users/"+u.ID+"/password", bytes.NewReader([]byte(body)))
		r.AddCookie(&http.Cookie{Name: "minhanos_session", Value: token})
		r.Header.Set("Origin", "http://account.test")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", session.CSRF)
		r.Header.Set("Idempotency-Key", authn.Token())
		r.Header.Set("If-Match", match)
		w := httptest.NewRecorder()
		s.HTTPHandler(DesktopRealm).ServeHTTP(w, r)
		return w
	}
	if w := send(`{"newPassword":"replacement-password-1","role":"system-admin"}`, fmt.Sprintf("\"%d\"", u.Version)); w.Code != 400 {
		t.Fatal("unknown field accepted", w.Code)
	}
	if w := send(`{"newPassword":"replacement-password-1"}`, ""); w.Code != 428 {
		t.Fatal("missing precondition accepted", w.Code, w.Body.String())
	}
	w := send(`{"newPassword":"replacement-password-1"}`, fmt.Sprintf("\"%d\"", u.Version))
	if w.Code != 200 || w.Header().Get("ETag") == "" || strings.Contains(w.Body.String(), "replacement") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestSignInAuditBoundedOrderedAndFiltered(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "audited")
	for i := 0; i < 6; i++ {
		if _, _, e := s.Auth.Login(ctx, DesktopRealm, "audited", "wrong-password-000", ""); !errors.Is(e, ErrUnauthorized) {
			t.Fatal(e)
		}
	}
	if n := auditCount(t, s, "auth.login.failed", u.ID); n != 2 {
		t.Fatal("failed sign-ins must be sampled at attempts 1 and 5, got", n)
	}
	for i := 0; i < 3; i++ {
		_, _, _ = s.Auth.Login(ctx, DesktopRealm, fmt.Sprintf("nobody%d", i), "wrong-password-000", "")
	}
	if n := auditCount(t, s, "auth.login.failed", ""); n != 0 {
		t.Fatal("unknown accounts must not create audit rows", n)
	}
	var reason string
	if e := s.db.QueryRow(ctx, "SELECT details->>'reasonCode' FROM identity_admin_events WHERE action='auth.login.failed' AND target_id=$1 LIMIT 1", u.ID).Scan(&reason); e != nil || reason != "INVALID_CREDENTIALS" {
		t.Fatal("reason", reason, e)
	}
	if _, e := s.db.Exec(ctx, "DELETE FROM identity_auth_attempts"); e != nil {
		t.Fatal(e)
	}
	_, userToken, e := s.Auth.Login(ctx, DriveRealm, "audited", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if auditCount(t, s, "auth.login", u.ID) != 1 {
		t.Fatal("sign-in not audited")
	}
	if e = s.Auth.Logout(ctx, DriveRealm, userToken); e != nil {
		t.Fatal(e)
	}
	if auditCount(t, s, "auth.logout", u.ID) != 1 {
		t.Fatal("sign-out not audited")
	}
	// Newest first, filtered to events about the user, stable across cursor pages.
	var times []string
	seen := map[string]bool{}
	page := ""
	for i := 0; i < 10; i++ {
		out, e := s.Read(ctx, DesktopRealm, token, "audit-events", "", u.ID, page, 1)
		if e != nil {
			t.Fatal(e)
		}
		var p struct {
			Items []struct {
				ID, ActorID, TargetID, OccurredAt string
			} `json:"items"`
			Next *string `json:"nextCursor"`
		}
		if e = json.Unmarshal(out.Body, &p); e != nil {
			t.Fatal(e)
		}
		for _, item := range p.Items {
			if item.TargetID != u.ID && item.ActorID != u.ID || seen[item.ID] {
				t.Fatal("filter or paging broken", item)
			}
			seen[item.ID] = true
			times = append(times, item.OccurredAt)
		}
		if p.Next == nil {
			break
		}
		page = *p.Next
	}
	if len(seen) < 4 {
		t.Fatal("expected creation, two failures, login and logout", len(seen))
	}
	for i := 1; i < len(times); i++ {
		if times[i] > times[i-1] {
			t.Fatal("audit not newest first", times)
		}
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "audit-events", "", "not-a-uuid", "", 10); e == nil {
		t.Fatal("invalid audit filter accepted")
	}
	_ = admin
}

func TestGroupsFilteredByMember(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "member")
	g := groupTest(t, s, token, "with-member")
	groupTest(t, s, token, "without-member")
	if _, e := s.GroupWrite(ctx, DesktopRealm, token, g.ID, "add", u.ID, fmt.Sprintf("\"%d\"", g.Version), authn.Token(), authn.Token(), GroupInput{}, []byte(u.ID)); e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, DesktopRealm, token, "groups", "", u.ID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	items := pageItems(t, out)
	if len(items) != 1 || !strings.Contains(string(items[0]), g.ID) {
		t.Fatal("member filter", string(out.Body))
	}
	if _, e = securityHTTP(t, s, token, "GET", "groups?member="+u.ID+"&search=x", nil); e == nil {
		t.Fatal("conflicting filters accepted")
	}
	if _, e = securityHTTP(t, s, token, "GET", "users?member="+u.ID, nil); e == nil {
		t.Fatal("member filter accepted on users")
	}
}
