Release status: Account Center usable v1 (a0b1248) deployed to VM 10.1.1.20 after pg_dump backup /opt/minhanos-backups/before-identity-006.sql; identity SQL 006 applied; Desktop, Drive, DB healthy. Browser acceptance pending user.

Fix (current): Account Center dialogs failed on plain HTTP because of crypto.randomUUID; use createIdempotencyKey from app-sdk. tests/insecure-context.mjs guards it.

# Handoff — Account Center usable v1 (current)

Read [usable v1 report](account-center/USABLE_V1_REPORT.md). Adds admin password reset (identityResetUserPassword, 34 operations), user detail page (groups?member=, audit-events?target=), sign-in audit (identity SQL 006 auth_audit; SQL 001–006 now immutable) and newest-first audit ordering. Host operators cannot have their password reset by web admins.
Checks: gofmt/vet/go test, migrations-integration (disposable DB, 5 new tests), clean npm ci + contracts:generate + npm run build: PASS. Next: request/correlation middleware, consolidate legacy audit tables, retention, browser acceptance.

# Handoff — Central Settings thin slice (current)

Branch settings-central-v1. Read [ADR 0007](adr/0007-central-settings.md) including the accepted hybrid ownership amendment (Language & Region panel, admin Users only in Account Center, embedded apps link to Settings for locale/theme) and AGENTS.md "Central Settings standard". Settings is now @minhanos/settings-app, built from manifest settingsIntegration; Desktop provides Appearance, Account Center provides profile/security/sessions/users panels, Files declares none. Legacy Settings "Users" UI removed (backend /api/admin/users unchanged). New manifests desktop.app.json, settings.app.json; optional manifest nameKey; shared-types regenerated.
app-sdk: useCatalog (refcount, fixes crash when an app and its panel mount together), host events session-ended/profile-changed, openSettings deep links. tests/settings-standard.mjs enforces the standard in npm run check.
Checks: clean npm ci + npm run build on VM check folder PASS (no Go change). Deployed; browser acceptance skipped by user, not performed. Next: generic settings storage API when Files needs settings; then Phase 5.

# Handoff — Account Center UI v1 / Phase 4 (previous)

Branch account-center-ui-v1. Read [UI v1 report](account-center/UI_V1_REPORT.md); it supersedes the Phase 3B "next step" note. Shared @minhanos/accounts-app is rendered natively in Desktop (app com.minhanos.account-center) and standalone at /accounts/ on both 8081 and 8082 via identity.MountAccounts. 33 v1 operations (new identityGetGroup); status.v1.json accountCenterUIEnabled=true.
package-lock.json now includes apps/accounts and packages/accounts-app (npm ci failed before). Identity client regenerated with npm run contracts:generate. No migration beyond 005.
Checks once in a separate VM source folder: gofmt, go vet, go test, tests/migrations-integration.sh (disposable DB), clean npm ci + npm run build (check, contracts, three builds): PASS. No browser trial loop.
Next: wire Files/registry resource enforcement, then expose app access policy editing; migrate remaining Desktop hardcoded strings; consider retiring Settings → Users once Account Center covers it.

Release status: Phase 3B images built and deployed once; Desktop, Drive and DB healthy. No extra browser/runtime trial loop.

# Handoff — Phase 3B (current)

Branch account-center-phase3b. Read [report](account-center/PHASE3B_REPORT.md), [ADR 0006](adr/0006-identity-security-and-delegation-apis.md) and contracts/identity/status.v1.json. 32 v1 operations active; schemas/client regenerated. Security APIs, groups, bounded custom roles/bindings and effective access are implemented. No new migration or UI application.
System Admin stays canonical users.role; generic bindings cannot grant it. Governance writes remain System Admin + grant + freshness. Custom roles cannot delegate policy-governance writes; only exact system scope. Host operator remains separate. Password change signs out both realms/current session; owner-only revocation is transactional/audited. Unknown session/device metadata is null.
Focused checks passed in separate source/disposable DB, no browser/stop-start test loops. Keep SQL 001–005/profile 001 immutable. Next: shared Account Center frontend/native+standalone. App policy editing waits for actual Files/registry enforcement; do not imply that a stored policy already blocks other apps.

