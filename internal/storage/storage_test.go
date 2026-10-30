package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNamesRejected(t *testing.T) {
	for _, v := range []Volume{ZFSVolume{Root: "pool/x"}, BtrfsVolume{Root: "/nonexistent"}} {
		for _, id := range []string{"", "../etc", "a/b", "UPPER", "with space", "-dash", "a\x00b"} {
			if _, e := v.Path(Homes, id); !errors.Is(e, ErrInvalid) {
				t.Fatalf("%s accepted %q", v.Kind(), id)
			}
		}
		if _, e := v.Path("other", "abc"); !errors.Is(e, ErrInvalid) {
			t.Fatal("unknown class accepted")
		}
	}
}

// Contract tests run against real volumes on a disposable machine, as root:
// MINHANOS_TEST_ZFS_ROOT=<pool>/<dataset> and/or MINHANOS_TEST_BTRFS_ROOT=<path on a btrfs mount with quota>.
func volumesUnderTest(t *testing.T) []Volume {
	var vs []Volume
	if root := os.Getenv("MINHANOS_TEST_ZFS_ROOT"); root != "" {
		vs = append(vs, ZFSVolume{Root: root})
	}
	if root := os.Getenv("MINHANOS_TEST_BTRFS_ROOT"); root != "" {
		vs = append(vs, BtrfsVolume{Root: root})
	}
	if len(vs) == 0 {
		t.Skip("no test volume configured")
	}
	return vs
}

// writeRandom writes incompressible data: both test volumes compress, so zeros would not count.
func writeRandom(path string, size int64) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	_, e = io.CopyN(f, rand.Reader, size)
	if e == nil {
		e = f.Sync()
	}
	if c := f.Close(); e == nil {
		e = c
	}
	return e
}

func TestVolumeContract(t *testing.T) {
	ctx := context.Background()
	for _, v := range volumesUnderTest(t) {
		t.Run(string(v.Kind()), func(t *testing.T) {
			id := "contract-" + time.Now().Format("150405")
			path, e := v.Create(ctx, Homes, id)
			if e != nil {
				t.Fatal(e)
			}
			defer v.Destroy(ctx, Homes, id)
			if _, e = v.Create(ctx, Homes, id); !errors.Is(e, ErrExists) {
				t.Fatal("duplicate create", e)
			}
			if p, _ := v.Path(Homes, id); p != path {
				t.Fatal("path mismatch", p, path)
			}
			if !v.Capabilities().Quota {
				t.Fatal("ZFS and Btrfs must support quota")
			}
			const mib = 1 << 20
			if e = v.SetQuota(ctx, Homes, id, 20*mib); e != nil {
				t.Fatal(e)
			}
			if e = writeRandom(filepath.Join(path, "ok"), 8*mib); e != nil {
				t.Fatal("write under quota", e)
			}
			u, e := v.Usage(ctx, Homes, id)
			if e != nil || u.Quota != 20*mib || u.Referenced < 7*mib || u.Available > 13*mib {
				t.Fatalf("usage %+v %v", u, e)
			}
			// Both backends refuse the write with EDQUOT near the limit (verified on ZFS 2.1 and Btrfs 6.2).
			over := writeRandom(filepath.Join(path, "over"), 30*mib)
			u, _ = v.Usage(ctx, Homes, id)
			if over == nil || u.Referenced > 22*mib {
				t.Fatalf("30 MiB under a 20 MiB quota must fail: %v %+v", over, u)
			}
			os.Remove(filepath.Join(path, "over"))
			if e = v.SetQuota(ctx, Homes, id, 0); e != nil {
				t.Fatal(e)
			}
			if u, e = v.Usage(ctx, Homes, id); e != nil || u.Quota != 0 {
				t.Fatalf("quota not removed %+v %v", u, e)
			}
			if e = v.Snapshot(ctx, Homes, id, "before-change"); e != nil {
				t.Fatal(e)
			}
			if e = v.Snapshot(ctx, Homes, id, "before-change"); !errors.Is(e, ErrExists) {
				t.Fatal("duplicate snapshot", e)
			}
			if e = v.Snapshot(ctx, Homes, id, "../x"); !errors.Is(e, ErrInvalid) {
				t.Fatal("bad snapshot name", e)
			}
			snaps, e := v.Snapshots(ctx, Homes, id)
			if e != nil || len(snaps) != 1 || snaps[0].Name != "before-change" || time.Since(snaps[0].Created) > time.Hour {
				t.Fatalf("snapshots %+v %v", snaps, e)
			}
			if e = v.Destroy(ctx, Homes, id); e != nil {
				t.Fatal(e)
			}
			if _, e = v.Usage(ctx, Homes, id); !errors.Is(e, ErrNotFound) {
				t.Fatal("destroyed unit still reported", e)
			}
			if e = v.SetQuota(ctx, Homes, "missing-unit", mib); !errors.Is(e, ErrNotFound) {
				t.Fatal("missing unit", e)
			}
		})
	}
}
