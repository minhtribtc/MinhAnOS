# Storage Q3 — quota folders and share measurement (2026-10-31)

Follows [Storage Q2](storage-q2.md) and the "subfolder quota = registered quota folder" decision in
[storage model review](storage-model-review.md). Soft quota only (ledger), like homes and shares.

## What changed

| Area | Q3 |
|---|---|
| Schema | `storage` SQL 004: `storage_quota_folders(id, share_id, path)` unique per share, path checked (no empty/`.`/`..` segments); `storage_quota` scope `folder`; `storage_reservations.folder_ids` (expired reservations release their folders too) |
| Where | Ordinary Shared Folders only. The homes share is refused (a home's quota is the home quota). At most 100 per share. Nesting allowed; overcommit allowed (every enclosing scope is still enforced) |
| Create | System Admin registers a path in Storage Manager. The Drive measurer creates the folder if missing (or adopts an existing one) and measures it; until then usage shows "waiting" and writes use the ledger as is |
| Writes | Upload, copy and text save into a share reserve on the share **and** every enclosing quota folder; refusal names scope `folder` (`QUOTA_EXCEEDED`, 507) |
| Move | Bytes moved (walked before the move) are reserved on folders entered and taken off folders left; the share total does not change |
| Trash / restore | Trash takes the bytes off enclosing folders (the share still holds them in its trash); restore adds them back without a limit check (restore is never refused) |
| Rename / move of a quota folder | The registration keeps its ID and usage; paths of it and every quota folder below are rewritten after the filesystem operation |
| Protection | A quota folder, or a folder containing one, cannot go to the trash: `QUOTA_FOLDER_PROTECTED` (409). Removing the quota folder in Storage Manager keeps the files |
| Share measurement | `POST /shares/{id}/measure` (ordinary shares; the homes share total is the sum of homes). The Drive measurer walks files and trash under the share's lock |
| Storage API | `GET/POST /api/v1/storage/shares/{id}/folders`, `PUT /folders/{id}/quota` (If-Match), `DELETE /folders/{id}` (If-Match), `POST /shares/{id}/measure`; all with reauthentication; audit `storage.folder.created|quota|removed`, `storage.share.measure` |
| Files | `GET /api/quota-folders?root=share:<key>` (share read access); the heading shows the innermost quota folder around the current path with its usage |
| Storage Manager | Share card: "Quota folders" panel (add path + limit, set limit, remove), "Re-measure" |

## Limits

- Registered paths follow renames and moves made through Files only; changes made directly on the host
  are not seen until the admin removes and re-adds the quota folder.
- The settle step after a move/rename runs after the filesystem operation under the share write lock; a
  crash between the two leaves a stale path or usage until re-measured (re-add the quota folder).
- Moving homes off the legacy share, quota notices (80/95/100 %) and hard quota on units remain open
  (Jobs contract, Files notices).

## Verification

See [validation](validation.md) (Storage Q3).
