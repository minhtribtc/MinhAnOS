# Changelog

## 0.2.0 — 2026-10-30

- Rebuilt the shell with an iPadOS-inspired Home, original abstract wallpaper, large app icons, floating Dock and Spotlight-style app search.
- Added live API/process widgets, private note preview, app switcher and Control Center with persistent light/dark settings.
- Kept app instances mounted across Home, minimization and switching. Explicit close still discards unsaved local forms.
- Retained optional draggable/resizable window mode and existing backend authentication/API contracts.
- Added mandatory agent handoff/documentation rules in AGENTS.md.

This is interface V2. Split View, durable background jobs, Terminal and service integrations remain planned.

## Unreleased — File Station design

- Added two interactive File Station design variants and a specification for UI, storage boundaries and task lifecycle.
- Design only; no deployed file operations or API changes.

## Drive 0.1 — 2026-10-30

- Added independently runnable Drive frontend/backend, login/session and standalone Compose.
- Added real private file operations, upload/download, rename/move, trash/restore and UTF-8 text edits with backups.
- Added admin Debian file agent over protected Unix socket. Desktop launches Drive in a separate tab.
- Added file isolation/security/integration tests and deployment/handoff documentation.

## File Station desktop integration — 2026-10-30

- Shared Files UI renders natively in desktop and independently in Drive.
- Added authenticated same-origin gateway using desktop identity, context/overflow menus, multi-select and shared keyboard actions.
- Added bounded regular-file copy and same-root clipboard operations without overwrite.
- Preserved mounted state across Home/app switches; closing asks when text or uploads are pending.

## Platform documentation — 2026-10-30

- Added a maintained user guide and documentation index.
- Recorded Core/Design System/App SDK/peer-app/replaceable-Shell architecture and accepted i18n/theme principles.
- Added foundation milestones, account-locale/error-code migration requirements and mandatory agent documentation updates.
- Documentation only; no new runtime i18n, theme package or API behavior in this change.


## 2026-10-30 — foundation preview

- Thêm locale/theme/timezone theo tài khoản cho Desktop và Drive, có bộ chọn trong Files.
- Thêm i18n ICU/Intl vi/en, catalog menu Files, semantic theme tokens và host SDK adapter độc lập Shell.
- Backend thêm code lỗi tương thích; bỏ phân tích chuỗi tiếng Việt để xác định phiên đăng nhập trong App/Files.
- Một npm workspace/lockfile, kiểm tra catalog/boundary trong build và workflow CI.
- Chuyển đổi từng phần; chưa dịch toàn bộ giao diện hoặc hoàn thành SDK/Design System.
