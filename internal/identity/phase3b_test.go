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

func groupTest(t *testing.T, s *Service, token, name string) Group {
	t.Helper()
	out, e := securityHTTP(t, s, token, "POST", "groups", GroupInput{Key: name, Name: name})
	if e != nil {
		t.Fatal(e)
	}
	var g Group
	if e = json.Unmarshal(out.Body, &g); e != nil {
		t.Fatal(e)
	}
	return g
}
func pageItems(t *testing.T, out Result) []json.RawMessage {
	t.Helper()
	var p struct {
		Items []json.RawMessage `json:"items"`
	}
	if e := json.Unmarshal(out.Body, &p); e != nil {
		t.Fatal(e)
	}
	return p.Items
}
func TestSessionPagesOwnerRevocationAndReplay(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	drive := loginTest(t, s.Auth, DriveRealm, testPassword)
	otherDesktop := loginTest(t, s.Auth, DesktopRealm, testPassword)
	out, e := securityHTTP(t, s, token, "GET", "me/sessions", nil)
	if e != nil {
		t.Fatal(e)
	}
	items := pageItems(t, out)
	if len(items) != 3 {
		t.Fatal("missing realms", len(items))
	}
	var driveHandle string
	for _, raw := range items {
		var v struct {
			ID     string `json:"id"`
			UserID string `json:"userId"`
		}
		json.Unmarshal(raw, &v)
		if strings.HasPrefix(v.ID, "files:") {
			driveHandle = v.ID
		}
		if len(v.UserID) != 36 || strings.Contains(string(raw), "token_hash") || strings.Contains(string(raw), authn.Digest(token)) {
			t.Fatal("session secret/storage metadata exposed")
		}
	}
	u := policyUser(t, s, token, "outsider")
	_, outsider, e := s.Auth.Login(ctx, DesktopRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.RevokeSession(ctx, DesktopRealm, outsider, driveHandle, "", authn.Token(), false, nil)
	code(t, e, "IDENTITY_NOT_FOUND")
	key := authn.Token()
	if _, e = s.RevokeSession(ctx, DesktopRealm, token, "", key, authn.Token(), true, nil); e != nil {
		t.Fatal(e)
	}
	denied(t, s.Auth, DriveRealm, drive)
	denied(t, s.Auth, DesktopRealm, otherDesktop)
	if _, e = s.Auth.Authenticate(ctx, DesktopRealm, token); e != nil {
		t.Fatal("current session revoked", e)
	}
	if _, e = s.RevokeSession(ctx, DesktopRealm, token, "", key, authn.Token(), true, nil); e != nil {
		t.Fatal("replay", e)
	}
	if _, e = s.Auth.Authenticate(ctx, DesktopRealm, outsider); e != nil {
		t.Fatal("other user revoked", e)
	}
}
func TestPasswordAdapterTransactionalAndCredentialMetadata(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	drive := loginTest(t, s.Auth, DriveRealm, testPassword)
	out, e := s.Read(ctx, DesktopRealm, token, "me/credentials", "", "", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(pageItems(t, out)) != 1 || strings.Contains(string(out.Body), "password_hash") {
		t.Fatal("credential metadata")
	}
	input := PasswordChange{"incorrect-password", "replacement-password-123"}
	raw, _ := json.Marshal(input)
	_, e = s.Password(ctx, DesktopRealm, token, authn.Token(), authn.Token(), input, raw)
	code(t, e, "INVALID_CREDENTIALS")
	input.CurrentPassword = testPassword
	raw, _ = json.Marshal(input)
	if _, e = securityHTTP(t, s, token, "POST", "me/password", input); e != nil {
		t.Fatal(e)
	}
	denied(t, s.Auth, DesktopRealm, token)
	denied(t, s.Auth, DriveRealm, drive)
	_, e = s.Password(ctx, DesktopRealm, token, authn.Token(), authn.Token(), input, raw)
	if !errors.Is(e, ErrUnauthorized) {
		t.Fatal("stale session password replay", e)
	}
	loginTest(t, s.Auth, DriveRealm, input.NewPassword)
	var serialized string
	s.db.QueryRow(ctx, "SELECT coalesce(string_agg(response::text||request_hash,''),'') FROM identity_mutations").Scan(&serialized)
	if strings.Contains(serialized, input.NewPassword) || strings.Contains(serialized, testPassword) {
		t.Fatal("credential material in replay cache")
	}
}
func TestGroupCustomRoleBindingAndRevocation(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	g := groupTest(t, s, token, "auditors")
	u := policyUser(t, s, token, "audit_user")
	input := RoleInput{"audit-reader", "Audit reader", []string{"identity.audit.read", "identity.roles.read"}}
	raw, _ := json.Marshal(input)
	out, e := s.CreateRole(ctx, DesktopRealm, token, authn.Token(), authn.Token(), input, raw)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		ID string `json:"id"`
	}
	json.Unmarshal(out.Body, &r)
	var b BindingInput
	b.RoleID = r.ID
	b.Subject.Kind = "group"
	b.Subject.ID = g.ID
	b.Scope.Kind = "system"
	raw, _ = json.Marshal(b)
	out, e = s.BindRole(ctx, DesktopRealm, token, "", "", authn.Token(), authn.Token(), b, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	var binding struct {
		ID string `json:"id"`
	}
	json.Unmarshal(out.Body, &binding)
	_, e = s.GroupWrite(ctx, DesktopRealm, token, g.ID, "add", u.ID, "\"99\"", authn.Token(), authn.Token(), GroupInput{}, []byte(u.ID))
	code(t, e, "VERSION_CONFLICT")
	out, e = s.GroupWrite(ctx, DesktopRealm, token, g.ID, "add", u.ID, fmt.Sprintf("\"%d\"", g.Version), authn.Token(), authn.Token(), GroupInput{}, []byte(u.ID))
	if e != nil {
		t.Fatal(e)
	}
	_, userToken, e := s.Auth.Login(ctx, DesktopRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Read(ctx, DesktopRealm, userToken, "audit-events", "", "", "", 10); e != nil {
		t.Fatal("custom group role not effective", e)
	}
	access, e := securityHTTP(t, s, token, "GET", "users/"+u.ID+"/access", nil)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(access.Body), "identity.audit.read") || strings.Contains(string(access.Body), "\"hostOperator\":true") {
		t.Fatal("effective target access", string(access.Body))
	}
	_, e = s.Read(ctx, DesktopRealm, userToken, "users", "", "", "", 10)
	code(t, e, "IDENTITY_PERMISSION_DENIED")
	if out, e = s.GroupWrite(ctx, DesktopRealm, token, g.ID, "remove", u.ID, out.ETag, authn.Token(), authn.Token(), GroupInput{}, []byte(u.ID)); e != nil {
		t.Fatal(e)
	}
	_, e = s.Read(ctx, DesktopRealm, userToken, "audit-events", "", "", "", 10)
	code(t, e, "IDENTITY_PERMISSION_DENIED")
	if _, e = s.BindRole(ctx, DesktopRealm, token, binding.ID, "\"1\"", authn.Token(), authn.Token(), BindingInput{}, nil, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GroupWrite(ctx, DesktopRealm, token, g.ID, "delete", "", out.ETag, authn.Token(), authn.Token(), GroupInput{}, nil); e != nil {
		t.Fatal(e)
	}
	var audited int
	if e = s.db.QueryRow(ctx, "SELECT count(*) FROM identity_admin_events WHERE action='group.deleted' AND target_id=$1", g.ID).Scan(&audited); e != nil || audited != 1 {
		t.Fatal("group deletion audit", audited, e)
	}
}
func TestProtectedPolicyCeilingsAndAuditRollback(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "delegate_policy")
	out, e := policyChange(s, token, u, "user-admin")
	if e != nil {
		t.Fatal(e)
	}
	u = decodeUser(t, out)
	_, dt, e := s.Auth.Login(ctx, DesktopRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	s.Auth.Reauthenticate(ctx, DesktopRealm, dt, testPassword)
	_, e = s.GroupWrite(ctx, DesktopRealm, dt, "", "create", "", "", authn.Token(), authn.Token(), GroupInput{"forged", "Forged"}, nil)
	code(t, e, "IDENTITY_PERMISSION_DENIED")
	_, e = s.CreateRole(ctx, DesktopRealm, token, authn.Token(), authn.Token(), RoleInput{"ceiling", "Ceiling", []string{"identity.roles.write"}}, nil)
	code(t, e, "IDENTITY_PERMISSION_CEILING")
	var sys string
	s.db.QueryRow(ctx, "SELECT id::text FROM identity_roles WHERE key='system-admin'").Scan(&sys)
	var b BindingInput
	b.RoleID = sys
	b.Subject.Kind = "user"
	b.Subject.ID = u.ID
	b.Scope.Kind = "system"
	_, e = s.BindRole(ctx, DesktopRealm, token, "", "", authn.Token(), authn.Token(), b, nil, false)
	code(t, e, "IDENTITY_PROTECTED_POLICY")
	b.Scope.Kind = "user"
	_, e = s.BindRole(ctx, DesktopRealm, token, "", "", authn.Token(), authn.Token(), b, nil, false)
	code(t, e, "IDENTITY_UNSUPPORTED_SCOPE")
	if _, e = s.db.Exec(ctx, "DROP TABLE identity_admin_events"); e != nil {
		t.Fatal(e)
	}
	_, e = s.Password(ctx, DesktopRealm, token, authn.Token(), authn.Token(), PasswordChange{testPassword, "rollback-password-123"}, []byte("opaque"))
	if e == nil {
		t.Fatal("audit failure committed password")
	}
	if _, e = s.Auth.Authenticate(ctx, DesktopRealm, token); e != nil {
		t.Fatal("rollback lost current session", e)
	}
	loginTest(t, s.Auth, DriveRealm, testPassword)
}

func securityHTTP(t *testing.T, s *Service, token, method, path string, input any) (Result, error) {
	t.Helper()
	var body []byte
	if input != nil {
		body, _ = json.Marshal(input)
	}
	session, e := s.Auth.Authenticate(context.Background(), DesktopRealm, token)
	if e != nil {
		return Result{}, e
	}
	r := httptest.NewRequest(method, "http://account.test/api/v1/identity/"+path, bytes.NewReader(body))
	r.AddCookie(&http.Cookie{Name: "minhanos_session", Value: token})
	r.Header.Set("Origin", "http://account.test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", session.CSRF)
	r.Header.Set("Idempotency-Key", authn.Token())
	w := httptest.NewRecorder()
	s.HTTPHandler(DesktopRealm).ServeHTTP(w, r)
	if w.Code >= 400 {
		return Result{}, fmt.Errorf("HTTP %d: %s", w.Code, w.Body.String())
	}
	return Result{Body: w.Body.Bytes(), Status: w.Code, ETag: w.Header().Get("ETag")}, nil
}
