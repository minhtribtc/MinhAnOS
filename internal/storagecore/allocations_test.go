package storagecore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// Allocations and organization-owned shares (ADR 0008): carve-out without over-allocation (also under
// concurrency), the allocation ledger on writes, and cross-tenant isolation of Shared Folders.

const (
	orgA  = "00000000-0000-4000-8000-0000000000a1"
	orgB  = "00000000-0000-4000-8000-0000000000b1"
	carol = "00000000-0000-4000-8000-00000000000c"
)

func orgFixture(t *testing.T) (Store, Manager) {
	t.Helper()
	s := fixture(t)
	ctx := context.Background()
	for _, q := range []string{
		"INSERT INTO storage_volumes(name,kind) VALUES('tank','zfs')",
		"INSERT INTO identity_organizations(id,name) VALUES('" + orgA + "','Acme')",
		"INSERT INTO identity_organizations(id,name,parent_id) VALUES('" + orgB + "','Acme Sales','" + orgA + "')",
		"INSERT INTO users VALUES('key-c','" + carol + "','carol','Carol','active')",
		"INSERT INTO identity_memberships VALUES('" + orgB + "','" + carol + "','admin')",
	} {
		if _, e := s.DB.Exec(ctx, q); e != nil {
			t.Fatal(q, e)
		}
	}
	return s, Manager{Store: s, Agent: fakeAgent(t, 201, "")}
}

func TestAllocationCarveOut(t *testing.T) {
	s, m := orgFixture(t)
	ctx := context.Background()
	var def int
	s.DB.QueryRow(ctx, "SELECT count(*) FROM storage_allocations WHERE organization_id=$1 AND volume='tank' AND limit_bytes IS NULL", defaultOrg).Scan(&def)
	if def != 1 {
		t.Fatal("a new pool must start with an unlimited default-organization allocation")
	}
	sixty := 60 * mib
	if _, e := m.CreateAllocation(ctx, "", orgB, "tank", &sixty); !errors.Is(e, ErrNoParentAllocation) {
		t.Fatal("child without parent allocation", e)
	}
	hundred := 100 * mib
	parent, e := m.CreateAllocation(ctx, "", orgA, "tank", &hundred)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.CreateAllocation(ctx, "", orgB, "tank", nil); !errors.Is(e, ErrInvalid) {
		t.Fatal("a child allocation needs a limit", e)
	}
	child, e := m.CreateAllocation(ctx, "", orgB, "tank", &sixty)
	if e != nil {
		t.Fatal(e)
	}
	if limit, _ := ledgerLimit(t, s, parent.ID); limit != 40*mib {
		t.Fatal("parent direct budget", limit)
	}
	// Growing the child past the parent's direct budget is refused; shrinking returns budget.
	tooBig := 101 * mib
	if e = m.SetAllocationLimit(ctx, "", child.ID, 1, &tooBig); !errors.Is(e, ErrAllocationExceeded) {
		t.Fatal("over-allocation", e)
	}
	twenty := 20 * mib
	if e = m.SetAllocationLimit(ctx, "", child.ID, 1, &twenty); e != nil {
		t.Fatal(e)
	}
	if limit, _ := ledgerLimit(t, s, parent.ID); limit != 80*mib {
		t.Fatal("budget not returned", limit)
	}
	// The parent cannot drop below what it delegated.
	ten := 10 * mib
	if e = m.SetAllocationLimit(ctx, "", parent.ID, 1, &ten); !errors.Is(e, ErrConflict) && !errors.Is(e, ErrAllocationCommitted) {
		t.Fatal(e)
	}
	var version int64
	s.DB.QueryRow(ctx, "SELECT version FROM storage_allocations WHERE id=$1", parent.ID).Scan(&version)
	if e = m.SetAllocationLimit(ctx, "", parent.ID, version, &ten); !errors.Is(e, ErrAllocationCommitted) {
		t.Fatal("below delegated", e)
	}

	// A share on the child's allocation: the allocation refuses what the (unlimited) share would allow.
	share, e := m.CreateOrgShare(ctx, carol, orgB, "sales", "Sales", "tank", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ReserveShare(ctx, share.ID, nil, 15*mib)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReserveShare(ctx, share.ID, nil, 6*mib); !errors.As(e, &QuotaExceeded{}) || e.(QuotaExceeded).Scope != "allocation" {
		t.Fatal("allocation must refuse", e)
	}
	if e = s.Commit(ctx, r, 12*mib); e != nil {
		t.Fatal(e)
	}
	if _, used := ledgerLimit(t, s, child.ID); used != 12*mib {
		t.Fatal("allocation usage", used)
	}
	if e = m.SetAllocationLimit(ctx, "", child.ID, 2, &ten); !errors.Is(e, ErrAllocationCommitted) {
		t.Fatal("below used", e)
	}
	if e = s.SetShareMeasured(ctx, share.ID, 2*mib); e != nil {
		t.Fatal(e)
	}
	if _, used := ledgerLimit(t, s, child.ID); used != 2*mib {
		t.Fatal("measurement must move the allocation", used)
	}
	if e = m.DeleteAllocation(ctx, "", child.ID, 2); !errors.Is(e, ErrNotEmpty) {
		t.Fatal("allocation with a share", e)
	}
	var orgEvents int
	s.DB.QueryRow(ctx, "SELECT count(*) FROM storage_events WHERE organization_id=$1", orgB).Scan(&orgEvents)
	if orgEvents < 3 { // allocation created, limit changed, share created
		t.Fatal("storage audit must carry the organization", orgEvents)
	}
}

func TestConcurrentAllocationsNeverOverAllocate(t *testing.T) {
	s, m := orgFixture(t)
	ctx := context.Background()
	hundred := 100 * mib
	parent, e := m.CreateAllocation(ctx, "", orgA, "tank", &hundred)
	if e != nil {
		t.Fatal(e)
	}
	children := []string{}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-0000000001%02d", i)
		s.DB.Exec(ctx, "INSERT INTO identity_organizations(id,name,parent_id) VALUES($1,$2,$3)", id, "child"+id, orgA)
		children = append(children, id)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for _, id := range children {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			twenty := 20 * mib
			if _, e := m.CreateAllocation(ctx, "", id, "tank", &twenty); e == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			} else if !errors.Is(e, ErrAllocationExceeded) {
				t.Error(e)
			}
		}(id)
	}
	wg.Wait()
	var delegated, sum int64
	s.DB.QueryRow(ctx, "SELECT delegated_bytes FROM storage_allocations WHERE id=$1", parent.ID).Scan(&delegated)
	s.DB.QueryRow(ctx, "SELECT coalesce(sum(limit_bytes),0) FROM storage_allocations WHERE parent_id=$1", parent.ID).Scan(&sum)
	if ok != 5 || delegated != 100*mib || sum != delegated {
		t.Fatal("concurrent allocations", ok, delegated, sum)
	}
}

