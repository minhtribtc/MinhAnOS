## Central Settings (current)

packages/settings-app renders Settings from manifest settingsIntegration and provider panels exported by app packages (app-sdk defineSettingsProvider). apps/desktop/src/settings.ts lists compiled first-party providers filtered by the backend registry; permissions from /api/v1/identity/me/access only filter the list. App-to-Shell bridge in app-sdk: useCatalog refcount, session-ended/profile-changed events, openSettings deep links. See [ADR 0007](adr/0007-central-settings.md).

## Account Center UI v1 (current)

packages/accounts-app is the single Account Center UI (Vue, app-sdk host, generated identity client). Desktop embeds it natively; apps/accounts is a standalone Vite host mounted at /accounts/ by internal/platform and internal/drive via identity.MountAccounts. No Desktop imports in the package, no iframe, no new service. Desktop api.ts tracks an auth epoch and store.expireSession() clears local state when the server session ends.

## Identity Phase 3B (current)

Shared security/group/delegation services extend the existing transaction wrapper; no schema or deployment dependency change. 32 operations now active, including current-user credential/session control and effective-access DTO. System Admin authority and host operator stay separate; custom governance delegation is capped. [ADR 0006](adr/0006-identity-security-and-delegation-apis.md).

## Identity Phase 3A (current)

identity.Service + HTTP adapter dùng chung Desktop/Drive, không import Platform vào Drive. Một nguồn users/password và hai cookie realms giữ nguyên. Lõi Users dùng registered app grants, reserved web-role ceilings, transaction-time checks và explicit host operators. users.role là nguồn System Admin hiện hành; đầy đủ group/custom-role policy vẫn pending. [ADR 0005](adr/0005-identity-policy-and-host-operators.md) ghi ranh giới, concurrency và migration 005. Hợp đồng x-implementation chỉ bật 14 operation đã kiểm thử.

## AuthCore Phase 2 (current)

Shared internal/identity.AuthCore thay cơ chế session/login trùng lặp trong Desktop/Drive. Giữ một credential store và hai canonical session tables; public realm desktop/files. Identity migration 004 thêm epoch/handles/recovery/throttles/auth events. Internal mutation transactions lock users trước sessions, recheck credential sau KDF và revoke cả hai realms. Không thêm IAM daemon hoặc quyền RBAC mới. [ADR 0004](adr/0004-shared-auth-core-and-recovery.md) ghi quyết định và giới hạn.

## Identity adoption đã triển khai

internal/identity là shared Go library trong Desktop và Drive, dùng cùng users và migration runner; chạy sau legacy bootstrap commit. Public UUID và credential reference thêm trên cùng row; session/password stores giữ nguyên. Relations mới rỗng, chưa có policy engine/business endpoints. Xem [Phase 1 report](account-center/PHASE1_REPORT.md).

## Cập nhật kiến trúc nền tảng 0.1 (2026-10-30)

Bản hiện tại dùng npm workspace và một lockfile root. Các package đầu tiên: i18n (ICU/Intl), ui (tokens/theme),
app-sdk (Vue host provider, profile, API client). Đây là đợt tách foundation, chưa phải SDK/UI kit đầy đủ.
Backend dùng chung coreprofile và apicontract; không tạo phụ thuộc Drive → platform.
Locale/theme/timezone thuộc Core profile, layout/wallpaper/notes vẫn thuộc Desktop preferences.
Xem [bảng mức độ đáp ứng và hợp đồng mở rộng](extension-contracts.md).
Các mô tả Desktop 0.1 bên dưới là lịch sử; trạng thái Files/agent mới nhất ở các phần bổ sung và drive.md.


# Kiến trúc MinhAnOS

Kiến trúc đích đã chốt: Platform Core + Shared Design System + App SDK + ứng dụng ngang hàng + nhiều Shell thay thế.
Quyết định, ranh giới, i18n và Theme/Design Tokens: [platform-foundations.md](platform-foundations.md).
Các phần bên dưới mô tả triển khai và lịch sử; không có nghĩa mọi năng lực nền tảng đích đã được xây xong.

## Desktop 0.1 — kiến trúc đã triển khai

- Frontend Vue 3/TypeScript/Pinia, Vite. Không phụ thuộc kodbox hoặc 1Panel.
- Go net/http phục vụ cùng origin cho API và static assets; không cần reverse
  proxy trong LAN test. PostgreSQL lưu users, sessions, preferences và audit.
- Session token ngẫu nhiên 256 bit; DB chỉ lưu SHA256 token; cookie HttpOnly,
  SameSite Strict, TTL tuyệt đối 24 giờ. LAN HTTP test chưa dùng Secure cookie.
- PBKDF2-SHA256 600.000 vòng, salt riêng cho password; giới hạn login theo IP.
- Write API kiểm tra Origin/fetch metadata và CSRF session token. Backend kiểm
  tra role admin tại các route user management; frontend chỉ điều chỉnh UI.
- Preferences gồm theme, wallpaper, notes, pin list và window geometry; khóa user
  lấy từ session, không nhận user ID đích từ request. Unknown app IDs bị từ chối.
- API chạy uid 10001, read-only rootfs, drop ALL capabilities, không có Docker socket.
- Admin bootstrap từ secret file khi users table rỗng. Không có password mặc định.
- Registry gồm 8 module: overview, files, apps, connections, settings, activity,
  notes, about. Files và integrations ghi rõ planned/unconfigured.
