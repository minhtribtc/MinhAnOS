package storagecore

import (
	"context"
	"errors"
	"testing"
)

func folderUsage(t *testing.T, s Store, id string) (used, reserved int64) {
	t.Helper()
	if e := s.DB.QueryRow(context.Background(), "SELECT used_bytes,reserved_bytes FROM storage_quota WHERE scope_type='folder' AND scope_id=$1", id).Scan(&used, &reserved); e != nil {
		t.Fatal(e)
	}
	return
}

func TestQuotaFoldersNestedLimits(t *testing.T) {
	s, m := accessFixture(t)
	ctx := context.Background()
	shareLimit, outerLimit, innerLimit := 100*mib, 20*mib, 5*mib
	share, e := m.CreateShare(ctx, alice, "team", "Team", "tank", &shareLimit, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{"", "/abs", "a//b", "a/../b", ".", "a/./b", "a\\b", "a/"} {
		if _, e = m.CreateQuotaFolder(ctx, alice, share.ID, bad, nil); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted path", bad, e)
		}
	}
	homes, _ := s.HomesShare(ctx)
	if _, e = m.CreateQuotaFolder(ctx, alice, homes.ID, "x", nil); !errors.Is(e, ErrInvalid) {
		t.Fatal("quota folder in the homes share", e)
	}
	outer, e := m.CreateQuotaFolder(ctx, alice, share.ID, "projects", &outerLimit)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := m.CreateQuotaFolder(ctx, alice, share.ID, "projects/small", &innerLimit)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.CreateQuotaFolder(ctx, alice, share.ID, "projects", nil); !errors.Is(e, ErrExists) {
		t.Fatal("duplicate", e)
	}
	pending, e := s.PendingFolderMeasures(ctx, 10)
	if e != nil || len(pending) != 2 || pending[0].ShareKey != "team" {
		t.Fatal("new quota folders must be measured by Files", pending, e)
	}
	for _, f := range []string{outer.ID, inner.ID} {
		if e = s.SetFolderMeasured(ctx, f, 0); e != nil {
			t.Fatal(e)
		}
	}
	// Every enclosing folder and the share must fit.
	var q QuotaExceeded
	if _, e = s.ReserveShare(ctx, share.ID, []string{outer.ID, inner.ID}, 6*mib); !errors.As(e, &q) || q.Scope != "folder" {
		t.Fatal("inner folder limit", e)
	}
	r, e := s.ReserveShare(ctx, share.ID, []string{outer.ID, inner.ID}, 4*mib)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Commit(ctx, r, 4*mib); e != nil {
		t.Fatal(e)
	}
	if used, _ := folderUsage(t, s, inner.ID); used != 4*mib {
		t.Fatal(used)
	}
	if used, _ := folderUsage(t, s, outer.ID); used != 4*mib {
		t.Fatal(used)
	}
	// An interrupted write releases its folder reservations too.
	if _, e = s.ReserveShare(ctx, "", []string{outer.ID}, 10*mib); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(ctx, "UPDATE storage_reservations SET expires_at=now()-interval '1 minute'")
	if _, e = s.ReserveShare(ctx, share.ID, nil, 1); e != nil {
		t.Fatal(e)
	}
	if _, reserved := folderUsage(t, s, outer.ID); reserved != 0 {
		t.Fatal("expired folder reservation kept", reserved)
	}
	// Renaming the outer folder rewrites both registrations; usage moves with the data.
	if e = s.MoveFolders(ctx, share.ID, "projects", "archive/projects"); e != nil {
		t.Fatal(e)
	}
	folders, e := s.ShareFolders(ctx, share.ID)
	if e != nil || len(folders) != 2 || folders[0].Path != "archive/projects" || folders[1].Path != "archive/projects/small" || folders[1].ID != inner.ID {
		t.Fatal(folders, e)
	}
	if e = s.AdjustFolders(ctx, []string{inner.ID}, -10*mib); e != nil {
		t.Fatal(e)
	}
	if used, _ := folderUsage(t, s, inner.ID); used != 0 {
		t.Fatal("usage below zero", used)
	}
	list, e := m.QuotaFolders(ctx, share.ID)
	if e != nil || len(list) != 2 {
		t.Fatal(list, e)
	}
	if e = m.SetQuotaFolderLimit(ctx, alice, inner.ID, list[1].Version, nil); e != nil {
		t.Fatal(e)
	}
	if e = m.DeleteQuotaFolder(ctx, alice, inner.ID, list[1].Version); !errors.Is(e, ErrConflict) {
		t.Fatal("stale version", e)
	}
	if e = m.DeleteQuotaFolder(ctx, alice, inner.ID, list[1].Version+1); e != nil {
		t.Fatal(e)
	}
	if folders, _ = s.ShareFolders(ctx, share.ID); len(folders) != 1 {
		t.Fatal(folders)
	}
}

func TestShareMeasureRequest(t *testing.T) {
	s, m := accessFixture(t)
	ctx := context.Background()
	share, e := m.CreateShare(ctx, alice, "team", "Team", "tank", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	homes, _ := s.HomesShare(ctx)
	if e = m.RequestShareMeasure(ctx, alice, homes.ID); !errors.Is(e, ErrInvalid) {
		t.Fatal("homes share total is the sum of homes", e)
	}
	if e = m.RequestShareMeasure(ctx, alice, share.ID); e != nil {
		t.Fatal(e)
	}
	pending, e := s.PendingShareMeasures(ctx, 5)
	if e != nil || len(pending) != 1 || pending[0].Key != "team" {
		t.Fatal(pending, e)
	}
	if e = s.SetShareMeasured(ctx, share.ID, 42); e != nil {
		t.Fatal(e)
	}
	if pending, _ = s.PendingShareMeasures(ctx, 5); len(pending) != 0 {
		t.Fatal(pending)
	}
}
