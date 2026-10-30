package storage

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ZFSVolume keeps every unit as its own dataset below Root (e.g. "mnzfs/minhanos"), so quota
// (refquota: the dataset's own data, not snapshots) and snapshots are per unit.
type ZFSVolume struct{ Root string }

func (z ZFSVolume) Kind() Kind                 { return ZFS }
func (z ZFSVolume) Capabilities() Capabilities { return Capabilities{Quota: true, Snapshots: true} }

func (z ZFSVolume) dataset(class Class, id string) (string, error) {
	if e := validUnit(class, id); e != nil {
		return "", e
	}
	return z.Root + "/" + string(class) + "/" + id, nil
}

func (z ZFSVolume) exists(ctx context.Context, ds string) bool {
	_, e := run(ctx, "zfs", "list", "-H", "-o", "name", ds)
	return e == nil
}

func (z ZFSVolume) Create(ctx context.Context, class Class, id string) (string, error) {
	ds, e := z.dataset(class, id)
	if e != nil {
		return "", e
	}
	if z.exists(ctx, ds) {
		return "", ErrExists
	}
	if _, e = run(ctx, "zfs", "create", "-p", ds); e != nil {
		return "", e
	}
	return z.Path(class, id)
}

func (z ZFSVolume) Path(class Class, id string) (string, error) {
	ds, e := z.dataset(class, id)
	if e != nil {
		return "", e
	}
	out, e := run(context.Background(), "zfs", "get", "-H", "-o", "value", "mountpoint", ds)
	if e != nil {
		return "", ErrNotFound
	}
	if !filepath.IsAbs(out) {
		return "", ErrNotFound
	}
	return out, nil
}

func (z ZFSVolume) SetQuota(ctx context.Context, class Class, id string, limit uint64) error {
	ds, e := z.dataset(class, id)
	if e != nil {
		return e
	}
	if !z.exists(ctx, ds) {
		return ErrNotFound
	}
	value := "none"
	if limit > 0 {
		value = strconv.FormatUint(limit, 10)
	}
	_, e = run(ctx, "zfs", "set", "refquota="+value, ds)
	return e
}

func (z ZFSVolume) Usage(ctx context.Context, class Class, id string) (Usage, error) {
	ds, e := z.dataset(class, id)
	if e != nil {
		return Usage{}, e
	}
	out, e := run(ctx, "zfs", "get", "-Hp", "-o", "value", "referenced,refquota,available", ds)
	if e != nil {
		return Usage{}, ErrNotFound
	}
	f := strings.Fields(out)
	if len(f) != 3 {
		return Usage{}, ErrNotFound
	}
	var u Usage
	u.Referenced, _ = strconv.ParseUint(f[0], 10, 64)
	u.Quota, _ = strconv.ParseUint(f[1], 10, 64) // "0" when none
	u.Available, _ = strconv.ParseUint(f[2], 10, 64)
	return u, nil
}

func (z ZFSVolume) Snapshot(ctx context.Context, class Class, id, name string) error {
	ds, e := z.dataset(class, id)
	if e != nil || !snapPattern.MatchString(name) {
		return ErrInvalid
	}
	if !z.exists(ctx, ds) {
		return ErrNotFound
	}
	if z.exists(ctx, ds+"@"+name) {
		return ErrExists
	}
	_, e = run(ctx, "zfs", "snapshot", ds+"@"+name)
	return e
}

func (z ZFSVolume) Snapshots(ctx context.Context, class Class, id string) ([]Snapshot, error) {
	ds, e := z.dataset(class, id)
	if e != nil {
		return nil, e
	}
	out, e := run(ctx, "zfs", "list", "-Hp", "-t", "snapshot", "-o", "name,creation", "-d", "1", ds)
	if e != nil {
		return nil, ErrNotFound
	}
	result := []Snapshot{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		_, name, _ := strings.Cut(f[0], "@")
		secs, _ := strconv.ParseInt(f[1], 10, 64)
		result = append(result, Snapshot{Name: name, Created: time.Unix(secs, 0).UTC()})
	}
	return result, nil
}

func (z ZFSVolume) Destroy(ctx context.Context, class Class, id string) error {
	ds, e := z.dataset(class, id)
	if e != nil {
		return e
	}
	if !z.exists(ctx, ds) {
		return ErrNotFound
	}
	_, e = run(ctx, "zfs", "destroy", "-r", ds)
	return e
}
