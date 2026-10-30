# Storage model review — Codex DSM + Nextcloud proposal (2026-10-30)

Review only; nothing implemented. Storage Manager, shared folders and quotas are **planned**.

## Current facts (checked)

- Files stores private data at `/data/users/{immutable-user-id}/files` in the Drive Docker volume; every file is
  owned by container UID 10001. No per-user POSIX UID/GID yet (LinuxIdentity is Phase 6).
- No shared folders, no quota enforcement (128 MiB per upload only), no persistent fileId/ACL (contracts planned).
- Host access goes only through the root files agent over a Unix socket; web containers are non-root.
- Test VM 10.1.1.20 is a single ext4 disk inside Proxmox: no ZFS, no project quotas. The Proxmox hosts use ZFS.

## Verdict

Direction is right and should be adopted as the storage target: DSM-style homes + shared folders as filesystem
resources, Nextcloud-style My Files / Shared with Me experience, quota enforced by the filesystem, refquota vs
quota made explicit, capability-declared backends, no scan-based fake quota, Shared Folder Manager as a module
of Storage Manager. Corrections below are required before it becomes a design.

## Required corrections

1. **Privilege boundary.** Dataset/quota/snapshot operations need root. They must be typed operations on a
   restricted storage agent API (create share dataset, set quota, snapshot, scrub), audited, never a generic
   `zfs`/shell passthrough. Web apps never get host root (AGENTS.md).
2. **Keep immutable IDs.** Name home datasets by the immutable storage key (or public UUID), not `user-1001`
   (UIDs are not allocated yet, usernames can change). Migrate by mounting each home dataset at the existing
   `/data/users/{id}` path so Files paths and ownership stay unchanged (ADR/Identity decision).
3. **ACL must follow the same rule as quota.** App-level ACL is fine while Files is the only access path. Before
   SMB/NFS/SSH, permissions must be enforced on the filesystem (POSIX or NFSv4 ACL) with real UID/GID, otherwise
   other protocols bypass them — the same flaw the proposal points out for Nextcloud quota.
4. **Per-user quota without UIDs.** Today all files are UID 10001, so ZFS `userquota` cannot work; personal quota
   must be the per-home dataset quota. Per-user quota inside shared folders waits for Phase 6 identities.
5. **Backend capability matrix.** ZFS: dataset quota/refquota, snapshots. XFS/ext4: project quota (needs
   `prjquota`), no snapshots. Btrfs: qgroups (weaker accounting). Plain ext4 without project quota: report
   "quota unsupported", never emulate. The current test VM falls in this last case; quota tests need a test pool
   (extra disk or file-backed zpool) on a disposable VM.
6. **Quota default.** Use `refquota` for user-visible limits on homes and shares so snapshots do not suddenly
   block writes; show snapshot usage and pool free space separately in Storage Manager.
7. **Shared with Me needs share records.** User-to-user sharing from My Files requires persistent file IDs and
   resource grants (planned file-resource/ACL contracts), not just shared folders.
8. **Trash per share.** Each shared folder needs its own recycle bin counted in that share's quota; `.zfs`
   snapshot browsing can be an extra restore path.
9. **Settings standard.** Snapshot/scrub/alert policies are Storage Manager panels; Files shows the user's quota
   read-only; nothing duplicated (ADR 0007 hybrid ownership).

## Recommended order

1. Phase 5 Files authorization (user permission AND app grant AND resource) — prerequisite for shared folders.
2. Storage abstraction + restricted storage agent API + capability reporting; disposable ZFS test pool.
3. Homes as datasets (mounted at current paths) + home quota; Shared Folders + share quota + per-share trash.
4. Share records / Shared with Me (persistent file IDs).
5. Phase 6 POSIX identities, filesystem ACL mapping, then SMB/NFS; per-user quota inside shares last.

## Decision 2026-10-30: production backends ZFS, ext4 and Btrfs

The user confirmed real installs will run ZFS, ext4 and Btrfs, so storage is a capability-based abstraction with
three drivers from the start (plus "unsupported" for anything else). One install may have volumes of different
types; homes live on one configured home volume, each share chooses its volume.

| Capability | ZFS | Btrfs | ext4 |
|---|---|---|---|
| Unit for a home/share | dataset | subvolume | directory + project ID |
| Hard quota (referenced data) | `refquota` | qgroup referenced limit (quota must be enabled) | project quota (`project`,`quota` features + `prjquota` mount) |
| Usage report | `referenced`/`used` | qgroup show | quotactl / `repquota -P` |
| Snapshots | yes | yes (read-only subvolume snapshot) | no — reported unsupported |
| New unit visible in Drive container | needs mount propagation (`rslave`) or mount under parent | appears as directory | appears as directory |

Rules: same user-visible meaning of quota on every backend (hard limit on the unit's own data, snapshots shown
separately); ext4 project IDs are allocated and stored in PostgreSQL, never reused; Btrfs quota accounting cost is
measured before enabling by default; every operation goes through typed storage-agent calls, never a shell.

Test plan: one extra virtual disk on the disposable test VM, split into a ZFS pool, a Btrfs filesystem and an ext4
filesystem with project quota, so all three drivers run the same contract tests. Loop files on the current 36 GB
root disk (71% used) are a fallback only.