Delivery note: user requested stopping repeated interactive tests. Existing checks already passed; fresh-test resources cleaned and Desktop restarted. Continue with focused verification only when new work needs it.

# Handoff — Phase 3A policy/users slice (current)

Branch account-center-phase3. Read [report](account-center/PHASE3_REPORT.md), [ADR 0005](adr/0005-identity-policy-and-host-operators.md) and contracts/identity/status.v1.json. Phase 3A is implemented, NOT the entire Phase 3 roadmap. Fourteen v1 operations are active in both hosts; advanced routes stay planned. Account Center UI is not enabled.
Shared identity.Service enforces app grant + actor/target ceilings, freshness, last-active-admin lock, versions, signed cursor and replay reauthorization. users.role remains the canonical System Admin flag; user-admin delegation is bounded. No group/custom-role path may introduce another System Admin authority. Legacy admin user routes use the same grant policy.
VM has identity 001–005 + profile 001; all applied SQL immutable. Pre-phase3 DB/source backups are private and restored in isolation. New web admins do not gain host access; existing operator snapshot is one-time, demotion revokes it. Broad root agent/ACL/commit-time transfer checks remain future work.
Settings admin-create requests current password with new vi/en labels. v1 blocks on incomplete UUID backfill; login assigns only the verified row's missing references. Next Phase 3B completes role/group/app policy and remaining session/password adapters before Phase 4 UI. Use report for actual tests and limitations; historical handoffs below are superseded.

# Handoff — Account Center Phase 2 (current)

Branch account-center-phase2. User authorized choosing suitable options and documenting decisions. Read [Phase 2 report](account-center/PHASE2_REPORT.md) and [ADR 0004](adr/0004-shared-auth-core-and-recovery.md). These supersede historical status below.
Shared AuthCore now powers Desktop, standalone Drive and native gateway. Active status/epoch checks, rotation, bounded account/IP/KDF controls and local OS-root recovery are implemented. Internal password/session mutation primitives exist; no v1 Identity HTTP handlers/Account Center UI/RBAC grants enabled. Files paths/legacy roles unchanged.
VM deployed, all services healthy; identity migrations 001–004 and profile 001 immutable. Private pre-phase2 DB/source backups restored in clone. Old cookies preserved across upgrade; runtime/fresh CLI and DB concurrency tests passed. Root check has one reserved IdentityLink warning. See report for exact tests/benchmark/limitations. Remaining users=1 after exact fixture cleanup.
Next Phase 3: backend business policy + last-admin race guard + host-root split BEFORE new administrative grants, then v1 endpoints and Phase 4 UI. Do not claim root privilege isolation already solved. Never create a third identity/session/password store or edit applied SQL. Recovery CLI is Linux-only OS-root, not a public API.

# Handoff — Account Center Phase 1

Nhánh account-center-phase1. Người dùng đã duyệt triển khai. Đọc [báo cáo Phase 1](account-center/PHASE1_REPORT.md) trước khi tiếp tục; đây là trạng thái hiện hành, các mục dưới là lịch sử.
Identity contracts/generated client + shared Go schema/repository/CLI đã triển khai. VM 8081/8082 healthy, UUID backfill users=1 pending=0. Không có Account Center UI/Identity HTTP handlers/RBAC enforcement mới.
Freeze identity SQL 001–003 và profile 001. Giữ users.id hex64 và Files paths; public UUID mapping không phải quyền. Không thêm users/session/password store. Backup trước migration đã restore thử DB cô lập; Files-volume recovery chưa kiểm chứng.
Tests: npm check, Docker builds/go checks, PostgreSQL concurrency/upgrade, runtime profile/API/Files/Drive và cài mới standalone đều PASS; một warning OpenAPI reserved IdentityLink. Chi tiết/commands/limitations trong report. Tiếp theo Phase 2 AuthCore, rồi Phase 3 policy/business APIs; host-root privilege split là gate trước cấp quyền mới.

# Historical handoff — Account Center Phase 0

