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
	noHomes  bool
	reserved map[string]int64
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
func (m *memShares) ReserveShare(_ context.Context, share string, n int64) (storagecore.Reservation, error) {
	m.reserved[share] += n
	return storagecore.Reservation{ID: "r", Share: share, Bytes: n}, nil
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