- Metrics: heap Go, goroutines, process uptime và filesystem /data. Chưa đo host
  toàn diện, không có Linux agent, không dùng số liệu hoặc phần trăm CPU giả.
- Migration schema v1 tạo transaction tại startup. Chưa có migration runner đa
  phiên bản hoặc chính sách downgrade; phải bổ sung trước các thay đổi schema tiếp.

## Những giới hạn chủ động của bản đầu

Chưa có OIDC, ACL file, NAS, jobs, package install, SSO, file upload/download,
change/reset password, disable/delete user hay nhiều node. App Center là registry
nội bộ, không phải marketplace. Nhấn vào một module chưa hoàn thành mở trang mô
tả trạng thái, không thực hiện thao tác dịch vụ.

Go API/desktop/database chạy Docker Compose trên VM test. Ứng dụng quản trị
thực tế cần HTTPS, IdP/permission model chi tiết, migration/backup restore đã thử
và hardening riêng trước khi đưa ra ngoài LAN.

## Các quyết định tiếp theo

1. Authentik OIDC có issuer/client/callback thật; phân tách external identity và local user.
2. Adapter đọc trạng thái NetBird/1Panel sau khi kiểm kê version/capability.
3. Permission registry chi tiết theo module/action; audit và thu hồi phiên.
4. File service có data volume, ACL và journal/reconciliation, không nối trực tiếp
   vào data của kodbox đang chạy.

## Interface V2 (0.2.0)

Shell components: App.vue orchestrates Home, search, switcher and Control Center.
HomeScreen renders widgets from existing store/API data; AppSwitcher lists real open instances.
ipados.css loads last, replacing the outer Fluent shell while keeping shared app interiors.
Home visibility is local UI state; it does not minimize/remove application instances.
Preference IDs and backend API/schema remain compatible with desktop-v1.
Do not confuse this interface iteration with completion of the full V2 multitasking roadmap.

## Drive 0.1 — independently runnable application

Read [drive.md](drive.md). apps/drive is a separate Vue build; cmd/drive/internal/drive serve its API/static files on 8082.
The Drive image does not copy desktop/platform sources. compose.drive.yaml deploys only Drive/PostgreSQL.
Integrated mode shares users but has a separate Drive session table/cookie. IdentityStore decouples identity integration;
authn holds shared password/token primitives. Both schema initializers serialize startup via the same advisory transaction lock.
No desktop HTTP call, UI state, iframe or parent-window messaging is required.
Host files are a separate privileged Unix-socket service; the web container remains uid 10001 without a host root mount.
Private files and trash are filesystem-backed under immutable user IDs. Request audit uses PostgreSQL; host outcomes use journald.
This release does not add durable transfers/jobs or claim power-loss transactionality; see drive.md for limits.

## Native Files host / shared action architecture

Shared UI now lives in packages/files-app, consumed by apps/drive and desktop AppContent through a source alias.
It owns file navigation, selection, menu/action registry and API client, with no desktop imports.
The desktop host controls visibility and calls exposed canClose; native instance stays mounted when backgrounded.
/api/drive/* is an authenticated same-origin gateway to the independent Drive service. An optional SQL identity adapter
uses platform sessions only in integrated mode; standalone deployment keeps its existing independent identity behavior.
Copy is a bounded regular-file operation using temp/finalize/no-replace semantics. Recursive copy/durable jobs remain pending.


## Platform Contracts v1

Nguồn hợp đồng tại [contracts](../contracts/README.md), quyết định tại [ADR 0001](adr/0001-platform-contracts-v1.md).
@minhanos/api-client tách transport/ApiError khỏi Vue SDK, đọc legacy và nested-v1 errors; backend chưa đổi envelope.
@minhanos/shared-types sinh từ JSON Schema và được SDK dùng cho Profile.
Manifest schema/protocol version không phải SDK package version. Runtime manifest registry, app grants, file identity,
jobs/uploads/event transport và module migration runner còn planned; không thay thế kiểm tra quyền backend hiện có.


## Transactional migrations — profile-first

Runner dùng chung internal/migrate, ledger platform_migrations, SQL thuộc coreprofile/migrations.
Desktop và Drive dùng cùng advisory transaction lock; history/checksum được kiểm trước pending SQL.
Identity/bootstrap/schema_version cũ chưa chuyển. Xem [ADR 0002](adr/0002-transactional-migration-runner.md).


## App naming and repository policy

[ADR 0003](adr/0003-app-naming-and-repository-policy.md) và contracts/apps/catalog.v1.json là nguồn tên/App ID chuẩn.
Monorepo MinhAnOS chứa các module minhanos-*. apps/drive là host standalone với npm workspace minhanos-files;
@minhanos/files-app là UI dùng chung. Catalog danh tính không phải runtime app registry hoặc hệ thống cấp quyền.
Shell variant classic/touch, theme light/dark/system và workspace mode focus/windows là ba khái niệm riêng.


## Account Center architecture checkpoint

[Khảo sát và phương án Identity](account-center/README.md) đã được ghi nhận ở Phase 0.
Đây là đề xuất chưa phê duyệt: public UUID giữ storage key cũ, shared IdentityService và độc lập Account Center UI.
Không có runtime/schema changes trong đợt này; nguồn hiện tại vẫn như các phần implementation phía trên.