Nhánh account-center-phase0; source khảo sát 026a24f7f9335bb0e42fbdee93aa08bc7e46066c.
Đọc docs/account-center/README.md và bốn báo cáo trước khi tiếp tục. Master prompt người dùng yêu cầu chỉ Phase 0
trong lượt đầu, chờ phê duyệt trước code/migration. Đây là trạng thái lịch sử trước khi người dùng duyệt Phase 1.
Đây là docs-only: chưa tạo AccountCenter runtime, chưa thêm schema/endpoint, không deploy hoặc đổi dữ liệu/quyền.

Các quyết định đề xuất: reuse users/two session realms; shared Go modular monolith; public UUID cùng users row,
giữ hex64 internal/storage key; backend-only RBAC/app grant/ACL; tách web admin khỏi host-root privilege.
Risk quan trọng: files-agent mở host / và current admin guard có thể vượt logical personal-file isolation;
chưa thực thi khai thác. User IDs không phải UUID; replacing IDs trực tiếp sẽ phá File paths/foreign keys.
Password hash đã dùng Go standard crypto/pbkdf2, không viết lại algorithm. UUID mapping/role split cần duyệt.
Identity schema adoption phải sau legacy bootstrap commit; profile migration 001 không sửa.
Kế hoạch Phase 1 chi tiết trong IMPLEMENTATION_PLAN.md. Tránh tạo users/session/password store thứ ba.


# Handoff — App naming v1

Nhánh platform-app-naming-v1. Đọc docs/adr/0003-app-naming-and-repository-policy.md.
contracts/apps/catalog.v1.json chốt 8 tên/App ID, alias và trạng thái triển khai; schema/CI kiểm tra xung đột.
Canonical brand Files là MinhAnOS Files; UI vẫn dùng File Station/Drive trong giai đoạn chuyển đổi.
Workspace frontend apps/drive đổi tên npm minhanos-drive → minhanos-files, không đổi thư mục/URL/service/volume.
Lệnh build standalone: npm run build --workspace=minhanos-files. Shared @minhanos/files-app giữ nguyên.
Monorepo GitHub vẫn minhtribtc/MinhAnOS; minhanos-* là tên module, chưa tạo/tách/đổi tên repository GitHub.
Legacy registry IDs files/apps/settings chưa chuyển sang reverse-DNS trong persisted state; catalog chưa là runtime resolver.
classic/touch là Shell variants tương lai; light/dark/system là theme; focus/windows là layout modes.
Không đổi UI Shell, nhãn quick panel, DB schema hoặc grant policy trong đợt này. Phần việc tiếp vẫn là Identity adoption/grants.


