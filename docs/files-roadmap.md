# Files roadmap — parity with Kodbox / Nextcloud Files (2026-10-31)

Gap list checked against the code (FilesApp.vue, internal/drive). Each phase ships with Go tests, smoke and browser
tests, vi-VN/en-US catalogs and app-standard rows.

## Done
- Layout: sidebar, list and details scroll independently; selection actions live in the fixed path row; notices float
  (`tests/files-layout-browser.mjs`, desktop + phone).
- Existing: homes, Shared Folders by organization, system root, trash/restore, grid/list, search in folder, multi-select,
  context menu, mkdir/rename/move/copy (server jobs)/delete, upload with progress (≤128 MB), download, text editor,
  details, quota, transfers panel, keyboard shortcuts, export (.tar).

## Phase 1 — daily use
Shipped 2026-10-31 except item 1 (preview, thumbnails and format editors become separate modules):
1. Preview viewer: images, PDF, video/audio (Range streaming), text; image thumbnails generated as jobs and cached — **separate module, later**.
2. Done: sort by name/size/modified/type, ascending/descending, folders first; remembered in the browser.
3. Done: upload whole folders (picker + drag-drop), keeping the tree and empty folders; existing folders are reused.
4. Done: download a folder or several items as one ZIP streamed by the server (`GET files/archive?item=…`, ≤1000 items).
5. Done: drag and drop to move into a folder or breadcrumb (same root; never into itself).
6. Done: search the whole subtree on the server by name (`GET files/search`, ≤200 results, walk ≤100k entries, `truncated`).
7. Done: empty trash and delete one trash item permanently (`empty-trash`, `trash-delete`, freed bytes, quota re-measured); trash shows original location and deletion date.
8. Done: favorites per user across roots (files SQL 007, `/api/favorites`, ≤500, removed with the account) and a Recent view (`GET files/recent`, 50 newest files).

## Phase 2 — collaboration
Items 1–2 shipped 2026-10-31 (ADR 0011, files SQL 008):
1. Done: public links `/s/<token>` — view, edit (upload + new folder) or drop (upload only), optional password and expiry, revocable, use count, uploads on the owner's quota and audited.
2. Done: share a file or folder of the home with people of a common organization, view or edit (`shared:<id>` roots, "Shared with me" / "Shared by me"); favorites, grants and links follow renames and moves, and go with deletes. Groups beyond the organization: later.
3. Versions: keep previous versions on overwrite, restore, retention.
4. Tags and comments; per-file activity from `files_events`.

Activity log (ADR 0012, files SQL 009, shipped 2026-10-31): levels minimal / writes / access / full chosen in central Settings; reads, refusals and public-link visitors recorded from `access`; client, duration, bytes and live presence ("In Files now" in Log Center) at `full`. Next: per-file history in the inspector, per-person timeline, CSV export, alerts on mass delete or download.

### Activity log — next steps (planned)
Building on ADR 0012; each step reads `files_events` and needs no new recording unless stated.
1. Per-file history in the inspector: who opened, edited, downloaded, shared the selected item (System Admin; the owner sees their own item from level `writes`).
2. Per-person timeline in Log Center: one person's Files activity by day, folders visited, bytes moved.
3. CSV export of a filtered Log Center view (audited `logs.export`, bounded rows).
4. Alerts: mass delete, mass download or many refusals by one person in a short window → security notification to System Admins (thresholds in Settings).
5. Retention per level (e.g. reads 90 days, writes 365 days) chosen in the Files activity panel.
6. Record access through WebDAV/SMB/NFS with the same levels once those services exist.
7. Optional: sign-in context in events (session ID, device name) and a "who still has it open" view per folder for admins.

## Phase 3 — integration
1. Resumable chunked upload above 128 MB.
2. WebDAV endpoint (desktop/mobile sync clients).
3. Office editing (OnlyOffice/Collabora via WOPI).
4. External storage (S3/SMB/FTP) as read/write roots.
