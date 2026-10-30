package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"minhanos.local/platform/internal/authn"
)

func policyFixture(t *testing.T) (*Service, string, User) {
	t.Helper()
	a, db := authFixture(t)
	s, e := NewService(context.Background(), db, a)
	if e != nil {
		t.Fatal(e)
	}
	token := loginTest(t, a, DesktopRealm, testPassword)
	if e = a.Reauthenticate(context.Background(), DesktopRealm, token, testPassword); e != nil {
		t.Fatal(e)
	}
	u, e := (Repository{db}).PublicUser(context.Background(), strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	return s, token, u
}
func policyUser(t *testing.T, s *Service, token, name string) User {
	t.Helper()
	input := CreateUser{name, "Disposable", testPassword}
	raw, _ := json.Marshal(input)
	out, e := s.Create(context.Background(), DesktopRealm, token, input, "user", authn.Token(), authn.Token(), raw, false)
	if e != nil {
		t.Fatal(e)
	}
	var u User
	if e = json.Unmarshal(out.Body, &u); e != nil {
		t.Fatal(e)
	}
	return u
}
func policyChange(s *Service, token string, u User, op string) (Result, error) {
	return s.ChangeUser(context.Background(), DesktopRealm, token, u.ID, op, "Changed", fmt.Sprintf("\"%d\"", u.Version), authn.Token(), authn.Token(), []byte(op))
}
func code(t *testing.T, e error, expected string) {
	t.Helper()
	var f *Fault
	if !errors.As(e, &f) || f.Code != expected {
		t.Fatalf("want %s: %v", expected, e)
	}
}
func decodeUser(t *testing.T, out Result) User {
	t.Helper()
	var u User
	if e := json.Unmarshal(out.Body, &u); e != nil {
		t.Fatal(e)
	}
	return u
}

func TestPolicyLastAdminConcurrentAndHostSplit(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	for _, op := range []string{"disable", "standard-user", "user-admin", "delete"} {
		_, e := policyChange(s, token, admin, op)
		code(t, e, "LAST_ADMIN_REQUIRED")
	}
	allowed, e := s.CanAccessHost(ctx, strings.Repeat("a", 64))
	if e != nil || !allowed {
		t.Fatal("existing operator lost", e)
	}
	second := policyUser(t, s, token, "second_admin")
	out, e := policyChange(s, token, second, "system-admin")
	if e != nil {
		t.Fatal(e)
	}
	second = decodeUser(t, out)
	storage, e := (Repository{s.db}).LegacyID(ctx, second.ID)
	if e != nil {
		t.Fatal(e)
	}
	allowed, e = s.CanAccessHost(ctx, storage)
	if e != nil || allowed {
		t.Fatal("new web admin inherited host", e)
	}
	_, secondToken, e := s.Auth.Login(ctx, DesktopRealm, "second_admin", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Auth.Reauthenticate(ctx, DesktopRealm, secondToken, testPassword); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, v := range []struct {
		token string
		u     User
	}{{token, admin}, {secondToken, second}} {
		wg.Add(1)
		go func(v struct {
			token string
			u     User
		}) {
			defer wg.Done()
			_, e := policyChange(s, v.token, v.u, "disable")
			outcomes <- e
		}(v)
	}
	wg.Wait()
	close(outcomes)
	success := 0
	for e := range outcomes {
		if e == nil {
			success++
		} else {
			code(t, e, "LAST_ADMIN_REQUIRED")
		}
	}
	if success != 1 {
		t.Fatal("race", success)
	}
	var remaining int
	s.db.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='admin' AND status='active'").Scan(&remaining)
	if remaining != 1 {
		t.Fatal("lost final admin")
	}
}

func TestDelegatedCeilingUnknownScopesAndSelfEscalation(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	delegated := policyUser(t, s, token, "delegate")
	out, e := policyChange(s, token, delegated, "user-admin")
	if e != nil {
		t.Fatal(e)
	}
	delegated = decodeUser(t, out)
	_, userToken, e := s.Auth.Login(ctx, DesktopRealm, "delegate", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Auth.Reauthenticate(ctx, DesktopRealm, userToken, testPassword); e != nil {
		t.Fatal(e)
	}
	plain := policyUser(t, s, userToken, "plain_user")
	if _, e = policyChange(s, userToken, admin, "disable"); e == nil {
		t.Fatal("delegate mutated admin")
	} else {
		code(t, e, "IDENTITY_PROTECTED_USER")
	}
	if _, e = policyChange(s, userToken, delegated, "system-admin"); e == nil {
		t.Fatal("self escalation")
	} else {
		code(t, e, "IDENTITY_PERMISSION_DENIED")
	}
	if _, e = s.Create(ctx, DesktopRealm, userToken, CreateUser{"forged_admin", "Forged", testPassword}, "admin", authn.Token(), authn.Token(), []byte("test"), false); e == nil {
		t.Fatal("delegate created admin")
	} else {
		code(t, e, "IDENTITY_PERMISSION_DENIED")
	}
	if _, e = policyChange(s, userToken, plain, "disable"); e != nil {
		t.Fatal("delegate could not manage standard user", e)
	}
	if _, e = s.db.Exec(ctx, "UPDATE identity_role_bindings SET scope='{}' WHERE user_id=$1", delegated.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Read(ctx, DesktopRealm, userToken, "users", "", "", "", 10); e == nil {
		t.Fatal("unknown scope authorized")
	} else {
		code(t, e, "IDENTITY_PERMISSION_DENIED")
	}
}

func TestMutationReplayPreconditionsAndRevokedAppGrant(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	input := CreateUser{"repeat_user", "Disposable", testPassword}
	raw, _ := json.Marshal(input)
	key := authn.Token()
	first, e := s.Create(ctx, DesktopRealm, token, input, "user", key, authn.Token(), raw, false)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Create(ctx, DesktopRealm, token, input, "user", key, authn.Token(), raw, false)
	if e != nil || string(first.Body) != string(again.Body) {
		t.Fatal("replay", e)
	}
	input.DisplayName = "Different"
	raw, _ = json.Marshal(input)
	if _, e = s.Create(ctx, DesktopRealm, token, input, "user", key, authn.Token(), raw, false); e == nil {
		t.Fatal("key rebound")
	} else {
		code(t, e, "IDEMPOTENCY_CONFLICT")
	}
	u := decodeUser(t, first)
	for match, want := range map[string]string{"": "PRECONDITION_REQUIRED", "\"99\"": "VERSION_CONFLICT"} {
		_, e = s.ChangeUser(ctx, DesktopRealm, token, u.ID, "patch", "Name", match, "", authn.Token(), []byte("test"))
		code(t, e, want)
	}
	if _, e = s.db.Exec(ctx, "UPDATE identity_app_grants SET state='revoked' WHERE permission_key='identity.users.write'"); e != nil {
		t.Fatal(e)
	}
	input.DisplayName = "Disposable"
	raw, _ = json.Marshal(input)
	_, e = s.Create(ctx, DesktopRealm, token, input, "user", key, authn.Token(), raw, false)
	code(t, e, "APP_PERMISSION_DENIED")
	var serialized string
	s.db.QueryRow(ctx, "SELECT row_to_json(m)::text FROM identity_mutations m LIMIT 1").Scan(&serialized)
	if strings.Contains(serialized, testPassword) || strings.Contains(serialized, "password_hash") {
		t.Fatal("secret persisted in replay")
	}
}

func TestPolicyAuditAtomicityAndSoftDelete(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "remove_user")
	_, desktop, e := s.Auth.Login(ctx, DesktopRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, drive, e := s.Auth.Login(ctx, DriveRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = policyChange(s, token, u, "delete")
	code(t, e, "IDENTITY_DISABLE_FIRST")
	out, e := policyChange(s, token, u, "disable")
	if e != nil {
		t.Fatal(e)
	}
	u = decodeUser(t, out)
	denied(t, s.Auth, DesktopRealm, desktop)
	denied(t, s.Auth, DriveRealm, drive)
	if _, e = policyChange(s, token, u, "delete"); e != nil {
		t.Fatal(e)
	}
	var n int
	s.db.QueryRow(ctx, "SELECT count(*) FROM users WHERE public_id=$1 AND status='deleted'", u.ID).Scan(&n)
	if n != 1 {
		t.Fatal("soft deletion lost storage row")
	}
	next := policyUser(t, s, token, "rollback_user")
	if _, e = s.db.Exec(ctx, "DROP TABLE identity_admin_events"); e != nil {
		t.Fatal(e)
	}
	if _, e = policyChange(s, token, next, "disable"); e == nil {
		t.Fatal("audit failure committed")
	}
	user, e := (Repository{s.db}).PublicUser(ctx, mustStorage(t, s, next.ID))
	if e != nil || user.Status != "active" {
		t.Fatal("audit rollback", e)
	}
}
func mustStorage(t *testing.T, s *Service, id string) string {
	t.Helper()
	v, e := (Repository{s.db}).LegacyID(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestCursorAuthenticationAndNoStorageIDs(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	policyUser(t, s, token, "page_one")
	policyUser(t, s, token, "page_two")
	out, e := s.Read(ctx, DesktopRealm, token, "users", "", "", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	var page struct {
		Next string `json:"nextCursor"`
	}
	json.Unmarshal(out.Body, &page)
	if page.Next == "" {
		t.Fatal("no cursor")
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "users", "", "", page.Next, 1); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "users", "", "page", page.Next, 1); e == nil {
		t.Fatal("cursor filter rebound")
	} else {
		code(t, e, "INVALID_CURSOR")
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "roles", "", "", page.Next, 1); e == nil {
		t.Fatal("cursor route rebound")
	} else {
		code(t, e, "INVALID_CURSOR")
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "users", "", "", page.Next+"x", 1); e == nil {
		t.Fatal("cursor tampered")
	} else {
		code(t, e, "INVALID_CURSOR")
	}
	_ = admin
}

func TestHTTPV1ErrorsCSRFAndMassAssignment(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	sess, e := s.Auth.Authenticate(ctx, DesktopRealm, token)
	if e != nil {
		t.Fatal(e)
	}
	handler := s.HTTPHandler(DesktopRealm)
	call := func(path, method, body, csrf, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.test/api/v1/identity/"+path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "minhanos_session", Value: token})
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Idempotency-Key", authn.Token())
		r.Header.Set("X-App-ID", "com.minhanos.desktop")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, v := range []struct{ csrf, origin string }{{"", "http://example.test"}, {sess.CSRF, "http://evil.test"}, {sess.CSRF, "https://example.test"}} {
		w := call("users", "POST", `{}`, v.csrf, v.origin)
		if w.Code != 403 || !strings.Contains(w.Body.String(), `"requestId"`) {
			t.Fatal("nested security error", w.Code, w.Body.String())
		}
	}
	w := call("users", "POST", `{"username":"forged","displayName":"Forged","password":"original-password-123","role":"admin","subject":"user:evil"}`, sess.CSRF, "http://example.test")
	if w.Code != 400 {
		t.Fatal("mass assignment", w.Code, w.Body.String())
	}
	w = call("me", "GET", "", "", "")
	if w.Code != 200 || w.Header().Get("ETag") == "" {
		t.Fatal("me", w.Code, w.Body.String())
	}
	for _, path := range []string{"me", "users", "roles", "permissions", "audit-events"} {
		if w = call(path, "GET", "", "", ""); w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
}

func TestInheritedPrivilegesStayProtectedAndDenyOverrides(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	delegate := policyUser(t, s, token, "direct_delegate")
	out, e := policyChange(s, token, delegate, "user-admin")
	if e != nil {
		t.Fatal(e)
	}
	delegate = decodeUser(t, out)
	target := policyUser(t, s, token, "group_delegate")
	var group string
	if e = s.db.QueryRow(ctx, "INSERT INTO identity_groups(key,name,kind) VALUES('admins','Delegates','user') RETURNING id::text").Scan(&group); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec(ctx, "INSERT INTO identity_group_members(group_id,user_id) VALUES($1,$2)", group, target.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec(ctx, `INSERT INTO identity_role_bindings(role_id,group_id,scope) SELECT id,$1,'{"kind":"system","id":null}' FROM identity_roles WHERE key='user-admin'`, group); e != nil {
		t.Fatal(e)
	}
	_, dt, e := s.Auth.Login(ctx, DesktopRealm, delegate.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Auth.Reauthenticate(ctx, DesktopRealm, dt, testPassword); e != nil {
		t.Fatal(e)
	}
	if _, e = policyChange(s, dt, target, "disable"); e == nil {
		t.Fatal("group-derived admin not protected")
	} else {
		code(t, e, "IDENTITY_PROTECTED_USER")
	}
	if _, e = policyChange(s, token, target, "standard-user"); e == nil {
		t.Fatal("silently kept inherited role")
	} else {
		code(t, e, "IDENTITY_GROUP_ROLE_CONFLICT")
	}
	_, tt, e := s.Auth.Login(ctx, DesktopRealm, target.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Read(ctx, DesktopRealm, tt, "users", "", "", "", 10); e != nil {
		t.Fatal("group delegation not resolved", e)
	}
	if _, e = s.db.Exec(ctx, `INSERT INTO identity_app_access(app_id,role_id,effect,scope) SELECT $1,id,'deny','{"kind":"system","id":null}' FROM identity_roles WHERE key='user-admin'`, AccountApp); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Read(ctx, DesktopRealm, tt, "users", "", "", "", 10); e == nil {
		t.Fatal("group-derived role escaped app deny")
	} else {
		code(t, e, "APP_ACCESS_DENIED")
	}
}

func TestStandardUserCanPatchOnlySelfWithoutFreshAdminTicket(t *testing.T) {
	s, adminToken, admin := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, adminToken, "self_user")
	_, token, e := s.Auth.Login(ctx, DesktopRealm, u.Username, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.ChangeUser(ctx, DesktopRealm, token, u.ID, "self", "My name", fmt.Sprintf("\"%d\"", u.Version), "", authn.Token(), []byte("My name"))
	if e != nil {
		t.Fatal(e)
	}
	changed := decodeUser(t, out)
	if changed.DisplayName != "My name" || changed.Version <= u.Version {
		t.Fatal("self profile", changed)
	}
	if _, e = s.ChangeUser(ctx, DesktopRealm, token, admin.ID, "self", "Forged", fmt.Sprintf("\"%d\"", admin.Version), "", authn.Token(), []byte("Forged")); e == nil {
		t.Fatal("cross-user self patch")
	} else {
		code(t, e, "IDENTITY_PERMISSION_DENIED")
	}
	if _, e = s.Read(ctx, DesktopRealm, token, "users", "", "", "", 10); e == nil {
		t.Fatal("standard user listed identities")
	} else {
		code(t, e, "IDENTITY_PERMISSION_DENIED")
	}
	if _, e = s.Create(ctx, DesktopRealm, adminToken, CreateUser{"short_unicode", "Unicode", "密码密码"}, "user", authn.Token(), authn.Token(), []byte("test"), false); !errors.Is(e, ErrPasswordPolicy) {
		t.Fatal("schema minLength was bypassed", e)
	}
}

func TestV1WaitsForBoundedHistoricalBackfill(t *testing.T) {
	s, token, _ := policyFixture(t)
	ctx := context.Background()
	if _, e := s.db.Exec(ctx, "INSERT INTO users(id,username,display_name,role,password_hash,public_id,password_credential_id) VALUES($1,'pending_user','Pending','user',$2,NULL,NULL)", strings.Repeat("f", 64), strings.Repeat("b", 32)+":"+strings.Repeat("c", 64)); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "http://example.test/api/v1/identity/me", nil)
	r.AddCookie(&http.Cookie{Name: "minhanos_session", Value: token})
	w := httptest.NewRecorder()
	s.HTTPHandler(DesktopRealm).ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "IDENTITY_NOT_READY") {
		t.Fatal("incomplete migration exposed v1", w.Code, w.Body.String())
	}
	if n, e := (Repository{s.db}).BackfillBatch(ctx, 1); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	w = httptest.NewRecorder()
	s.HTTPHandler(DesktopRealm).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("backfill did not open v1", w.Code, w.Body.String())
	}
}
