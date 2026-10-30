package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"minhanos.local/platform/internal/authn"
)

func denyApp(s *Service, token, app, kind, id string) (Result, error) {
	var input AppAccessInput
	input.Subject.Kind, input.Subject.ID = kind, id
	raw, _ := json.Marshal(input)
	return s.AppAccess(context.Background(), DesktopRealm, token, app, "", authn.Token(), authn.Token(), input, raw, false)
}

func TestFilesPolicyEntitlementGrantAndDenyRules(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	u := policyUser(t, s, token, "filesuser")
	storage := mustStorage(t, s, u.ID)
	if e := s.CheckApp(ctx, storage, FilesApp, "files.read"); e != nil {
		t.Fatal("standard user lost own files", e)
	}
	if e := s.CheckApp(ctx, storage, FilesApp, "files.write"); e != nil {
		t.Fatal(e)
	}
	code(t, s.CheckApp(ctx, storage, FilesApp, "files.system.read"), "IDENTITY_PERMISSION_DENIED")
	code(t, s.CheckApp(ctx, storage, FilesApp, "identity.users.write"), "APP_PERMISSION_DENIED")

	out, e := denyApp(s, token, FilesApp, "user", u.ID)
	if e != nil || out.Status != 201 {
		t.Fatal(e, out.Status)
	}
	var rule struct{ ID string }
	json.Unmarshal(out.Body, &rule)
	code(t, s.CheckApp(ctx, storage, FilesApp, "files.read"), "APP_ACCESS_DENIED")
	_, e = denyApp(s, token, FilesApp, "user", u.ID)
	code(t, e, "IDENTITY_BINDING_EXISTS")
	if auditCount(t, s, "app.access.denied", rule.ID) != 1 {
		t.Fatal("deny rule not audited")
	}
	// Effective access shown in Account Center reflects the deny rule.
	access, e := s.Read(ctx, DesktopRealm, token, "user/access", u.ID, "", "", 50)
	if e != nil || strings.Contains(string(access.Body), "files.read") {
		t.Fatal("effective access must drop denied Files permissions", string(access.Body), e)
	}
	raw, _ := json.Marshal(struct{}{})
	if _, e = s.AppAccess(ctx, DesktopRealm, token, FilesApp, rule.ID, authn.Token(), authn.Token(), AppAccessInput{}, raw, true); e != nil {
		t.Fatal(e)
	}
	if e = s.CheckApp(ctx, storage, FilesApp, "files.read"); e != nil {
		t.Fatal("removing the rule must restore access", e)
	}
	if access, e = s.Read(ctx, DesktopRealm, token, "user/access", u.ID, "", "", 50); e != nil || !strings.Contains(string(access.Body), "files.read") {
		t.Fatal("effective access must list Files permissions", string(access.Body), e)
	}

	// Group rules apply to members.
	g := groupTest(t, s, token, "no-files")
	if _, e = s.GroupWrite(ctx, DesktopRealm, token, g.ID, "add", u.ID, fmt.Sprintf("\"%d\"", g.Version), authn.Token(), authn.Token(), GroupInput{}, []byte(u.ID)); e != nil {
		t.Fatal(e)
	}
	if _, e = denyApp(s, token, FilesApp, "group", g.ID); e != nil {
		t.Fatal(e)
	}
	code(t, s.CheckApp(ctx, storage, FilesApp, "files.write"), "APP_ACCESS_DENIED")
	if e = s.CheckApp(ctx, mustStorage(t, s, admin.ID), FilesApp, "files.read"); e != nil {
		t.Fatal("rule leaked to non-members", e)
	}
}

func TestAppAccessGuards(t *testing.T) {
	s, token, admin := policyFixture(t)
	ctx := context.Background()
	_, e := denyApp(s, token, AccountApp, "user", admin.ID)
	code(t, e, "IDENTITY_PROTECTED_POLICY")
	_, e = denyApp(s, token, "com.minhanos.unknown", "user", admin.ID)
	code(t, e, "IDENTITY_NOT_FOUND")
	_, e = denyApp(s, token, FilesApp, "device", admin.ID)
	code(t, e, "IDENTITY_INVALID_SUBJECT")
	_, e = denyApp(s, token, FilesApp, "group", "6b1d2c3e-1111-4a4a-8b8b-000000000000")
	code(t, e, "IDENTITY_NOT_FOUND")
	u := policyUser(t, s, token, "nopolicy")
	_, userToken, e := s.Auth.Login(ctx, DesktopRealm, "nopolicy", testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = denyApp(s, userToken, FilesApp, "user", u.ID); e == nil {
		t.Fatal("standard user edited app policy")
	}
	out, e := s.Read(ctx, DesktopRealm, token, "applications", "", "", "", 50)
	if e != nil || !strings.Contains(string(out.Body), FilesApp) || !strings.Contains(string(out.Body), `"allowed":true`) {
		t.Fatal("applications list", string(out.Body), e)
	}
	if out, e = s.Read(ctx, DesktopRealm, token, "app/access", FilesApp, "", "", 50); e != nil || !strings.Contains(string(out.Body), `"items":[]`) {
		t.Fatal("empty rule list", string(out.Body), e)
	}
}
