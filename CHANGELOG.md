## Account Center Phase 3A — 2026-10-30

- Bật 14 Identity v1 operations cho Users/My Account/roles-permissions read/audit và reauthentication; thêm version/cursor/replay guards.
- Bảo vệ admin cuối cùng, giới hạn User Admin và tách host operator khỏi quyền System Admin mới.
- Settings tạo admin có xác thực lại vi/en. Các API group/custom-role/app policy, session/password v1 đầy đủ và Account Center UI vẫn pending. Xem docs/account-center/PHASE3_REPORT.md.

## Account Center Phase 2 — 2026-10-30

- Dùng chung AuthCore cho Desktop/Drive; kiểm trạng thái/epoch, xoay cookie khi login và giới hạn đăng nhập có chặn quá tải.
- Thêm primitives đổi mật khẩu/thu hồi cả hai realm và CLI recovery dành OS root với token 15 phút, một lần dùng.
- Giữ giao diện/API legacy; chưa bật Account Center UI hoặc Identity HTTP quản trị. Xem docs/account-center/PHASE2_REPORT.md.

## Account Center Phase 1 — 2026-10-30

- Thêm Identity contracts, generated types/client, schema adoption và public UUID cùng user hiện hữu.
- Thêm CLI backfill có giới hạn; giữ Files storage IDs và session realms. Chưa bật Account Center UI/API/RBAC mới.
- Kiểm thử upgrade DB thật, concurrency, cài mới và smoke VM; xem docs/account-center/PHASE1_REPORT.md.

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


## 2026-10-30 — Platform Contracts v1 preview

- Thêm manifest/API/authz/files/jobs/uploads/events schemas và chính sách migration theo module.
- Sinh shared-types từ schema; tách api-client khỏi Vue SDK, đọc cả legacy và nested-v1 errors.
- Thêm kiểm tra schema, SDK semver/capability, generated drift, OpenAPI lint và breaking-change guard trong CI.
- Các dịch vụ /api/v1 vẫn planned; không thay API legacy, DB hay chức năng upload đang chạy.


## 2026-10-30 — transactional migration runner

- Thêm module DAG validation, immutable checksum ledger và khóa chống startup đồng thời.
- Profile schema dùng migration embedded; tiếp nhận dữ liệu hiện có sau kiểm tra tương thích.
- Thêm integration tests PostgreSQL riêng cho rollback/cancellation/concurrency/connection loss và adoption.
- Chưa chuyển identity bootstrap; chưa có destructive/down/backfill runner hoặc grants/jobs/upload mới.

- Sửa generator shared-types để xuất đúng tên công khai và newline ổn định; thêm TypeScript contract check không dùng skipLibCheck.


## 2026-10-30 — App naming v1

- Chốt tên/App ID/module của 8 ứng dụng bằng catalog có schema và kiểm tra trùng ID/alias trong CI.
- Đổi npm workspace host Files sang minhanos-files; shared package, thư mục, API và deployment giữ nguyên.
- Chuẩn hóa thuật ngữ Desktop/Shell/Workspace; tách Shell variants khỏi theme/layout.
- Tên UI File Station/Drive chưa đổi; các app planned chưa được tạo hoặc kích hoạt.


## 2026-10-30 — Account Center Phase 0

- Bổ sung repository audit, gap/security analysis, Identity architecture và implementation/acceptance plan.
- Ghi checkpoint phê duyệt trước code/migration; không thay chức năng hoặc dữ liệu hiện tại.
