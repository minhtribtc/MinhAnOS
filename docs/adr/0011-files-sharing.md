# ADR 0011 — Files sharing: grants to people and public links

Status: accepted 2026-10-31. Files roadmap phase 2 items 1–2.

## Context

Files had Shared Folders (an admin-managed share of an organization, storage Q2) but a user could not hand one file
or folder of their own home to a colleague, nor send a link to someone without an account (Kodbox / Nextcloud
parity). Favorites also pointed at stale paths after a delete, move or rename.

## Decision

**Data (files SQL 008, module `files`).**
- `files_grants(id, owner_id, path, dir, grantee_id, permission view|edit, created_at)`, unique per
  (owner, path, grantee), owner ≠ grantee, ≤1000 per owner (an existing grant can always be updated).
- `files_links(id, token, owner_id, path, dir, permission view|edit|drop, password_hash, expires_at, uses,
  last_used_at)`; `edit` and `drop` only for folders; ≤500 per owner. ID and token are 64 random hex characters.
- Only items of the **personal home** are shared (not Shared Folders, not the system root). Paths are logical
  paths of the home; the server checks the item exists and is a regular file or a folder.

**Who may receive a grant.** Both accounts active and members of a common active organization (ADR 0008) — checked
on every read, so leaving the organization or a disabled account hides the grant without deleting it. Recipients are
searched by name or username among those people only (≤20 results); clients see public IDs, never account IDs.

**Opening a grant.** A received grant is the root `shared:<grant id>`. The engine is the owner's home with
`Base` = the parent of the item and `Only` = the item, so nothing outside it can be named (404 FILES_NOT_FOUND).
- view: list, download, zip, text, search, recent. edit on a folder: every write; on a file: save-text only;
  else 403 SHARE_READ_ONLY. Writes take the owner's home lock, count against the owner's quota and are audited
  in the owner's home (root `personal`). Trash of a grant root: 403 FILES_SHARED_NO_TRASH (deleted items go to the
  owner's trash with their full path). No copy jobs in grant roots.

**Public links** at `/s/<token>` (Files origin; the Desktop builds the URL from `FILES_PUBLIC_URL` or its host
and the Files port). API `/api/public/<token>[/unlock|/files/<op>]`, no session:
- view: list, download, archive; edit: also upload and mkdir; drop: upload into the top folder only, nothing listed.
- Password: PBKDF2-SHA256 hash via `authn.Hash`; `unlock` sets an HttpOnly cookie scoped to the link's path for 12 h,
  an HMAC over link ID + hash with a per-process key (a new password or a restart asks again). 20 tries per link and
  IP per 10 minutes, counted apart from logins (`AUTH_RATE_LIMITED`). Writes require a same-origin request (`ORIGIN_INVALID`).
- 404 FILES_LINK_NOT_FOUND (unknown, item gone, account gone), 410 FILES_LINK_EXPIRED, 401
  FILES_LINK_PASSWORD_REQUIRED, 403 FILES_LINK_PASSWORD_WRONG / FILES_LINK_FORBIDDEN. Uploads reserve the owner's
  quota; downloads and uploads count `uses`. The page sends `Referrer-Policy: no-referrer`.

**Paths follow changes.** Before a rename, move, trash or delete, the server computes `[from, to]`; after success
it retargets favorites, grants and links under `from` (to empty = removed) for the owner — or, in a Shared Folder,
favorites of every user. Favorites whose item is gone for any other reason are pruned when the list is read.
Account removal deletes grants (as owner or grantee), links and `shared:` favorites of the account.

## Consequences

- No group grants yet (organizations are the group); a later "teams" model can add `grantee_kind`.
- A link token is a bearer secret in the URL; revoking means deleting the link. Expiry is a date (end of day).
- Sharing a Shared Folder item, versions, and notifications to the recipient are later work (phase 2 items 3–4).
