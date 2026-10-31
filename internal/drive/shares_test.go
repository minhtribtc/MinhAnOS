package drive

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"minhanos.local/platform/internal/storagecore"
)

// memShares is a ShareRegistry with homes on a ZFS volume and two Shared Folders; units are temp dirs.
type memShares struct {
	noHomes      bool
	reserved     map[string]int64
	folders      []storagecore.FolderScope
	limits       map[string]int64 // folder id -> limit
	used         map[string]int64 // folder id -> used
	measured     map[string]int64 // share id -> measured
	pendingShare bool
}

func (m *memShares) HomesShare(context.Context) (storagecore.HomesShare, error) {
	if m.noHomes {
		return storagecore.HomesShare{}, storagecore.ErrNoHomes
	}
	return storagecore.HomesShare{ID: "h", Key: "homes", Volume: "tank", VolumeKind: "zfs"}, nil
}
func (m *memShares) UserShares(context.Context, string) ([]storagecore.UserShare, error) {
	return []storagecore.UserShare{{ID: "s1", Key: "team", Name: "Team", Volume: "tank", Access: "write"}, {ID: "s2", Key: "docs", Name: "Docs", Volume: "tank", Access: "read"}}, nil
}
func (m *memShares) ReserveShare(_ context.Context, share string, folders []string, n int64) (storagecore.Reservation, error) {
	for _, f := range folders {
		if l, ok := m.limits[f]; ok && m.used[f]+n > l {
			return storagecore.Reservation{}, storagecore.QuotaExceeded{Scope: "folder"}
		}
	}
	m.reserved[share] += n
	return storagecore.Reservation{ID: "r", Share: share, Folders: folders, Bytes: n}, nil
}
func (m *memShares) ShareFolders(context.Context, string) ([]storagecore.FolderScope, error) {
	return m.folders, nil
}
func (m *memShares) MoveFolders(_ context.Context, _, from, to string) error {
	for i, f := range m.folders {
		if f.Path == from || strings.HasPrefix(f.Path, from+"/") {
			m.folders[i].Path = to + strings.TrimPrefix(f.Path, from)
		}
	}
	return nil
}
func (m *memShares) AdjustFolders(_ context.Context, folders []string, delta int64) error {
	for _, f := range folders {
		m.used[f] += delta
	}
	return nil
}
func (m *memShares) PendingFolderMeasures(context.Context, int) ([]storagecore.PendingFolder, error) {
	result := []storagecore.PendingFolder{}
	for _, f := range m.folders {
		if _, ok := m.used[f.ID]; !ok {
			result = append(result, storagecore.PendingFolder{ID: f.ID, ShareID: "s1", ShareKey: "team", Volume: "tank", Path: f.Path})
		}
	}
	return result, nil
}
func (m *memShares) SetFolderMeasured(_ context.Context, id string, n int64) error {
	m.used[id] = n
	return nil
}
func (m *memShares) PendingShareMeasures(context.Context, int) ([]storagecore.PendingShare, error) {
	if !m.pendingShare {
		return nil, nil
	}
	return []storagecore.PendingShare{{ID: "s1", Key: "team", Volume: "tank"}}, nil
}
func (m *memShares) SetShareMeasured(_ context.Context, id string, n int64) error {
	m.measured[id], m.pendingShare = n, false
	return nil
}
func (m *memShares) QuotaFolderUsage(context.Context, string) ([]storagecore.QuotaFolder, error) {
	result := []storagecore.QuotaFolder{}
	for _, f := range m.folders {
		result = append(result, storagecore.QuotaFolder{ID: f.ID, Path: f.Path})
	}
	return result, nil
}

type memUnits struct {
	root    string
	created []string
}

func (m *memUnits) Unit(_ context.Context, volume, class, id string) (storagecore.AgentUnit, error) {
	p := filepath.Join(m.root, volume, class, id)
	if _, e := os.Stat(p); e != nil {
		return storagecore.AgentUnit{}, storagecore.AgentError{Status: 404, Code: "STORAGE_NOT_FOUND"}
	}
	return storagecore.AgentUnit{Path: p}, nil
}
func (m *memUnits) CreateUnit(_ context.Context, volume, class, id string) (storagecore.AgentUnit, error) {
	p := filepath.Join(m.root, volume, class, id)
	m.created = append(m.created, class+"/"+id)
	return storagecore.AgentUnit{Path: p}, os.MkdirAll(p, 0750)
}

