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
