package identity

import (
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

func identityGET(s *Service, realm Realm, token, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://account.test/api/v1/identity/"+path, nil)
	name := "minhanos_session"
	if realm == DriveRealm {
		name = "minhanos_drive_session"
	}
	r.AddCookie(&http.Cookie{Name: name, Value: token})
	w := httptest.NewRecorder()
	s.HTTPHandler(realm).ServeHTTP(w, r)
	return w
}

func TestResetRequiresPasswordChangeUntilUserChangesIt(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "mustchange")
	if _, e := resetPassword(s, token, u, "temporary-password-1"); e != nil {
		t.Fatal(e)
	}
	session, userToken, e := s.Auth.Login(ctx, DesktopRealm, "mustchange", "temporary-password-1", "")
	if e != nil || !session.Principal.PasswordChangeRequired {
		t.Fatal("login must report the required change", e)
	}
	if w := identityGET(s, DesktopRealm, userToken, "users"); w.Code != 403 || !strings.Contains(w.Body.String(), "PASSWORD_CHANGE_REQUIRED") {
		t.Fatal("other APIs must be closed", w.Code, w.Body.String())
	}
	w := identityGET(s, DesktopRealm, userToken, "me")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"passwordChangeRequired":true`) {
		t.Fatal("me must stay readable and show the flag", w.Code, w.Body.String())
	}
	if _, e = s.Password(ctx, DesktopRealm, userToken, authn.Token(), authn.Token(), PasswordChange{"temporary-password-1", "own-new-password-1"}, []byte("x")); e != nil {
		t.Fatal(e)
	}
	session, _, e = s.Auth.Login(ctx, DesktopRealm, "mustchange", "own-new-password-1", "")
	if e != nil || session.Principal.PasswordChangeRequired {
		t.Fatal("own change must clear the flag", e)
	}
	// An admin may also reset without forcing a change.
	fresh, e := (Repository{s.db}).PublicUser(ctx, mustStorage(t, s, u.ID))
	if e != nil {
		t.Fatal(e)
	}
	keep := false
	input := PasswordReset{"temporary-password-2", &keep}
	raw, _ := json.Marshal(input)
	if _, e = s.ResetPassword(ctx, DesktopRealm, token, u.ID, fmt.Sprintf("\"%d\"", fresh.Version), authn.Token(), authn.Token(), input, raw); e != nil {
		t.Fatal(e)
	}
	if session, _, e = s.Auth.Login(ctx, DesktopRealm, "mustchange", "temporary-password-2", ""); e != nil || session.Principal.PasswordChangeRequired {
		t.Fatal("requireChange=false must not force a change", e)
	}
}

func TestAdminListsAndRevokesAnotherUsersSessions(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "devices")
	_, desk, e := s.Auth.Login(ctx, DesktopRealm, "devices", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Auth.Login(ctx, DriveRealm, "devices", testPassword, ""); e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, DesktopRealm, token, "user/sessions", u.ID, "", "", 50)
	if e != nil {
		t.Fatal(e)
	}
	items := pageItems(t, out)
	if len(items) != 2 || strings.Contains(string(out.Body), `"current":true`) || strings.Contains(string(out.Body), "token") {
		t.Fatal("expected two non-current sessions without secrets", string(out.Body))
	}
	if w := identityGET(s, DesktopRealm, desk, "users/"+admin.ID+"/sessions"); w.Code != 403 {
		t.Fatal("standard user listed an admin's sessions", w.Code)
	}
	_, e = s.RevokeUserSessions(ctx, DesktopRealm, token, admin.ID, authn.Token(), authn.Token(), nil)
	code(t, e, "IDENTITY_USE_OWN_SESSIONS")
	if _, e = s.RevokeUserSessions(ctx, DesktopRealm, token, u.ID, authn.Token(), authn.Token(), nil); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Auth.Authenticate(ctx, DesktopRealm, desk); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("session survived admin revoke")
	}
	if out, e = s.Read(ctx, DesktopRealm, token, "user/sessions", u.ID, "", "", 50); e != nil || len(pageItems(t, out)) != 0 {
		t.Fatal("sessions remain", e)
	}
	if auditCount(t, s, "user.sessions.revoked", u.ID) != 1 || outboxCount(t, s, "sessions.revoked", u.ID) != 1 {
		t.Fatal("revoke not audited or notified")
	}
	// The HTTP route needs a fresh reauthentication like other administrative writes.
	_, fresh, e := s.Auth.Login(ctx, DesktopRealm, " LEGACY ", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.RevokeUserSessions(ctx, DesktopRealm, fresh, u.ID, authn.Token(), authn.Token(), nil)
	code(t, e, "REAUTH_REQUIRED")
}
