package storage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// memVolume is an in-memory Volume for handler tests; noSnapshots simulates a quota-only backend.
type memVolume struct {
	units       map[string]Usage
	snaps       map[string][]Snapshot
	noSnapshots bool
}

func (m *memVolume) Kind() Kind { return ZFS }
func (m *memVolume) Capabilities() Capabilities {
	return Capabilities{Quota: true, Snapshots: !m.noSnapshots}
}
func (m *memVolume) key(class Class, id string) string { return string(class) + "/" + id }
func (m *memVolume) Create(_ context.Context, class Class, id string) (string, error) {
	if e := validUnit(class, id); e != nil {
		return "", e
	}
	if _, ok := m.units[m.key(class, id)]; ok {
		return "", ErrExists
	}
	m.units[m.key(class, id)] = Usage{Available: 100}
	return "/mem/" + m.key(class, id), nil
}
func (m *memVolume) Path(class Class, id string) (string, error) {
	if _, ok := m.units[m.key(class, id)]; !ok {
		return "", ErrNotFound
	}
	return "/mem/" + m.key(class, id), nil
}
func (m *memVolume) SetQuota(_ context.Context, class Class, id string, limit uint64) error {
	u, ok := m.units[m.key(class, id)]
	if !ok {
		return ErrNotFound
	}
	u.Quota = limit
	m.units[m.key(class, id)] = u
	return nil
}
func (m *memVolume) Usage(_ context.Context, class Class, id string) (Usage, error) {
	u, ok := m.units[m.key(class, id)]
	if !ok {
		return Usage{}, ErrNotFound
	}
	return u, nil
}
func (m *memVolume) Snapshot(_ context.Context, class Class, id, name string) error {
	if !snapPattern.MatchString(name) {
		return ErrInvalid
	}
	m.snaps[m.key(class, id)] = append(m.snaps[m.key(class, id)], Snapshot{Name: name})
	return nil
}
func (m *memVolume) Snapshots(_ context.Context, class Class, id string) ([]Snapshot, error) {
	return m.snaps[m.key(class, id)], nil
}
func (m *memVolume) Destroy(context.Context, Class, string) error { return nil }

func call(h http.Handler, method, path, body string) (int, map[string]any) {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAgentTypedOperations(t *testing.T) {
	vol := &memVolume{units: map[string]Usage{}, snaps: map[string][]Snapshot{}}
	h := AgentHandler(map[string]Volume{"tank": vol, "plain": &memVolume{units: map[string]Usage{}, snaps: map[string][]Snapshot{}, noSnapshots: true}}, Owner{})
	if code, out := call(h, "GET", "/v1/volumes", ""); code != 200 || len(out["items"].([]any)) != 2 {
		t.Fatal(code, out)
	}
	if code, out := call(h, "POST", "/v1/volumes/tank/units", `{"class":"homes","id":"u1"}`); code != 201 || out["path"] != "/mem/homes/u1" {
		t.Fatal(code, out)
	}
	for _, c := range []struct {
		method, path, body string
		want               int
		code               string
	}{
		{"POST", "/v1/volumes/tank/units", `{"class":"homes","id":"u1"}`, 409, "STORAGE_EXISTS"},
		{"POST", "/v1/volumes/tank/units", `{"class":"homes","id":"../etc"}`, 400, "STORAGE_INVALID"},
		{"POST", "/v1/volumes/tank/units", `{"class":"homes","id":"u2","path":"/"}`, 400, "STORAGE_INVALID"},
		{"POST", "/v1/volumes/missing/units", `{"class":"homes","id":"u2"}`, 404, "STORAGE_NOT_FOUND"},
		{"PUT", "/v1/volumes/tank/units/homes/u1/quota", `{"bytes":-1}`, 400, "STORAGE_INVALID"},
		{"PUT", "/v1/volumes/tank/units/homes/u1/quota", `{}`, 400, "STORAGE_INVALID"},
		{"PUT", "/v1/volumes/tank/units/homes/u1/quota", `{"bytes":2305843009213693952}`, 400, "STORAGE_INVALID"},
		{"PUT", "/v1/volumes/tank/units/homes/nobody/quota", `{"bytes":1}`, 404, "STORAGE_NOT_FOUND"},
		{"GET", "/v1/volumes/tank/units/other/u1", ``, 400, "STORAGE_INVALID"},
		{"POST", "/v1/volumes/tank/units/homes/u1/snapshots", `{"name":"a b"}`, 400, "STORAGE_INVALID"},
		{"DELETE", "/v1/volumes/tank/units/homes/u1", ``, 405, ""},
	} {
		code, out := call(h, c.method, c.path, c.body)
		if code != c.want || (c.code != "" && out["code"] != c.code) {
			t.Fatalf("%s %s %s: %d %v", c.method, c.path, c.body, code, out)
		}
	}
	if code, out := call(h, "PUT", "/v1/volumes/tank/units/homes/u1/quota", `{"bytes":1048576}`); code != 200 || out["quotaBytes"] != float64(1048576) {
		t.Fatal(code, out)
	}
	if code, _ := call(h, "POST", "/v1/volumes/tank/units/homes/u1/snapshots", `{"name":"daily-1"}`); code != 201 {
		t.Fatal(code)
	}
	if code, out := call(h, "GET", "/v1/volumes/tank/units/homes/u1/snapshots", ""); code != 200 || len(out["items"].([]any)) != 1 {
		t.Fatal(code, out)
	}
	call(h, "POST", "/v1/volumes/plain/units", `{"class":"shares","id":"media"}`)
	if code, out := call(h, "POST", "/v1/volumes/plain/units/shares/media/snapshots", `{"name":"x"}`); code != 409 || out["code"] != "STORAGE_UNSUPPORTED" {
		t.Fatal("snapshot on a volume without snapshots", code, out)
	}
}

func TestParseVolumes(t *testing.T) {
	v, e := ParseVolumes("tank=zfs:mnzfs/minhanos,fast=btrfs:/srv/minhanos/btrfs/minhanos")
	if e != nil || v["tank"].Kind() != ZFS || v["fast"].Kind() != Btrfs {
		t.Fatal(v, e)
	}
	for _, bad := range []string{"tank=zfs:mnzfs", "tank=zfs:pool/../x;rm", "x=btrfs:relative", "x=btrfs:/srv/../etc", "x=ext4:/srv", "Bad=zfs:a/b", "a=zfs:p/a,a=zfs:p/b", "x=btrfs:/"} {
		if _, e = ParseVolumes(bad); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestParseOwner(t *testing.T) {
	if o, e := ParseOwner("10001:10001"); e != nil || !o.Set || o.UID != 10001 {
		t.Fatal(o, e)
	}
	if o, e := ParseOwner(""); e != nil || o.Set {
		t.Fatal(o, e)
	}
	for _, bad := range []string{"0:0", "root:root", "10001", "-1:5"} {
		if _, e := ParseOwner(bad); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}
