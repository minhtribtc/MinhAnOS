# Storage layer S1 — drivers and restricted storage agent (2026-10-30)

Decision: managed data volumes on **ZFS and Btrfs**; ext4 only as the system disk
([review](storage-model-review.md)). The user proceeded by adding two test disks, which was taken as
confirmation. Nothing user-visible yet: Files still stores data in its Docker volume.

## Code

- `internal/storage`: `Volume` interface (Create, Path, SetQuota, Usage, Snapshot, Snapshots, Destroy) with
  `ZFSVolume` (dataset per unit, `refquota`, `zfs snapshot`) and `BtrfsVolume` (subvolume per unit, qgroup
  referenced limit, read-only snapshots in `<root>/.snapshots/<class>/<id>/<name>`). Units are
  `homes/<id>` or `shares/<id>`; IDs `^[a-z0-9][a-z0-9_-]{0,127}$`, snapshot names `^[a-z0-9][a-z0-9_.-]{0,63}$`.
  Commands run with `exec` and fixed arguments, never a shell.
- `cmd/storage-agent`: root host service on `/run/minhanos-storage/agent.sock` (root:10001, 0660), built into
  the Drive image as `/app/storage-agent` like the files agent. Typed HTTP over the socket:

| Method | Path | Purpose |
|---|---|---|
| GET | `/v1/volumes` | configured volumes and capabilities |
| POST | `/v1/volumes/{volume}/units` | create `{class,id}` → path and usage |
| GET | `/v1/volumes/{volume}/units/{class}/{id}` | path and usage |
| PUT | `/v1/volumes/{volume}/units/{class}/{id}/quota` | `{bytes}` (0 = none, max 2^60) |
| GET/POST | `/v1/volumes/{volume}/units/{class}/{id}/snapshots` | list / create `{name}` |

  No delete endpoint. Errors are codes only (`STORAGE_INVALID|NOT_FOUND|EXISTS|UNSUPPORTED|FAILED`); command
  stderr stays in the host journal. Callers must authorize the user and audit before calling.

## Install (test VM 10.1.1.20 as done)

1. `contrib` enabled; `linux-headers-$(uname -r) zfs-dkms zfsutils-linux btrfs-progs` (ZFS 2.1.11, Btrfs 6.2).
2. ZFS: `zpool create -o ashift=12 -O compression=lz4 -O atime=off -O xattr=sa -O acltype=posixacl
   -O mountpoint=/srv/minhanos/zfs mnzfs /dev/disk/by-id/...scsi1` (10 GB), then `zfs create mnzfs/minhanos`.
3. Btrfs: `mkfs.btrfs -L mnbtrfs /dev/disk/by-id/...scsi2` (11 GB), fstab by UUID at `/srv/minhanos/btrfs`
   (`noatime,compress=zstd:3`), `btrfs quota enable`, root dir `/srv/minhanos/btrfs/minhanos`.
4. `/etc/minhanos/storage-agent.env` (0600), plus `MINHANOS_STORAGE_OWNER=10001:10001` so new units belong to the service account:
   `MINHANOS_STORAGE_VOLUMES=tank=zfs:mnzfs/minhanos,fast=btrfs:/srv/minhanos/btrfs/minhanos`.
5. Binary to `/usr/local/libexec/minhanos-storage-agent`; unit `deploy/minhanos-storage-agent.service`.

The unit deliberately has **no** `PrivateTmp`, `Protect*` or `ProtectSystem`: those create a private mount
namespace and ZFS datasets created by the agent were then mounted only inside it (verified: invisible on the
host, "dataset is busy"). Remaining hardening: `NoNewPrivileges`, `RestrictAddressFamilies`, `UMask`, socket
permissions.

## Verification

Contract test `internal/storage` (`MINHANOS_TEST_ZFS_ROOT`, `MINHANOS_TEST_BTRFS_ROOT`, run as root on the VM)
PASS on both backends: create/duplicate, quota 20 MiB with 8 MiB random data, 30 MiB write refused with
EDQUOT, quota removal, snapshot create/list/duplicate/invalid, destroy. Random data is required because both
volumes compress. Agent handler tests and volume-spec parsing tests PASS. Live agent smoke over the socket on
both volumes PASS; test units removed afterwards.

## Next (S2)

Homes as units mounted at the existing `/data/users/{id}` path (with migration of current data), quota and
usage in Files and Account Center/Storage Manager, storage permissions and audit in the platform, then Shared
Folders (S3).