CI xác nhận implementation commit `fbc07c4` đã qua toàn bộ workflow, gồm DB integration: [run 41003701847](https://github.com/minhtribtc/MinhAnOS/actions/runs/41003701847).

# Bổ sung kiểm tra types và rehearsal

Đã sửa generator lấy tên type từ `$id`: nay xuất đúng AppManifest/AccountProfile/... theo tên public,
chuẩn hóa một newline cuối file. tests/shared-types.ts + tsconfig.contracts.json chạy skipLibCheck=false,
kiểm tra Profile không thành any và enum locale/job state vẫn chặt. Root contracts:check chạy tsc riêng.
Backup DB trước nâng cấp và source Git trước thay đổi ở /opt/minhanos-backups/before-migration-runner.* (0600).
Đã restore DB copy trong container tách biệt và kiểm tra fingerprint users/profiles không đổi sau adoption.
Không tương đương chứng nhận recovery dữ liệu volume Files. Binaries legacy chưa dùng runner cần deployment gate riêng.

# Handoff — transactional migrations, profile-first

Nhánh platform-migrations-v1 kế thừa platform-contracts-v1.
Đọc docs/adr/0002-transactional-migration-runner.md. Runner Go trong internal/migrate đã có DAG validation,
version bounds, immutable checksum/history, advisory lock 681024, atomic DDL+ledger và timeout.
Core profile dùng embedded migration SQL 001 để tạo mới hoặc kiểm tra/tiếp nhận bảng user_profiles hiện có.
Không rewrite user/profile IDs hay preferences. Drive vẫn độc lập, chỉ thêm shared internal/migrate vào image.

platform_migrations là ledger mới; legacy schema_version và identity bootstrap chưa chuyển.
contracts/migrations/modules.v1.yaml vẫn là kế hoạch registry tương lai, không phải applied runtime ledger.
Không có down/destructive/nontransactional/backfill runner. Chỉ chạy SQL additive được review và embed.
Không sửa migration đã áp dụng; thêm step mới. Khi checksum/history lệch, startup phải dừng để chẩn đoán.

CI thêm sh tests/migrations-integration.sh: DB/network disposable riêng, không publish port, không dùng secret production.
Thông số MIGRATION_TEST_DATABASE_URL chỉ dành cho test DB riêng. go test thường bỏ qua integration nếu không có biến.
Không chạy test này trực tiếp vào DB người dùng. Kết quả, deployment và giới hạn ghi đầu docs/validation.md.
Các bước tiếp: adopt identity có kiểm chứng → grants/delegation → file identity/ACL → durable uploads/jobs → events.
Nợ i18n/Desktop/Files từ các handoff trước vẫn tồn tại.


# Handoff — Platform Contracts v1

Nhánh platform-contracts-v1, kế thừa platform-foundations-v1. Đọc contracts/README.md,
docs/adr/0001-platform-contracts-v1.md và docs/migration-policy.md.
Đã thêm schema manifest/authz/error/profile/file/job/upload/event, OpenAPI 3.1, shared-types sinh tự động,
api-client độc lập Vue và parser hỗ trợ cả lỗi legacy/nested v1. SDK re-export để tương thích.
SDK package vẫn 0.1.0; sdkVersion=1 là host protocol, không phải package semver. Manifest sample dùng ^0.1.0.

Không có backend jobs/upload session/ACL/grants/event bus mới. Các /api/v1 route là planned;
/api/profile là endpoint hiện có duy nhất mô tả implemented trong OpenAPI này, các route legacy khác ở docs/api.md.
Không đổi schema DB hay URL ứng dụng. File Station vẫn embedded, Drive vẫn độc lập.
Run npm run contracts:generate khi đổi schema; npm run contracts:check kiểm tra generated drift/semver/schema.
CI thêm OpenAPI compatibility bằng oasdiff digest pinned; lần đầu không có predecessor spec nên tự bootstrap baseline.
Kiểm chứng và giới hạn thực tế xem đầu docs/validation.md. Cần tiếp tục hoàn tất i18n và triển khai các dịch vụ planned.


# Handoff hiện tại — foundation 0.1, chuyển đổi từng phần

Nhánh `platform-foundations-v1`, dựa trên `file-station-desktop`.
Đọc [extension-contracts.md](extension-contracts.md) trước khi thêm app: bảng rà soát, hợp đồng mở rộng và nợ chuyển đổi.
Đã có các package i18n/ui/app-sdk đầu tiên, npm workspace và lockfile chung, `/api/profile` dùng chung,
backend error codes tương thích. Phần hướng dẫn cũ bên dưới là lịch sử; câu "packages do not exist" không còn đúng.

Locale/theme/timezone lưu trong `user_profiles` theo session user ID; không nhận ID đích từ client.
Khởi tạo schema dưới advisory lock 681023. GET fallback theme preferences cũ nếu chưa có profile;
PUT yêu cầu đủ locale/theme/timeZone, lưu toàn bộ hồ sơ. Locale vi-VN/en-US; theme light/dark/system.
Auth, CSRF/Origin và isolation giữ nguyên. Drive dùng coreprofile, không import platform.
Theme hồ sơ ưu tiên hơn preferences.theme cũ; trường cũ còn được giữ để tương thích.

Files có bộ chọn ngôn ngữ/theme, menu/action labels và errors dùng catalog; Desktop dùng profile cho đồng hồ/theme.
Chọn English chưa dịch toàn bộ app. Còn hardcoded text, palette, một số catch cũ và Settings chưa tập trung hết hồ sơ.
SDK hiện là host/profile/client adapter, chưa có manifest/lifecycle hoàn chỉnh. Không đổi keyed v-for/v-show.
Không có durable/resumable transfers, Terminal hay OIDC mới trong thay đổi này.

Build mới: chạy npm ci ở root; npm run build. Docker dùng root workspace, không cần lockfile riêng từng app.
`npm run check` kiểm tra catalog ICU/key/parameter, provider isolation, CSRF, unknown codes và boundary imports.
Workflow `.github/workflows/verify.yml` cấu hình build/test Go/frontend trên push/PR; chỉ ghi CI xanh khi có kết quả thực tế.
Kiểm chứng và triển khai thực tế ghi ở đầu docs/validation.md. Không chạy compose down -v.
Backup trước thay đổi: /opt/minhanos-backups/before-foundations.tgz và before-foundations.sql, quyền 0600.


# Current direction — Platform foundations and living documentation

Accepted target: Platform Core + Shared Design System + App SDK + peer apps + replaceable Shells.
Read docs/platform-foundations.md before new foundation or UI work. It records the six i18n principles,
vi-VN/en-US foundation V0.1, account locale/fallback, ICU/Intl, stable backend error codes,
V0.2 build validation/plugin catalogs and independent Theme/Design Tokens.
These are accepted design requirements, not newly implemented runtime capabilities.
@minhanos/i18n, the official App SDK and unified theme tokens do not exist yet.
Next foundation work should establish these contracts and migrate Files/Drive/Desktop without remounting state.

User-facing guide: docs/user-guide.md. Update it whenever features, workflows, limits or settings change.
Documentation index: docs/README.md. AGENTS.md now requires maintaining the guide/index in the same implementation commit.
This change is documentation only; running apps, API and database were not modified or redeployed.
Earlier implementation handoffs follow below.

---
# Current handoff — native File Station + context menus

Branch: file-station-desktop. Desktop 8081 now renders File Station inside its native application host.
Drive 8082 still runs independently. Both import packages/files-app; no iframe and no duplicated file UI.
Embedded mode uses the desktop session through /api/drive/*; standalone mode uses its own Drive session.
The shared package imports no desktop code; API base and foreground lifecycle are explicit props.
Read docs/drive.md and docs/api.md for the adapter/gateway boundary and current limits.

Menu, overflow, toolbar and keyboard dispatch shared action IDs. Multi-select: Ctrl/Cmd or Shift.
Copy supports regular files <=128 MiB; folder copy remains disabled. Cut/move supports same-root folders.
Clipboard is application memory, not the OS clipboard; it survives Home/app switching but not closing/reloading the app.
Modals/menu are rendered through body portals, scoped to this UI and hidden when the instance is backgrounded.
Desktop close checks canClose for unsaved text/uploads; explicit accepted close cancels browser-owned uploads.
Container queries handle narrow desktop windows, independent of browser viewport size.

Backup before change: /opt/minhanos-backups/before-files-desktop.tgz and .sql (root-only).
The sections below are history and may describe the previous external-tab launcher.

---
# Current handoff — Drive 0.1 / branch file-station-v1

Read docs/drive.md first for current Files behavior, architecture and operational limits.
Drive is live on http://10.1.1.20:8082 with a separate Vue frontend, Go process, session cookie and build.
Desktop 8081 only launches the URL. Personal file CRUD/trash/text edits and admin Debian file operations are real.
Host agent: minhanos-files-agent.service on the VM; Unix socket /run/minhanos-files/agent.sock, root:10001 0660.
Integrated identities share users table; standalone compose.drive.yaml bootstraps its own empty database without desktop.
No iframe, desktop Pinia import, platform package import or platform HTTP call is needed by Drive.

Current backup: /opt/minhanos-backups/before-drive-v1.tgz + before-drive-v1.sql; root-only.
Keep drive-files volume and users IDs together. Do not delete volumes or overwrite .env/secrets.
Full details/API/tests/limits are in docs/drive.md, docs/api.md and docs/validation.md.
Next: resumable uploads + durable jobs, multi-select/copy and quota, then shared folders/ACL and richer previews.
The older design/history below predates implementation; Files is no longer a placeholder.

---
# Agent handoff — Desktop 0.2

## Source of truth

Repository: minhtribtc/MinhAnOS. Working branch: desktop-v2-ipados.
Local checkout: C:/dev/MinhAnOS-next. Use git log/status for exact revision; do not infer from old conversations.
Read root AGENTS.md and update this file on every subsequent change.

## Implemented

Vue 3/TypeScript/Pinia frontend; Go API; PostgreSQL. Interface V2 has iPadOS-inspired Home,
original vector wallpaper, floating Dock, app search, app switcher and Control Center.
Home widgets show API process metrics and a preview of the current user's stored notes.
Default focus mode shows one native application; Settings retains optional window mode.
App.vue owns Home/panel visibility. HomeScreen.vue and AppSwitcher.vue are shell components.
DesktopWindow/AppContent instances stay mounted with keyed v-for + v-show. Do not replace with conditional unmounting.
Explicit close/log out/reload destroys local drafts; only notes/preferences have durable storage.

Styles load in order: style.css (base/app layouts), fluent.css (shared app tokens/interiors),
ipados.css (V2 shell overrides). Future work should consolidate legacy shell selectors without breaking native app interiors.
Existing wallpaper preference IDs remain compatible: mountain uses the new default art; midnight dims it; sage shifts its hue.
No API/schema migration is required for this release.

## Runtime and deployment

Test URL: http://10.1.1.20:8081. VM root source: /opt/minhanos-next.
Compose project minhanos-next; services desktop/db; data persists in named volumes.
Local SSH identity: %USERPROFILE%/.ssh/id_ed25519_homelab. Never publish its content.
Runtime .env and secrets/bootstrap_password exist only on the VM; preserve their ownership/permissions.
Bootstrap file initializes an empty database, not an existing user's password.

Before V2 deployment, source/config backup was made at /opt/minhanos-backups/desktop-v1-before-ipados.tgz
(root-only) and image tagged minhanos-next-desktop:desktop-v1-before-ipados. This archive is not a PostgreSQL dump.
Restore source/image only with data compatibility reviewed. Never remove database volumes for a UI rollback.

To deploy: copy a source archive excluding .git, .env, secrets, artifacts and node_modules;
extract into /opt/minhanos-next, then docker compose up -d --build. Check health, API version and browser.
Keep deployment checkout on the same Git branch/revision as the tested source.

## Verification

See docs/validation.md for current V2 results. Build/test results must be recorded after execution.
API smoke script creates one e2e_ user; artifacts/test-user.txt records it. Remove only that exact disposable user.
Never commit runtime artifacts. Test draft retention with a harmless form draft; do not submit/create users from the browser.

## Limitations and next work

1. V2 means the second interface iteration; roadmap Split View, app tabs and panel multitasking are not implemented yet.
2. No Terminal/upload engine or durable jobs; keeping Vue mounted does not provide server-side task persistence.
3. NetBird, Authentik, 1Panel and file management remain clearly marked placeholders; no iframe-only integration.
4. API metrics describe Go process/filesystem, not all Debian resources. No Linux host agent exists yet.
5. Modal overlays use inert background, initial focus, Tab wrapping and focus restoration; full screen-reader/device testing remains outstanding.
6. Next: split view with explicit lifecycle contracts, then restricted Debian agent and first real integration adapter.
7. Current deployment is a LAN HTTP preview, not a production release.

## File Station design handoff — 2026-10-30

User requested a design before implementation. Read docs/file-station-design.md and
its interactive fragment docs/designs/file-station.html. Both variants use mock data only.
No Files API, filesystem volume, schema or running deployment was changed in this design pass.
Next priority is File Station after design feedback; the earlier Split View priority is deferred.
Implement per-user storage and authorization before connecting the UI to real files.

### Latest requirement — host filesystem administration

Admin must be able to manage Debian system files. Read the appended section of docs/file-station-design.md.
Add a distinct privileged «Hệ thống Debian» location and restricted host file agent; keep desktop/API non-root.
No host filesystem functionality has been implemented or deployed. The existing interactive mockup predates this requirement.
Feature gap inventory now includes desktop file interactions, archive tools, transfer recovery and cloud collaboration.

Verification caveat: the native browser confirm for pending changes could not complete in automated UI control.
The close guard is implemented centrally in store.close/logout and called from titlebar/switcher paths,
but manual acceptance/cancellation across browsers is still required. Draft retention itself was verified.
Browser automation Ctrl+X did not trigger; Cut through the shared action/menu did pass.
