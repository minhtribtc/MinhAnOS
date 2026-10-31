package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"minhanos.local/platform/internal/authn"
)

// Organizations (ADR 0008): roles, cross-tenant isolation (IDOR reads as not found), structure rules,
// organization groups and leaving an organization.

func orgCreate(t *testing.T, s *Service, token, name string, parent *string, children bool) (Organization, error) {
	t.Helper()
	in := OrgInput{Name: name, ParentID: parent, AllowChildren: children}
	raw, _ := json.Marshal(in)
	out, e := s.CreateOrg(context.Background(), DesktopRealm, token, authn.Token(), authn.Token(), in, raw)
	var o Organization
	if e == nil {
		if e = json.Unmarshal(out.Body, &o); e != nil {
			t.Fatal(e)
		}
	}
	return o, e
}

func orgSession(t *testing.T, s *Service, token, org, name, role string) string {
	t.Helper()
	in := OrgUserInput{Username: name, DisplayName: name, Password: testPassword, Role: role}
	raw, _ := json.Marshal(in)
	if _, e := s.CreateOrgUser(context.Background(), DesktopRealm, token, org, authn.Token(), authn.Token(), in, raw); e != nil {
		t.Fatal(e)
	}
	_, session, e := s.Auth.Login(context.Background(), DesktopRealm, name, testPassword, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Auth.Reauthenticate(context.Background(), DesktopRealm, session, testPassword); e != nil {
		t.Fatal(e)
	}
	return session
}

func orgGet(s *Service, token string, parts ...string) (Result, error) {
	return s.ReadOrganizations(context.Background(), DesktopRealm, token, append([]string{"organizations"}, parts...), "", 50)
}

func addMember(s *Service, token, org, username, role string) (Result, error) {
	in := MemberInput{Username: username, Role: role}
	raw, _ := json.Marshal(in)
	return s.MemberWrite(context.Background(), DesktopRealm, token, org, "add", "", authn.Token(), authn.Token(), in, raw)
}

func TestOrganizationsRolesAndIsolation(t *testing.T) {
	s, root, rootAccount := policyFixture(t)
	ctx := context.Background()
	acme, e := orgCreate(t, s, root, "Acme", nil, true)
	if e != nil || acme.Role != "root" || acme.Depth != 1 {
		t.Fatal(acme, e)
	}
	beta, e := orgCreate(t, s, root, "Beta", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = orgCreate(t, s, root, "acme", nil, false); e == nil {
		t.Fatal("sibling names must be unique ignoring case")
	} else {
		code(t, e, "ORG_NAME_EXISTS")
	}
	acmeAdmin := orgSession(t, s, root, acme.ID, "acme_admin", "admin")
	betaAdmin := orgSession(t, s, root, beta.ID, "beta_admin", "admin")

	// Only Root creates top-level organizations; an admin creates children where the parent allows them.
	_, e = orgCreate(t, s, acmeAdmin, "Rogue", nil, false)
	code(t, e, "ORG_PERMISSION_DENIED")
	sales, e := orgCreate(t, s, acmeAdmin, "Sales", &acme.ID, false)
	if e != nil || sales.Role != "manager" || sales.Depth != 2 {
		t.Fatal(sales, e)
	}
	_, e = orgCreate(t, s, betaAdmin, "Child", &beta.ID, false)
	code(t, e, "ORG_CHILDREN_NOT_ALLOWED")

	// Cross-tenant: another organization's admin cannot read, list, change or join anything there.
	for _, parts := range [][]string{{acme.ID}, {acme.ID, "members"}, {acme.ID, "groups"}, {sales.ID}} {
		_, e = orgGet(s, betaAdmin, parts...)
		code(t, e, "ORG_NOT_FOUND")
	}
	_, e = addMember(s, betaAdmin, acme.ID, "beta_admin", "admin")
	code(t, e, "ORG_NOT_FOUND")
	name := "Hijacked"
	patch, _ := json.Marshal(OrgPatch{Name: &name})
	_, e = s.PatchOrg(ctx, DesktopRealm, betaAdmin, acme.ID, fmt.Sprintf("\"%d\"", acme.Version), authn.Token(), OrgPatch{Name: &name}, patch)
	code(t, e, "ORG_NOT_FOUND")
	out, e := orgGet(s, betaAdmin)
	if e != nil {
		t.Fatal(e)
	}
	var list struct{ Items []Organization }
	json.Unmarshal(out.Body, &list)
	if len(list.Items) != 1 || list.Items[0].ID != beta.ID {
		t.Fatalf("beta admin sees %+v", list.Items)
	}
	out, _ = orgGet(s, acmeAdmin)
	json.Unmarshal(out.Body, &list)
	roles := map[string]string{}
	for _, o := range list.Items {
		roles[o.ID] = o.Role
	}
	if len(roles) != 2 || roles[acme.ID] != "admin" || roles[sales.ID] != "manager" {
		t.Fatalf("acme admin sees %v", roles)
	}

	// An admin renames but cannot lift its own limits (status, children); a manager or Root can.
	suspended := "suspended"
	raw, _ := json.Marshal(OrgPatch{Status: &suspended})
	_, e = s.PatchOrg(ctx, DesktopRealm, acmeAdmin, acme.ID, fmt.Sprintf("\"%d\"", acme.Version), authn.Token(), OrgPatch{Status: &suspended}, raw)
	code(t, e, "ORG_PERMISSION_DENIED")
	if _, e = s.PatchOrg(ctx, DesktopRealm, acmeAdmin, acme.ID, fmt.Sprintf("\"%d\"", acme.Version), authn.Token(), OrgPatch{Name: &name}, patch); e != nil {
		t.Fatal(e)
	}

	// Members: added by username, notified; a plain member cannot list members.
	if _, e = addMember(s, acmeAdmin, acme.ID, "beta_admin", "member"); e != nil {
		t.Fatal(e)
	}
	_, e = addMember(s, acmeAdmin, acme.ID, "beta_admin", "member")
	code(t, e, "ORG_MEMBER_EXISTS")
	_, e = addMember(s, acmeAdmin, acme.ID, "nobody_here", "member")
	code(t, e, "IDENTITY_NOT_FOUND")
	if out, e = orgGet(s, betaAdmin, acme.ID); e != nil || !strings.Contains(string(out.Body), `"role":"member"`) {
		t.Fatal("member view", string(out.Body), e)
	}
	_, e = orgGet(s, betaAdmin, acme.ID, "members")
	code(t, e, "ORG_PERMISSION_DENIED")
	var notices int
	s.db.QueryRow(ctx, "SELECT count(*) FROM identity_notification_outbox o JOIN users u ON u.public_id=o.recipient_id WHERE u.username='beta_admin' AND o.kind='organization.joined'").Scan(&notices)
	if notices != 1 {
		t.Fatal("joined notice", notices)
	}

	// Organization groups take only members; leaving the organization leaves its groups.
	group := GroupInput{Key: "staff", Name: "Staff"}
	graw, _ := json.Marshal(group)
	gout, e := s.OrgGroupWrite(ctx, DesktopRealm, acmeAdmin, acme.ID, "", "create", "", "", authn.Token(), authn.Token(), group, graw)
	if e != nil {
		t.Fatal(e)
	}
	var g Group
	json.Unmarshal(gout.Body, &g)
	// The same key in another organization is not a conflict (keys are per organization).
	if _, e = s.OrgGroupWrite(ctx, DesktopRealm, betaAdmin, beta.ID, "", "create", "", "", authn.Token(), authn.Token(), group, graw); e != nil {
		t.Fatal(e)
	}
	var betaUser string
	s.db.QueryRow(ctx, "SELECT public_id::text FROM users WHERE username='beta_admin'").Scan(&betaUser)
	rootUser := rootAccount.ID
	if _, e = s.OrgGroupWrite(ctx, DesktopRealm, acmeAdmin, acme.ID, g.ID, "add", betaUser, fmt.Sprintf("\"%d\"", g.Version), authn.Token(), authn.Token(), GroupInput{}, []byte(betaUser)); e != nil {
		t.Fatal(e)
	}
	_, e = s.OrgGroupWrite(ctx, DesktopRealm, acmeAdmin, acme.ID, g.ID, "add", rootUser, fmt.Sprintf("\"%d\"", g.Version+1), authn.Token(), authn.Token(), GroupInput{}, []byte(rootUser))
	code(t, e, "IDENTITY_NOT_MEMBER")
	// Beta's admin cannot touch Acme's group, even by ID through Beta.
	_, e = s.OrgGroupWrite(ctx, DesktopRealm, betaAdmin, beta.ID, g.ID, "delete", "", fmt.Sprintf("\"%d\"", g.Version+1), authn.Token(), authn.Token(), GroupInput{}, nil)
	code(t, e, "IDENTITY_NOT_FOUND")
	if _, e = s.MemberWrite(ctx, DesktopRealm, acmeAdmin, acme.ID, "remove", betaUser, authn.Token(), authn.Token(), MemberInput{}, nil); e != nil {
		t.Fatal(e)
	}
	var left int
	s.db.QueryRow(ctx, "SELECT count(*) FROM identity_group_members WHERE group_id=$1", g.ID).Scan(&left)
	if left != 0 {
		t.Fatal("left member stayed in the organization's group")
	}
	_, e = orgGet(s, betaAdmin, acme.ID)
	code(t, e, "ORG_NOT_FOUND")

	// Audit rows carry the organization.
	var audited int
	s.db.QueryRow(ctx, "SELECT count(*) FROM identity_admin_events WHERE organization_id=$1 AND action IN ('org.member.added','org.member.removed','org.updated','group.created','group.member.added')", acme.ID).Scan(&audited)
	if audited != 5 {
		t.Fatal("organization audit rows", audited)
	}

	// Suspension removes the admin's and the managers' authority below it; Root keeps it.
	acme, _ = readOrgAs(t, s, root, acme.ID)
	raw, _ = json.Marshal(OrgPatch{Status: &suspended})
	if _, e = s.PatchOrg(ctx, DesktopRealm, root, acme.ID, fmt.Sprintf("\"%d\"", acme.Version), authn.Token(), OrgPatch{Status: &suspended}, raw); e != nil {
		t.Fatal(e)
	}
	_, e = orgGet(s, acmeAdmin, sales.ID)
	code(t, e, "ORG_NOT_FOUND")

	// Structure: the default organization stays; a non-empty organization cannot be deleted.
	var def Organization
	s.db.QueryRow(ctx, "SELECT id::text,version FROM identity_organizations WHERE is_default").Scan(&def.ID, &def.Version)
	_, e = s.DeleteOrg(ctx, DesktopRealm, root, def.ID, fmt.Sprintf("\"%d\"", def.Version), authn.Token(), authn.Token(), nil)
	code(t, e, "ORG_DEFAULT_PROTECTED")
	_, e = s.DeleteOrg(ctx, DesktopRealm, root, acme.ID, fmt.Sprintf("\"%d\"", acme.Version+1), authn.Token(), authn.Token(), nil)
	code(t, e, "ORG_NOT_EMPTY")
	sales, _ = readOrgAs(t, s, root, sales.ID)
	if _, e = s.DeleteOrg(ctx, DesktopRealm, root, sales.ID, fmt.Sprintf("\"%d\"", sales.Version), authn.Token(), authn.Token(), nil); e != nil {
		t.Fatal(e)
	}
}

func TestOrganizationAdoptionAndDepth(t *testing.T) {
	s, root, admin := policyFixture(t)
	ctx := context.Background()
	var role string
	if e := s.db.QueryRow(ctx, "SELECT m.role FROM identity_memberships m JOIN identity_organizations o ON o.id=m.organization_id AND o.is_default WHERE m.user_id=$1", admin.ID).Scan(&role); e != nil || role != "admin" {
		t.Fatal("bootstrap admin not adopted as default-organization admin", role, e)
	}
	u := policyUser(t, s, root, "joins_default")
	if e := s.db.QueryRow(ctx, "SELECT m.role FROM identity_memberships m JOIN identity_organizations o ON o.id=m.organization_id AND o.is_default WHERE m.user_id=$1", u.ID).Scan(&role); e != nil || role != "member" {
		t.Fatal("platform-created account not in default organization", role, e)
	}
	parent, e := orgCreate(t, s, root, "L1", nil, true)
	if e != nil {
		t.Fatal(e)
	}
	for depth := 2; depth <= maxOrgDepth; depth++ {
		if parent, e = orgCreate(t, s, root, fmt.Sprintf("L%d", depth), &parent.ID, true); e != nil {
			t.Fatal(depth, e)
		}
	}
	_, e = orgCreate(t, s, root, "TooDeep", &parent.ID, true)
	code(t, e, "ORG_TOO_DEEP")
	if _, e = s.db.Exec(ctx, "UPDATE identity_organizations SET parent_id=NULL WHERE id=$1", parent.ID); e == nil {
		t.Fatal("re-parenting must be refused")
	}
}

func readOrgAs(t *testing.T, s *Service, token, id string) (Organization, error) {
	t.Helper()
	out, e := orgGet(s, token, id)
	var o Organization
	if e == nil {
		json.Unmarshal(out.Body, &o)
	}
	return o, e
}