func TestShareRootsAndPermissions(t *testing.T) {
	registry := &memShares{reserved: map[string]int64{}}
	units := &memUnits{root: t.TempDir()}
	for _, key := range []string{"team", "docs"} {
		os.MkdirAll(filepath.Join(units.root, "tank", "shares", key), 0750)
	}
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir(), Quota: &memLedger{}, Shares: registry, Units: units}
	w := quotaRequest(t, s, "GET", "/api/roots", "")
	for _, want := range []string{`"id":"personal"`, `"id":"share:team"`, `"access":"read"`, `"homes":true`} {
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Fatal("roots", want, w.Code, w.Body.String())
		}
	}
	if w = quotaRequest(t, s, "POST", "/api/files/upload?root=share:team&path=&name=a.txt", "hello"); w.Code != 201 || registry.reserved["s1"] != 5 {
		t.Fatal("share upload", w.Code, w.Body.String(), registry.reserved)
	}
	if _, e := os.Stat(filepath.Join(units.root, "tank", "shares", "team", "files", "a.txt")); e != nil {
		t.Fatal("share file not in the share unit", e)
	}
	if w = quotaRequest(t, s, "POST", "/api/files/upload?root=share:docs&path=&name=a.txt", "x"); w.Code != 403 || !strings.Contains(w.Body.String(), "SHARE_READ_ONLY") {
		t.Fatal("read-only share accepted a write", w.Code, w.Body.String())
	}
	if w = quotaRequest(t, s, "GET", "/api/files/list?root=share:docs&path=", ""); w.Code != 200 {
		t.Fatal("read share list", w.Code, w.Body.String())
	}
	if w = quotaRequest(t, s, "GET", "/api/files/list?root=share:secret&path=", ""); w.Code != 404 || !strings.Contains(w.Body.String(), "SHARE_NOT_FOUND") {
		t.Fatal("share without permission", w.Code, w.Body.String())
	}
	// The home is a unit on the homes volume, created on first use.
	if w = quotaRequest(t, s, "GET", "/api/files/list?root=personal&path=", ""); w.Code != 200 || len(units.created) != 1 || units.created[0] != "homes/"+strings.Repeat("a", 64) {
		t.Fatal("home unit", w.Code, w.Body.String(), units.created)
	}
}

func TestNoHomesShareYet(t *testing.T) {
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir(), Quota: &memLedger{}, Shares: &memShares{noHomes: true, reserved: map[string]int64{}}, Units: &memUnits{root: t.TempDir()}}
	if w := quotaRequest(t, s, "GET", "/api/roots", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"homes":false`) || strings.Contains(w.Body.String(), `"id":"personal"`) || strings.Contains(w.Body.String(), `"quota"`) {
		t.Fatal("roots without homes", w.Code, w.Body.String())
	}
	if w := quotaRequest(t, s, "GET", "/api/files/list?root=personal&path=", ""); w.Code != 409 || !strings.Contains(w.Body.String(), "STORAGE_NOT_READY") {
		t.Fatal("personal without homes", w.Code, w.Body.String())
	}
}

func TestQuotaFoldersInShare(t *testing.T) {
	registry := &memShares{reserved: map[string]int64{}, limits: map[string]int64{"q1": 4}, used: map[string]int64{}, measured: map[string]int64{}, pendingShare: true,
		folders: []storagecore.FolderScope{{ID: "q1", Path: "projects"}}}
	units := &memUnits{root: t.TempDir()}
	team := filepath.Join(units.root, "tank", "shares", "team")
	os.MkdirAll(filepath.Join(team, "files"), 0750)
	os.WriteFile(filepath.Join(team, "files", "big.txt"), []byte("123456"), 0640)
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir(), Quota: &memLedger{}, Shares: registry, Units: units}
	// The measurer creates (or adopts) the folder and measures it; then the share.
	if e := s.measureShares(context.Background()); e != nil || registry.used["q1"] != 0 || registry.measured["s1"] != 6 {
		t.Fatal("measure", e, registry.used, registry.measured)
	}
	if _, e := os.Stat(filepath.Join(team, "files", "projects")); e != nil {
		t.Fatal("quota folder not created", e)
	}
	if w := quotaRequest(t, s, "POST", "/api/files/upload?root=share:team&path=projects&name=a.txt", "12345"); w.Code != 507 {
		t.Fatal("upload over the folder limit", w.Code, w.Body.String())
	}
	if w := quotaRequest(t, s, "POST", "/api/files/upload?root=share:team&path=&name=a.txt", "12345"); w.Code != 201 {
		t.Fatal("upload outside the folder", w.Code, w.Body.String())
	}
	// Moving 6 bytes into a 4-byte folder is refused; moving 5 out-of-folder bytes in would be too.
	if w := quotaRequest(t, s, "POST", "/api/files/move?root=share:team&path=big.txt", `{"destination":"projects/big.txt"}`); w.Code != 507 {
		t.Fatal("move into a full folder", w.Code, w.Body.String())
	}
	registry.limits["q1"] = 100
	if w := quotaRequest(t, s, "POST", "/api/files/move?root=share:team&path=big.txt", `{"destination":"projects/big.txt"}`); w.Code != 200 {
		t.Fatal("move into the folder", w.Code, w.Body.String())
	}
	if w := quotaRequest(t, s, "POST", "/api/files/trash?root=share:team&path=projects/big.txt", `{}`); w.Code != 200 || registry.used["q1"] != -6 {
		t.Fatal("trash leaves the folder", w.Code, w.Body.String(), registry.used)
	}
	if w := quotaRequest(t, s, "POST", "/api/files/trash?root=share:team&path=projects", `{}`); w.Code != 409 || !strings.Contains(w.Body.String(), "QUOTA_FOLDER_PROTECTED") {
		t.Fatal("quota folder trashed", w.Code, w.Body.String())
	}
	if w := quotaRequest(t, s, "POST", "/api/files/rename?root=share:team&path=projects", `{"name":"work"}`); w.Code != 200 || registry.folders[0].Path != "work" {
		t.Fatal("rename keeps the registration", w.Code, w.Body.String(), registry.folders)
	}
	if w := quotaRequest(t, s, "GET", "/api/quota-folders?root=share:team", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"path":"work"`) {
		t.Fatal("quota folder list", w.Code, w.Body.String())
	}
}
