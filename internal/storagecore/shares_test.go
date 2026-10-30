package storagecore

import (
	"context"
	"errors"
	"testing"
)

const (
	alice = "00000000-0000-4000-8000-00000000000a"
	bob   = "00000000-0000-4000-8000-00000000000b"
	staff = "00000000-0000-4000-8000-0000000000aa"
)

func accessFixture(t *testing.T) (Store, Manager) {
	t.Helper()
	s := fixture(t)
	ctx := context.Background()
	for _, q := range []string{
		"INSERT INTO storage_volumes(name,kind) VALUES('tank','zfs')",
		"INSERT INTO users VALUES('key-a','" + alice + "','alice','Alice','active'),('key-b','" + bob + "','bob','Bob','active')",
		"INSERT INTO identity_groups VALUES('" + staff + "','staff','Staff')",
		"INSERT INTO identity_group_members VALUES('" + staff + "','" + bob + "')",
	} {
		if _, e := s.DB.Exec(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	return s, Manager{Store: s, Agent: fakeAgent(t, 201, "")}
}

func userAccess(t *testing.T, s Store, user string) map[string]string {
	t.Helper()
	shares, e := s.UserShares(context.Background(), user)
	if e != nil {
		t.Fatal(e)
	}
	result := map[string]string{}
	for _, share := range shares {
		result[share.Key] = share.Access
	}
	return result
}

func TestShareAccessPrecedence(t *testing.T) {
	s, m := accessFixture(t)
	ctx := context.Background()
	share, e := m.CreateShare(ctx, alice, "team", "Team", "tank", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := userAccess(t, s, "key-a"); got["team"] != "write" {
		t.Fatal("creator has no write access", got)
	}
	if got := userAccess(t, s, "key-b"); len(got) != 0 {
		t.Fatal("no entry must mean no access", got)
	}
	list, e := m.ShareAccess(ctx, share.ID)
	if e != nil || len(list.Entries) != 1 || list.Entries[0].Name != "alice" {
		t.Fatal(list, e)
	}
	entries := []AccessEntry{{SubjectType: "user", SubjectID: alice, Access: "write"}, {SubjectType: "group", SubjectID: staff, Access: "read"}}
	if e = m.SetShareAccess(ctx, alice, share.ID, list.Version, entries); e != nil {
		t.Fatal(e)
	}
	if got := userAccess(t, s, "key-b"); got["team"] != "read" {
		t.Fatal("group read", got)
	}
	if e = m.SetShareAccess(ctx, alice, share.ID, list.Version, entries); !errors.Is(e, ErrConflict) {
		t.Fatal("stale version accepted", e)
	}
	// Deny on the user wins over write through a group.
	entries = []AccessEntry{{SubjectType: "group", SubjectID: staff, Access: "write"}, {SubjectType: "user", SubjectID: bob, Access: "deny"}}
	if e = m.SetShareAccess(ctx, alice, share.ID, list.Version+1, entries); e != nil {
		t.Fatal(e)
	}
	if got := userAccess(t, s, "key-b"); len(got) != 0 {
		t.Fatal("deny must win", got)
	}
	for _, bad := range [][]AccessEntry{
		{{SubjectType: "user", SubjectID: "00000000-0000-4000-8000-0000000000ff", Access: "read"}},
		{{SubjectType: "role", SubjectID: alice, Access: "read"}},
		{{SubjectType: "user", SubjectID: alice, Access: "owner"}},
		{{SubjectType: "user", SubjectID: alice, Access: "read"}, {SubjectType: "user", SubjectID: alice, Access: "write"}},
	} {
		if e = m.SetShareAccess(ctx, alice, share.ID, list.Version+2, bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted", bad, e)
		}
	}
	homes, e := s.HomesShare(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.ShareAccess(ctx, homes.ID); !errors.Is(e, ErrInvalid) {
		t.Fatal("homes share has no share permissions", e)
	}
}

func TestShareQuota(t *testing.T) {
	s, m := accessFixture(t)
	ctx := context.Background()
	limit := 10 * mib
	share, e := m.CreateShare(ctx, alice, "team", "Team", "tank", &limit, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ReserveShare(ctx, share.ID, 8*mib)
	if e != nil {
		t.Fatal(e)
	}
	var q QuotaExceeded
	if _, e = s.ReserveShare(ctx, share.ID, 3*mib); !errors.As(e, &q) || q.Scope != "share" {
		t.Fatal("share limit not enforced", e)
	}
	if e = s.Commit(ctx, r, 6*mib); e != nil {
		t.Fatal(e)
	}
	var used, reserved int64
	s.DB.QueryRow(ctx, "SELECT used_bytes,reserved_bytes FROM storage_quota WHERE scope_type='share' AND scope_id=$1", share.ID).Scan(&used, &reserved)
	if used != 6*mib || reserved != 0 {
		t.Fatal(used, reserved)
	}
	homeUsed, _ := usage(t, s, "share", "")
	if homeUsed != 0 {
		t.Fatal("share write counted on the homes share", homeUsed)
	}
}

func TestFirstRunHomesShare(t *testing.T) {
	s, m := accessFixture(t)
	ctx := context.Background()
	if done, e := s.RetireEmptyLegacyHomes(ctx, true); done || e != nil {
		t.Fatal("legacy homes with data must stay", done, e)
	}
	if done, e := s.RetireEmptyLegacyHomes(ctx, false); !done || e != nil {
		t.Fatal("empty legacy homes not retired", done, e)
	}
	if _, e := s.HomesShare(ctx); !errors.Is(e, ErrNoHomes) {
		t.Fatal(e)
	}
	if _, e := s.Reserve(ctx, "key-a", 1); !errors.Is(e, ErrNoHomes) {
		t.Fatal("personal write without homes", e)
	}
	if _, e := m.CreateShare(ctx, alice, "homes", "Homes", "tank", nil, true); e != nil {
		t.Fatal(e)
	}
	h, e := s.HomesShare(ctx)
	if e != nil || h.Volume != "tank" || h.VolumeKind != "zfs" {
		t.Fatal(h, e)
	}
	if _, e = m.CreateShare(ctx, alice, "users2", "Users", "tank", nil, true); !errors.Is(e, ErrHomesExists) {
		t.Fatal("second homes share", e)
	}
	if _, e = s.Reserve(ctx, "key-a", 1); e != nil {
		t.Fatal(e)
	}
	if done, e := s.RetireEmptyLegacyHomes(ctx, false); done || e != nil {
		t.Fatal("a managed homes share is never retired", done, e)
	}
}
