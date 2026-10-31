# Storage Q2 — share permissions, share roots in Files, homes share (2026-10-31)

Builds on [Storage Q1](storage-quota.md) and the first-run rule in [Files review](files-app-review.md).
**Quota folders are not in Q2; they move to Q3.**

## What changed

| Area | Q2 |
|---|---|
| Schema | `storage` SQL 003: `storage_share_access(share_id, subject_type user/group, subject_id = identity public ID, access read/write/deny)`; unique index so at most one share has `kind='homes'` |
| Effective access | Entries for the user and their groups; **deny > write > read**; no entry = no access. The homes share has no share permissions (each home is private to its owner) |
| Storage API | `GET/PUT /api/v1/storage/shares/{id}/access` (PUT replaces the list, If-Match on the share version, reauthentication), `GET /api/v1/storage/subjects` (active accounts and groups, 500 each), `hostsHomes` on `POST /shares`. Codes `STORAGE_HOMES_EXISTS` (409), `STORAGE_NOT_READY` (409). Audit `storage.share.access` stores counts, not the list |
| Create share | The creator gets `write` (audited, editable). A share that hosts homes needs no unit of its own |
| Files roots | `/api/roots` returns `{id, kind home/share/system, name, access}` and `homes`. Shared Folders are `share:<key>` |
| Files requests | `root=share:<key>`: Files grant (`files.read`/`files.write`) AND share access (write for every non-GET); `SHARE_NOT_FOUND` 404 (no entry or deny), `SHARE_READ_ONLY` 403. Lock per share, own `trash/`, quota on the share ledger (`ReserveShare`) |
| Homes | Legacy directory share → `DataDir/users/<storage ID>` (unchanged). Homes share on ZFS/Btrfs → unit `homes/<storage ID>` on that volume, created through the storage agent on first use; Files reaches it through the `/srv/minhanos` bind mount (rslave) |
| First run | Drive start retires the legacy homes registration when `DataDir/users` is empty and no home has usage (event `storage.homes.legacy_retired`). Until a share hosts homes: no "My files" root, personal requests `STORAGE_NOT_READY`, Files shows "No storage yet", Storage Manager shows a banner and the "Hosts user homes" option |
| Agent | After creating a unit it sets the class directory (`homes/`, `shares/`) to `root:<service gid>` 0750 so the service can traverse it |
| UI | Storage Manager: Permissions dialog (add account/group, read/write/deny, remove), "Hosts user homes" checkbox while none exists. Files: share icon, "View only" badge, write actions disabled for read access, trash and restore of the current root |

## Decisions

- Share keys are the ids in Files (`share:<key>`); keys never change after creation.
- Subjects are identity public IDs, checked when the list is saved. Deleting an account or group leaves its rows
  inert (the query only matches active accounts and existing memberships); cleanup belongs to the identity
  deletion flow later.
- A share without entries is invisible in Files. `share2` on the test VM was created before Q2 and has no
  entries until an admin adds them.
- Share usage is ledger-only (writes through Files); no measurer walks shares yet.
- Moving homes from the legacy share to a new homes share is a job for later (Jobs contract, row 20).

## Limits / next (Q3)

Quota folders and share measurement shipped in Q3 ([storage Q3](storage-q3.md)). Still open: moving homes; hard quota on units (agent supports it); share notices and
Files audit in the Log Center; per-folder ACL inside a share is not planned.

## Verification

See [validation](validation.md) (Storage Q2).
