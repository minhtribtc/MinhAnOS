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
1. Preview viewer: images, PDF, video/audio (Range streaming), text; image thumbnails generated as jobs and cached.
2. Sort by name/size/modified/type and list columns; remember per user.
3. Upload whole folders (picker + drag-drop), keeping the tree.
4. Download several items as one ZIP (streamed).
5. Drag and drop to move into a folder or breadcrumb.
6. Search the whole subtree on the server (name), with paging.
7. Empty trash; trash shows original location and deletion date.
8. Favorites (per user) and a Recent view.

## Phase 2 — collaboration
1. Public links: password, expiry, download-only / upload-only (file drop), revocable, audited.
2. Share a folder with users/groups of the organization (beyond Shared Folders), with read/write.
3. Versions: keep previous versions on overwrite, restore, retention.
4. Tags and comments; per-file activity from `files_events`.

## Phase 3 — integration
1. Resumable chunked upload above 128 MB.
2. WebDAV endpoint (desktop/mobile sync clients).
3. Office editing (OnlyOffice/Collabora via WOPI).
4. External storage (S3/SMB/FTP) as read/write roots.
