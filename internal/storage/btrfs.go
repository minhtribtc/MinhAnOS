package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// BtrfsVolume keeps every unit as a subvolume below Root (a mounted Btrfs path with quota enabled).
// Quota is the referenced limit of the subvolume's level-0 qgroup; read-only snapshots live in
// Root/.snapshots/<class>/<id>/<name>, outside the unit so users cannot change them.
type BtrfsVolume struct{ Root string }

func (b BtrfsVolume) Kind() Kind                 { return Btrfs }
func (b BtrfsVolume) Capabilities() Capabilities { return Capabilities{Quota: true, Snapshots: true} }

func (b BtrfsVolume) Path(class Class, id string) (string, error) {
	if e := validUnit(class, id); e != nil {
		return "", e
	}
	return filepath.Join(b.Root, string(class), id), nil
}

// subvolume reports whether path is the root of a Btrfs subvolume (inode 256).
func subvolume(path string) bool {
	st, e := os.Lstat(path)
	if e != nil || !st.IsDir() {
		return false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && sys.Ino == 256
}

func (b BtrfsVolume) unit(class Class, id string) (string, error) {
	p, e := b.Path(class, id)
	if e != nil {
		return "", e
	}
	if !subvolume(p) {
		return "", ErrNotFound
	}
	return p, nil
}

func (b BtrfsVolume) Create(ctx context.Context, class Class, id string) (string, error) {
	p, e := b.Path(class, id)
	if e != nil {
		return "", e
	}
	if _, e = os.Lstat(p); e == nil {
		return "", ErrExists
	}
	if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return "", e
	}
	if _, e = run(ctx, "btrfs", "subvolume", "create", p); e != nil {
		return "", e
	}
	return p, nil
}

func (b BtrfsVolume) SetQuota(ctx context.Context, class Class, id string, limit uint64) error {
	p, e := b.unit(class, id)
	if e != nil {
		return e
	}
	value := "none"
	if limit > 0 {
		value = strconv.FormatUint(limit, 10)
	}
	_, e = run(ctx, "btrfs", "qgroup", "limit", value, p)
	return e
}

func (b BtrfsVolume) Usage(ctx context.Context, class Class, id string) (Usage, error) {
	p, e := b.unit(class, id)
	if e != nil {
		return Usage{}, e
	}
	out, e := run(ctx, "btrfs", "qgroup", "show", "--sync", "--raw", "-re", "-f", p)
	if e != nil {
		return Usage{}, e
	}
	// Row: 0/<id> <referenced> <exclusive> <max referenced|none> <max exclusive|none> <path>
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.HasPrefix(f[0], "0/") {
			continue
		}
		var u Usage
		u.Referenced, _ = strconv.ParseUint(f[1], 10, 64)
		if f[3] != "none" {
			u.Quota, _ = strconv.ParseUint(f[3], 10, 64)
		}
		var fs syscall.Statfs_t
		if e = syscall.Statfs(p, &fs); e != nil {
			return Usage{}, e
		}
		u.Available = fs.Bavail * uint64(fs.Bsize)
		if u.Quota > 0 {
			left := uint64(0)
			if u.Quota > u.Referenced {
				left = u.Quota - u.Referenced
			}
			u.Available = min(u.Available, left)
		}
		return u, nil
	}
	return Usage{}, errors.New("btrfs qgroup not found; is quota enabled on the volume?")
}

func (b BtrfsVolume) snapshotDir(class Class, id string) string {
	return filepath.Join(b.Root, ".snapshots", string(class), id)
}

func (b BtrfsVolume) Snapshot(ctx context.Context, class Class, id, name string) error {
	p, e := b.unit(class, id)
	if e != nil {
		return e
	}
	if !snapPattern.MatchString(name) {
		return ErrInvalid
	}
	dir := b.snapshotDir(class, id)
	if _, e = os.Lstat(filepath.Join(dir, name)); e == nil {
		return ErrExists
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	_, e = run(ctx, "btrfs", "subvolume", "snapshot", "-r", p, filepath.Join(dir, name))
	return e
}

func (b BtrfsVolume) Snapshots(ctx context.Context, class Class, id string) ([]Snapshot, error) {
	if _, e := b.unit(class, id); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(b.snapshotDir(class, id))
	if errors.Is(e, os.ErrNotExist) {
		return []Snapshot{}, nil
	}
	if e != nil {
		return nil, e
	}
	result := []Snapshot{}
	for _, entry := range entries {
		path := filepath.Join(b.snapshotDir(class, id), entry.Name())
		if !subvolume(path) {
			continue
		}
		out, e := run(ctx, "btrfs", "subvolume", "show", path)
		if e != nil {
			return nil, e
		}
		snap := Snapshot{Name: entry.Name()}
		for _, line := range strings.Split(out, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Creation time:"); ok {
				if t, e := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(v)); e == nil {
					snap.Created = t.UTC()
				}
			}
		}
		result = append(result, snap)
	}
	return result, nil
}

func (b BtrfsVolume) Destroy(ctx context.Context, class Class, id string) error {
	p, e := b.unit(class, id)
	if e != nil {
		return e
	}
	snaps, _ := os.ReadDir(b.snapshotDir(class, id))
	for _, s := range snaps {
		if _, e = run(ctx, "btrfs", "subvolume", "delete", filepath.Join(b.snapshotDir(class, id), s.Name())); e != nil {
			return e
		}
	}
	_ = os.Remove(b.snapshotDir(class, id))
	_, e = run(ctx, "btrfs", "subvolume", "delete", p)
	return e
}