func TestSharesAreTenantIsolated(t *testing.T) {
	s, m := orgFixture(t)
	ctx := context.Background()
	if _, e := m.CreateOrgShare(ctx, carol, orgA, "acme", "Acme", "tank", nil, false); !errors.Is(e, ErrNoAllocation) {
		t.Fatal("share without allocation", e)
	}
	hundred := 100 * mib
	if _, e := m.CreateAllocation(ctx, "", orgA, "tank", &hundred); e != nil {
		t.Fatal(e)
	}
	ten := 10 * mib
	if _, e := m.CreateAllocation(ctx, "", orgB, "tank", &ten); e != nil {
		t.Fatal(e)
	}
	// Carol (orgB admin, not an orgA member) creates an orgA share: no implicit access for the creator.
	acme, e := m.CreateOrgShare(ctx, carol, orgA, "acme", "Acme", "tank", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	sales, e := m.CreateOrgShare(ctx, carol, orgB, "sales", "Sales", "tank", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	list, _ := s.UserShares(ctx, "key-c")
	if len(list) != 1 || list[0].Key != "sales" || list[0].OrganizationID != orgB {
		t.Fatalf("carol sees %+v", list)
	}
	// A permission for someone outside the owning organization is refused.
	if e = m.SetShareAccess(ctx, "", acme.ID, 1, []AccessEntry{{SubjectType: "user", SubjectID: carol, Access: "write"}}); !errors.Is(e, ErrInvalid) {
		t.Fatal("cross-tenant permission accepted", e)
	}
	// An ACL row that bypassed validation still gives nothing without membership.
	s.DB.Exec(ctx, "INSERT INTO storage_share_access VALUES($1,'user',$2,'write')", acme.ID, carol)
	if list, _ = s.UserShares(ctx, "key-c"); len(list) != 1 {
		t.Fatal("non-member reached another organization's share")
	}
	// Suspending the parent organization closes the child's shares; leaving removes permissions.
	s.DB.Exec(ctx, "UPDATE identity_organizations SET status='suspended' WHERE id=$1", orgA)
	if list, _ = s.UserShares(ctx, "key-c"); len(list) != 0 {
		t.Fatal("suspended organization still serves shares")
	}
	s.DB.Exec(ctx, "UPDATE identity_organizations SET status='active' WHERE id=$1", orgA)
	s.DB.Exec(ctx, "DELETE FROM identity_memberships WHERE organization_id=$1 AND user_id=$2", orgB, carol)
	var left int
	s.DB.QueryRow(ctx, "SELECT count(*) FROM storage_share_access WHERE share_id=$1", sales.ID).Scan(&left)
	if left != 0 {
		t.Fatal("leaving the organization kept share permissions")
	}
	subjects, _ := m.Subjects(ctx, orgB)
	for _, x := range subjects {
		if x.Type == "user" {
			t.Fatal("subjects of an organization list only its members", subjects)
		}
	}
}

func ledgerLimit(t *testing.T, s Store, allocation string) (limit, used int64) {
	t.Helper()
	var l *int64
	if e := s.DB.QueryRow(context.Background(), "SELECT limit_bytes,used_bytes FROM storage_quota WHERE scope_type='allocation' AND scope_id=$1", allocation).Scan(&l, &used); e != nil {
		t.Fatal(e)
	}
	if l != nil {
		limit = *l
	}
	return
}
