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

## Phase 3 — integration
1. Resumable chunked upload above 128 MB.
2. WebDAV endpoint (desktop/mobile sync clients).
3. Office editing (OnlyOffice/Collabora via WOPI).
4. External storage (S3/SMB/FTP) as read/write roots.
