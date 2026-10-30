package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The storage agent runs as root on the host and listens on a Unix socket readable only by the
// MinhAnOS service group. It exposes typed unit operations; there is no delete and no shell.
// Callers (platform services) authorize users and write the audit before calling it.

var (
	volumeName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	zfsRoot    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*(/[A-Za-z0-9][A-Za-z0-9_.:-]*)+$`)
)

// maxQuota keeps requests far below 2^64 so no backend parses an overflowed value.
const maxQuota = uint64(1) << 60

// ParseVolumes reads "name=kind:root,..." e.g. "tank=zfs:mnzfs/minhanos,fast=btrfs:/srv/minhanos/btrfs/minhanos".
// ZFS roots must be a child dataset (never a whole pool); Btrfs roots must be clean absolute paths.
func ParseVolumes(spec string) (map[string]Volume, error) {
	volumes := map[string]Volume{}
	for _, item := range strings.Split(spec, ",") {
		name, rest, ok := strings.Cut(strings.TrimSpace(item), "=")
		kind, root, ok2 := strings.Cut(rest, ":")
		if !ok || !ok2 || !volumeName.MatchString(name) || volumes[name] != nil {
			return nil, fmt.Errorf("invalid volume entry %q", item)
		}
		switch Kind(kind) {
		case ZFS:
			if !zfsRoot.MatchString(root) {
				return nil, fmt.Errorf("invalid ZFS root %q", root)
			}
			volumes[name] = ZFSVolume{Root: root}
		case Btrfs:
			if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
				return nil, fmt.Errorf("invalid Btrfs root %q", root)
			}
			volumes[name] = BtrfsVolume{Root: root}
		default:
			return nil, fmt.Errorf("unsupported volume kind %q", kind)
		}
	}
	return volumes, nil
}

type volumeInfo struct {
	Name         string       `json:"name"`
	Kind         Kind         `json:"kind"`
	Capabilities Capabilities `json:"capabilities"`
}

type unitInfo struct {
	Volume string `json:"volume"`
	Class  Class  `json:"class"`
	ID     string `json:"id"`
	Path   string `json:"path"`
	Usage
}

// Owner, when set, receives every new unit root (mode 0750) so the unprivileged Files service can use it.
type Owner struct {
	UID, GID int
	Set      bool
}

// ParseOwner reads "uid:gid" (empty = keep root ownership).
func ParseOwner(spec string) (Owner, error) {
	if spec == "" {
		return Owner{}, nil
	}
	u, g, ok := strings.Cut(spec, ":")
	uid, e1 := strconv.Atoi(u)
	gid, e2 := strconv.Atoi(g)
	if !ok || e1 != nil || e2 != nil || uid <= 0 || gid <= 0 {
		return Owner{}, fmt.Errorf("invalid owner %q", spec)
	}
	return Owner{uid, gid, true}, nil
}

func AgentHandler(volumes map[string]Volume, owner Owner) http.Handler {
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
	fail := func(w http.ResponseWriter, r *http.Request, e error) {
		status, code := 500, "STORAGE_FAILED"
		switch {
		case errors.Is(e, ErrInvalid):
			status, code = 400, "STORAGE_INVALID"
		case errors.Is(e, ErrNotFound):
			status, code = 404, "STORAGE_NOT_FOUND"
		case errors.Is(e, ErrExists):
			status, code = 409, "STORAGE_EXISTS"
		case errors.Is(e, errUnsupported):
			status, code = 409, "STORAGE_UNSUPPORTED"
		default:
			log.Printf("storage-agent %s %s failed: %v", r.Method, r.URL.Path, e) // details stay in the host log
		}
		reply(w, status, map[string]string{"code": code})
	}
	decode := func(r *http.Request, v any) error {
		d := json.NewDecoder(io.LimitReader(r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
			return ErrInvalid
		}
		return nil
	}
	unit := func(r *http.Request) (Volume, Class, string, error) {
		v := volumes[r.PathValue("volume")]
		if v == nil {
			return nil, "", "", ErrNotFound
		}
		class, id := Class(r.PathValue("class")), r.PathValue("id")
		if e := validUnit(class, id); e != nil {
			return nil, "", "", e
		}
		return v, class, id, nil
	}
	describe := func(w http.ResponseWriter, r *http.Request, v Volume, class Class, id string, status int) {
		u, e := v.Usage(r.Context(), class, id)
		if e != nil {
			fail(w, r, e)
			return
		}
		path, e := v.Path(class, id)
		if e != nil {
			fail(w, r, e)
			return
		}
		reply(w, status, unitInfo{r.PathValue("volume"), class, id, path, u})
	}

	mux.HandleFunc("GET /v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		items := []volumeInfo{}
		for name, v := range volumes {
			items = append(items, volumeInfo{name, v.Kind(), v.Capabilities()})
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /v1/volumes/{volume}/units", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Class Class  `json:"class"`
			ID    string `json:"id"`
		}
		v := volumes[r.PathValue("volume")]
		if v == nil {
			fail(w, r, ErrNotFound)
			return
		}
		if e := decode(r, &in); e != nil {
			fail(w, r, e)
			return
		}
		path, e := v.Create(r.Context(), in.Class, in.ID)
		if e != nil {
			fail(w, r, e)
			return
		}
		if owner.Set {
			if e = os.Chown(path, owner.UID, owner.GID); e == nil {
				e = os.Chmod(path, 0750)
			}
			if e != nil {
				fail(w, r, e)
				return
			}
		}
		log.Printf("storage-agent created %s/%s/%s", r.PathValue("volume"), in.Class, in.ID)
		describe(w, r, v, in.Class, in.ID, 201)
	})
	mux.HandleFunc("GET /v1/volumes/{volume}/units/{class}/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, class, id, e := unit(r)
		if e != nil {
			fail(w, r, e)
			return
		}
		describe(w, r, v, class, id, 200)
	})
	mux.HandleFunc("PUT /v1/volumes/{volume}/units/{class}/{id}/quota", func(w http.ResponseWriter, r *http.Request) {
		v, class, id, e := unit(r)
		if e != nil {
			fail(w, r, e)
			return
		}
		var in struct {
			Bytes *uint64 `json:"bytes"`
		}
		if e = decode(r, &in); e != nil || in.Bytes == nil || *in.Bytes > maxQuota {
			fail(w, r, ErrInvalid)
			return
		}
		if !v.Capabilities().Quota {
			fail(w, r, errUnsupported)
			return
		}
		if e = v.SetQuota(r.Context(), class, id, *in.Bytes); e != nil {
			fail(w, r, e)
			return
		}
		log.Printf("storage-agent quota %s/%s/%s=%d", r.PathValue("volume"), class, id, *in.Bytes)
		describe(w, r, v, class, id, 200)
	})
	mux.HandleFunc("GET /v1/volumes/{volume}/units/{class}/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		v, class, id, e := unit(r)
		if e != nil {
			fail(w, r, e)
			return
		}
		if !v.Capabilities().Snapshots {
			reply(w, 200, map[string]any{"items": []Snapshot{}})
			return
		}
		items, e := v.Snapshots(r.Context(), class, id)
		if e != nil {
			fail(w, r, e)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /v1/volumes/{volume}/units/{class}/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		v, class, id, e := unit(r)
		if e != nil {
			fail(w, r, e)
			return
		}
		var in struct {
			Name string `json:"name"`
		}
		if e = decode(r, &in); e != nil {
			fail(w, r, e)
			return
		}
		if !v.Capabilities().Snapshots {
			fail(w, r, errUnsupported)
			return
		}
		if e = v.Snapshot(r.Context(), class, id, in.Name); e != nil {
			fail(w, r, e)
			return
		}
		log.Printf("storage-agent snapshot %s/%s/%s@%s", r.PathValue("volume"), class, id, in.Name)
		reply(w, 201, map[string]string{"name": in.Name})
	})
	return mux
}

var errUnsupported = errors.New("operation not supported by this volume")
