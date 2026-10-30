# Storage Q1 — registry, soft quota and Storage Manager (2026-10-31)

Implements Q1 of [storage review](storage-model-review.md): application-level (soft) quota first, ZFS/Btrfs
hard quota later. Builds on [storage agent S1](storage-agent.md).

## Decisions taken while implementing

- **No data migration in Q1.** The existing Files data directory is registered as volume `files`
  (kind `directory`) and its per-user areas form the default `homes` share. Moving homes onto ZFS/Btrfs is a
  later explicit job (with backup). This keeps ext4 data where it is (soft quota needs no filesystem support).
- **Usage ledger, not a file index.** One row per scope (`home` = user storage key, `share`) with
  `used_bytes`, `reserved_bytes`, `limit_bytes` (NULL = unlimited). No file-metadata table.
- **Home writes count against the home and the homes share.** Both must fit, atomically.

## Schema (`storage` module, migration 001)

`storage_volumes`, `storage_shares` (key, name, volume, kind `share|homes`, version), `storage_quota`
(ledger), `storage_reservations` (in-flight writes with 15-minute expiry), `storage_settings`
(`default_home_quota_bytes`), `storage_events` (append-only audit with request/correlation IDs).
Identity SQL 010 adds `storage.manage` (System Admin, app `com.minhanos.storage`).

## Enforcement in Files (personal homes)

Under the user's write lock: measure the home if the ledger asks (first use or admin "re-measure"), reserve
the bytes the operation may add, run it, then commit the actual bytes or release on failure (settled with a
fresh context even if the client disconnected).

| Operation | Reserved | Committed |
|---|---|---|
| upload | request body length (raw bytes, ≤128 MiB) | same |
| copy | source file size | same |
| save-text | 256 KiB upper bound | new file size (old version stays as backup) |
| mkdir, rename, move, trash, restore | nothing | — |

Reservation SQL is `used + reserved + n <= limit` per scope inside one transaction, so concurrent writes can
never pass a limit together (tested with 12 parallel reservations). Expired reservations are released by the
next reservation; a late commit after expiry adds usage without subtracting twice. Over quota → `507
QUOTA_EXCEEDED`. Personal files have no permanent delete yet, so usage only grows; trash counts until purged.

Limits: **application-enforced** — valid while Files is the only write path. The host-operator system root is
outside quota; SMB/NFS/WebDAV/SSH (not offered) would bypass it until optional hard quota (Q4).

## Storage Manager

App `com.minhanos.storage` (System Admin only; hidden from others in the launcher). Tabs: Shared folders
(create on a ZFS/Btrfs volume through the storage agent, total quota, logical vs on-disk size), User homes
(usage, quota, re-measure), Volumes (kind, online, snapshots). Every write asks for the admin's password
(reauthentication) and is audited. Settings → Storage Manager → Default quota sets the quota for homes created
later. Files shows "Your storage" with a usage bar.

API `/api/storage/...` (Desktop host, session + CSRF, `storage.manage`, writes need reauthentication within
5 minutes): `GET volumes|shares|homes|settings`, `POST shares`, `PUT shares/{id}/quota` (with `version`),
`PUT homes/{userId}/quota`, `POST homes/{userId}/measure`, `PUT settings`.

The storage agent now gives new units to the service account (`MINHANOS_STORAGE_OWNER=10001:10001`, mode 0750).
Shares are created but not yet browsable in File Station (Q2: share permissions and roots).

## Verification

See [validation](validation.md).
