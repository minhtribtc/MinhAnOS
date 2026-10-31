## Organizations and tenancy (row 27) — 2026-10-31

Go DB suites (`tests/migrations-integration.sh`, 8 packages) PASS, including `TestOrganizationsRolesAndIsolation`,
`TestOrganizationAdoptionAndDepth`, `TestAllocationCarveOut`, `TestConcurrentAllocationsNeverOverAllocate` (10
concurrent 20 MiB children of a 100 MiB parent: exactly 5 succeed, delegated = sum of child limits),
`TestSharesAreTenantIsolated` and `TestTenancyStorageAndLogsOverHTTP` (IDOR 404 on every share route, cross-tenant
create 403, scoped share list and Log Center, launcher). `npm run check` (incl. `tests/app-standard.mjs` row 27:
organization columns, no client-side active organization, catalog entries for every tenancy error, per-request scope)
and vue-tsc builds of Desktop, Files and Accounts PASS.
Deployed to VM 10.1.1.20 (backup before-organizations.sql): smoke suites, backup-restore and a11y-browser (organization
screens included, after labelling the usage progress bars) PASS; live API probe of an organization admin PASS.

## Desktop and Files second standard pass (rows 9, 11, 15, 17, 19, 20, 22) — 2026-10-31

VM /opt/minhanos-std: `npm run check` PASS (new: app-data, a11y-tokens, files-desktop-contracts incl. job routes and job.v1 compatibility); `vue-tsc --noEmit` + `vite build` Desktop and Drive PASS; files-actions, files-client PASS; gofmt, `go vet ./...` PASS; migrations-integration PASS for all packages incl. TestQuotaThresholdNotices, TestFilesJobsCopyCancelAndLease (copy to the end, staging removed, FILES_EXISTS without overwrite, idempotent cancel, reclaimed lease fences the old run, transient failure requeued), TestRemoveDeletedUsers (rows at once, home only after the grace period), TestExportHomeArchive (no links, trash or other users), platform preferences 428/412, export and removal.
Deployed (backup /opt/minhanos-backups/before-files-jobs.sql): storage 5 and files 1–4 in `platform_migrations`; containers healthy. api-smoke (10), drive-smoke (12, incl. folder copy job 202 → succeeded, copy into itself 400, unknown job 404), files-desktop-smoke (stale `CONFLICT` expectation fixed to `FILES_EXISTS`, the code since d291919), profile-smoke PASS. `sh tests/backup-restore.sh`: 8 tables with dump row counts, 5 migration modules, 18 files with equal sha256 PASS. `tests/a11y-browser.mjs` in mcr.microsoft.com/playwright:v1.56.1-noble with playwright 1.56.1 + axe-core 4.10.3 installed in /opt/pw (not repo dependencies): axe WCAG 2.1 A/AA with no violations on Desktop and Files in light and dark (incl. open dialogs), focus stays in the Desktop search and Files new-folder dialogs and Escape closes them, no horizontal overflow at 375 px PASS. Not performed: a job interrupted by a real container restart (covered by the lease test only), export of a very large home (each archive entry extends the write deadline; not measured).

## Desktop and Files app standard pass — 2026-10-31

VM /opt/minhanos-std (node:22-alpine, golang:1.26-alpine): `npm run check` PASS incl. new app-standard check (manifests, registry IDs ↔ catalog ↔ Shell registry, vi/en parity and literal keys, no hex/rgba, no Vietnamese literals, no `registerCatalog`, no `error.message`, no legacy names, modal + labelled dialogs, Drive and legacy `/api` error codes, no `log.Print`, no legacy audit inserts); `vue-tsc --noEmit` and `vite build` Desktop and Drive PASS; gofmt (4 files reformatted), go vet, `go test` drive/logcore/platform/identity/storagecore PASS; migrations-integration incl. TestFilesAuditPrivacyAndAppendOnly (personal rows reject paths, share rows need a key, update/delete refused, retention delete only after 365 days) and the logcore `files` source.
Deployed (backup before-files-events.sql): files step 1 applied; api-smoke (10, incl. `/api/audit` 404 and no placeholder integrations), drive-smoke (10), files-actions, files-client PASS; `files_events` rows from the smoke run: personal rows without path, system rows with path. Login screens of Desktop (8081) and Files (8082) render with tokens and the new names. Not performed: signed-in browser check of Desktop windows, Spotlight pin and Files in light/dark (needs a signed-in user).

## Storage Q3 quota folders and share measurement — 2026-10-31

VM /opt/minhanos-q3: gofmt (2 files reformatted), `go vet ./...`, `go test` drive (TestQuotaFoldersInShare: measurer creates the folder and measures the share, upload over the folder limit 507, upload outside it OK, move into a full folder 507 then OK after raising the limit, trash takes the bytes off the folder, trashing the quota folder 409 QUOTA_FOLDER_PROTECTED, rename rewrites the registration, /api/quota-folders lists it), storagecore, platform PASS. migrations-integration PASS incl. TestQuotaFoldersNestedLimits (invalid paths, homes share refused, duplicate, pending measurement, inner limit refuses although outer fits, commit on both, expired folder reservation released, prefix rewrite keeps IDs, usage floors at 0, limit update, stale-version delete 412) and TestShareMeasureRequest. `npm run check` PASS (storage contracts incl. quota folder path validation), `vue-tsc --noEmit` Desktop and Drive PASS.
Deployed d9ad03a (backup before-storage-004.sql): storage step 4 applied; a probe quota folder on share2 (`q3-probe/inner`) was created by the Drive measurer (owner 10001, 0750) and measured, and a share re-measure request was processed, both within 35 s; probe rows and directory removed; new endpoints 401 without session. Not performed: browser use of the quota folder panel and the Files heading (needs a signed-in admin).

## Storage Q2 share permissions and share roots — 2026-10-31

VM /opt/minhanos-q2 (golang:1.26-alpine, node:22-alpine, disposable postgres:18-alpine): gofmt (1 file reformatted), `go vet ./...`, `go test` drive (TestShareRootsAndPermissions: roots, write to a write share reserves on the share ledger and lands in the share unit, read share refuses writes with SHARE_READ_ONLY and allows listing, unknown share 404, home unit created on first use; TestNoHomesShareYet: no personal root, STORAGE_NOT_READY), storage, storagecore, platform PASS. migrations-integration PASS incl. storagecore TestShareAccessPrecedence (creator write, group read, stale version 412, deny beats group write, invalid subjects/types/access/duplicates rejected, homes share has no permissions), TestShareQuota (share limit, commit, homes share untouched), TestFirstRunHomesShare (kept with data, retired when empty, personal reserve ErrNoHomes, homes share on ZFS, second homes share refused, managed homes never retired). `npm run check` PASS incl. storage contracts (new schemas = OpenAPI, generated client, hostsHomes, access input rejects role subjects and unknown access); `vue-tsc --noEmit` Desktop and Drive PASS.
Deployed 607f7c6 (backup before-storage-003.sql): storage step 3 applied; legacy homes kept (has data); agent rebuilt and restarted, class directories root:10001 0750; Drive container (uid 10001) lists and writes the existing Btrfs share unit and reaches the agent socket; agent returns the share path and 404 for a missing home unit; effective-access query checked against the live identity schema inside a rolled-back transaction; /api/roots and /api/v1/storage/subjects 401 without session. Not performed: browser use of the permission dialog and share roots (needs a signed-in admin).

## Notification Center + Log Center minimal v1 — 2026-10-31

On VM 10.1.1.20 (node:22-alpine, golang:1.26-alpine, disposable postgres:18-alpine): `npm run check` PASS (foundations, settings standard, insecure context, docker context, contracts, identity/storage/notifications+logs contracts, tsc, redocly); `vue-tsc --noEmit` for Desktop PASS; gofmt, `go vet ./...` PASS; `go test` notifycore (delivery without duplicates after redelivery, paging without overlap, own-inbox isolation, idempotent mark, mark-all up to a time, cursor/limit validation, 90-day retention), logcore (merge order across sources, actor/target names, filters, paging, invalid input), identity, platform, storagecore, drive PASS.
Deployed d5312ce (backup before-notifications-logs.sql): migrations identity 11 and notifications 1 applied, grants present, worker moved a throwaway outbox row into the inbox within 10 s (deleted afterwards), /api/v1/notifications and /api/v1/logs 401 without session, health ok.


## Storage standard fixes (rows 15, 25) — 2026-10-31

VM /opt/minhanos-fx2: gofmt (2 reformatted), go vet, go test PASS incl. drive background measurement; migrations-integration PASS (storage 002); clean npm ci + contracts:generate + build PASS incl. new tests/storage-contracts.mjs (schema = OpenAPI, generated client, If-Match/Idempotency-Key/CSRF on writes). go test -race not available (Alpine image without cgo); the measurer channel is created before serving to avoid a race by design. Deployed 939d0ea: settings row version 1, storage_idempotency present, /api/v1/storage 401 without session, old /api/storage 404, no measurer errors in the Drive log.

## Storage Q1 soft quota — 2026-10-31

VM /opt/minhanos-q1: gofmt (4 files reformatted), go vet, go test PASS incl. drive TestPersonalWritesUseSoftQuota; migrations-integration PASS incl. storagecore TestReserveCommitReleaseAndLimits, TestConcurrentReservationsNeverExceedLimit (12 parallel, limit for 5), TestExpiredReservationAndMeasurement, TestCreateShareRollsBackOnAgentFailure (two test bugs fixed first: reading a rolled-back row, UPDATE on an empty table not firing the row trigger). Clean npm ci + build PASS after removing a cmd/ path the app catalog schema rejects. Storage agent with MINHANOS_STORAGE_OWNER: new ZFS and Btrfs units are 10001:10001 mode 750. Deployed 3b7930c: live DB has volume files (directory) and share homes; storage agent socket visible in the Desktop container (group 10001); /api/storage without session 401. End-to-end Storage Manager use in a browser not performed.

## Storage S1 — 2026-10-30

Test VM: ZFS 2.1.11 pool mnzfs (10 GB) and Btrfs (11 GB, quota on). internal/storage contract tests PASS on both (quota enforcement with random data, snapshots, destroy), agent handler tests PASS, live socket smoke PASS on both. Found and fixed: systemd sandbox options gave the agent a private mount namespace so ZFS mounts were invisible on the host. apicontract test failed only in a partial source upload without contracts/ fixtures (not a regression).

## Phase 5 Files authorization and app access — 2026-10-30

VM /opt/minhanos-m3: gofmt (3 files reformatted), go vet, go test PASS incl. drive TestFilesRequestsUseIdentityPolicy; migrations-integration PASS incl. TestFilesPolicyEntitlementGrantAndDenyRules, TestAppAccessGuards and existing TestDriveProxyRequiresDesktopSession with real SQLIdentity; clean npm ci + contracts:generate + npm run build PASS (vue-tsc). Browser acceptance not performed (requires user sign-in). Deployed 3869353 after pg_dump before-identity-009.sql: all containers healthy; live DB role grants standard-user/user-admin files.read+write, system-admin all four; 4 Files app grants enabled; Drive /api/roots without session 401.

## M2 admin sessions and forced password change — 2026-10-30

VM /opt/minhanos-m2: gofmt clean, vet, go test PASS (incl. drive TestFilesClosedWhilePasswordChangeRequired); migrations-integration PASS incl. TestResetRequiresPasswordChangeUntilUserChangesIt and TestAdminListsAndRevokesAnotherUsersSessions after tightening two old leak checks that matched any "password" substring; clean npm ci + contracts:generate + build PASS incl. new docker-context test.

## Phase A audit/event foundation — 2026-10-30

VM check folder /opt/minhanos-m1: gofmt (auth.go reformatted), go vet, go test PASS incl. new internal/requestid tests; migrations-integration on disposable DB PASS incl. TestSecurityLogAppendOnlyRetentionAndRetiredLedger, TestSignInUsesRequestAndCorrelationIDs, TestSecurityNoticesQueuedWithTheChange and updated TestCredentialChangeRollsBackIfAuditFails; clean npm ci + contracts:generate + npm run build PASS.

## Fix: dialogs on plain HTTP — 2026-10-30

User reported create account/group/role not working. Live DB showed no create or denied events after sign-in, so requests never reached the API. Root cause: AccountsApp called crypto.randomUUID (secure contexts only) in open(), the idempotency-key watchers and changePassword; on http://10.1.1.20 it is undefined, so every dialog threw. Node-based tests had randomUUID and missed it. Fix: createIdempotencyKey (api-client, re-exported by app-sdk) with getRandomValues fallback; new tests/insecure-context.mjs removes crypto.randomUUID and scans frontend sources. Clean npm ci + npm run build PASS on VM. Browser confirmation by the user still pending.

## Account Center usable v1 — 2026-10-30

VM check folder /opt/minhanos-ac-usable: gofmt clean, go vet, go test PASS; tests/migrations-integration.sh on disposable DB PASS incl. new TestAdminPasswordResetRevokesBothRealmsAndAudits, TestPasswordResetCannotTakeOverHostOperator, TestPasswordResetHTTPRoute, TestSignInAuditBoundedOrderedAndFiltered, TestGroupsFilteredByMember; adjusted TestProtectedPolicyCeilingsAndAuditRollback (sign-in now audits transactionally). Clean npm ci + contracts:generate + npm run build PASS. VM network was briefly unreachable during the work (no reboot). Deployed a0b1248 after private pg_dump; live DB shows the widened outcome constraint and new indexes, existing events intact; /accounts/ 200 on 8081 and 8082. Browser acceptance not performed (requires user sign-in).

## Central Settings thin slice — 2026-10-30

Hybrid amendment: clean npm ci + contracts:generate + npm run build PASS on VM check folder; the new test first FAILED on shorthand provider syntax `{language}` and its pattern was widened. No lockfile/generated drift.
Deployed 9cf4075 to VM: containers healthy, / returns 200 on 8081 and 8082. Browser acceptance was skipped at the user's request and is NOT performed.
VM check folder /opt/minhanos-settings-claude: clean npm ci + npm run build PASS (foundations, accounts-session-state, new settings-standard, contracts after contracts:generate for manifest nameKey, identity contracts, vue-tsc + desktop/files/accounts builds). First run FAILED twice and was fixed: an escaped regex in the new test, and shared-types drift until regeneration. No Go changes, so Go tests not rerun. Not yet deployed; no browser pass yet.

## Account Center UI v1 — 2026-10-30

Ran once in separate VM folder /opt/minhanos-ui-claude (not the app DB): gofmt (7 unformatted files fixed), go vet ./..., go test ./... and tests/migrations-integration.sh on its disposable DB (0 FAIL, includes TestAccountsStandaloneAssets and group detail/ETag check): PASS. Clean npm ci initially FAILED because package-lock.json lacked the new workspaces; regenerated lockfile, then npm ci + npm run build (foundations, accounts-session-state, contracts, identity contracts after contracts:generate, desktop/files/accounts builds): PASS. No browser/VM stop-start trials. Deployed 964e75a on VM (docker compose up -d --build --wait): all three containers healthy; HTTP smoke on 8081 and 8082: / 200, /accounts/ 200, /accounts 307, hashed asset 200, /api/v1/identity/me without cookie 401.

## Phase 3B — 2026-10-30

One full Go/vet + isolated PostgreSQL regression and root contract/type checks passed in a separate source folder. Targeted affected security/group tests additionally checked actual HTTP requests and group-deletion audit/effective-access behavior. No browser/VM stop-start trials or production credential mutations. Existing unused IdentityLink warning remains. Deployment health is not a substitute for the recorded isolated checks. [Report](account-center/PHASE3B_REPORT.md).

## Phase 3A — 2026-10-30

Go/unit/vet, PostgreSQL concurrency/policy/auth regression, HTTP adapters, contract/type/client generation, OpenAPI compatibility, restored pre-upgrade DB và VM smoke (Identity v1/legacy/profile/Files/agent): PASS. Browser thật kiểm tra trường current-password khi chọn admin và bố cục; không submit quyền qua browser. Replay byte-order test ban đầu fail do jsonb, đã canonicalize và rerun PASS. Một IdentityLink lint warning còn nguyên. [Report](account-center/PHASE3_REPORT.md) ghi phạm vi, migration và các kiểm tra cài mới độc lập.

## Account Center Phase 2 — 2026-10-30

Shared AuthCore Go/unit/vet, dedicated PostgreSQL concurrency/ownership/recovery/throttle tests, actual HTTP adapter CSRF/Origin/rotation tests, restored pre-upgrade DB, Docker builds, npm check, existing-cookie upgrade, runtime profile/API/Files/agent smoke, standalone recovery CLI/replay/nonroot và Drive khi Desktop dừng: PASS. CI script đã thêm package platform cho DB-backed HTTP test. Một reserved IdentityLink lint warning còn nguyên. Read-only docker cp test bị từ chối, chuyển root stdin vào /tmp tmpfs; không nới rootfs. [Báo cáo Phase 2](account-center/PHASE2_REPORT.md) chứa inventory, security review, migration, benchmark nhỏ và giới hạn.

## Account Center Phase 1 — 2026-10-30

Đã chạy npm check/generate, OpenAPI compatibility, Docker/go checks, dedicated PostgreSQL migration/concurrent backfill, restore DB pre-upgrade, runtime API/profile/Files/Drive smoke, Drive khi Desktop dừng và cài mới standalone project cô lập: PASS. Một warning IdentityLink reserved chưa dùng. Import-name build collision đã sửa. Chưa có UI Account Center, chưa kiểm thử RBAC mới hoặc full Files-volume recovery. Evidence và phạm vi chính xác: [Phase 1 report](account-center/PHASE1_REPORT.md).

# Kiểm chứng Account Center Phase 0 — 2026-10-30

Docs-only, source audit tại 026a24f. Đã đọc authentication/session, middleware, native host, SDK/manifest,
Files namespace/host agent, schema/bootstrap và migration runner. Đã đối chiếu nguồn Authentik/Go/OWASP/PG chính thức.
Bốn báo cáo chứa phạm vi, evidence, security risks, proposed contracts/schema/tasks và phase acceptance criteria.
Không chạy exploitation, đọc dữ liệu riêng trên VM, login test, benchmark hoặc migration mới.
Không đánh dấu kiến trúc đề xuất là triển khai; code/migration cần approval theo master prompt.
Kiểm tra 60 liên kết tài liệu nội bộ qua; diff chỉ gồm Markdown. CI runtime cũ không được coi là test chức năng Account Center.

---


# Kiểm chứng app naming — 2026-10-30

Nhánh platform-app-naming-v1. Thay đổi naming/catalog và npm workspace; không đổi runtime UI/API/DB.

- npm ci và npm run build tại root qua cho cả Desktop và workspace mới minhanos-files.
- Root check qua: catalog JSON Schema, uniqueness App ID/module/key/alias, actual source paths,
  manifest membership và package name alignment; TypeScript/ICU/OpenAPI kiểm tra hiện có vẫn qua.
- Compatibility tests chặn xóa App ID/alias và cho đổi display name; duplicate/traversal fixtures bị từ chối.
- OpenAPI breaking check không phát hiện thay đổi API; catalog mới chưa có predecessor baseline.
- Docker compose build drive qua bằng workspace mới; image runtime digest giống bản trước, không cần restart service.
- Không rename repo GitHub/remote, source paths, ports, compose service, volumes, deployment folder hoặc URLs.
- Vite còn warning import attributes JSON không đồng nhất đã có từ trước; build thành công.
- Không chạy lại kiểm thử DB migration/backend API hoặc browser vì không sửa runtime/backend/UI trong đợt này.
- GitHub CI cho nhánh naming chưa được xác nhận tại thời điểm ghi nhận này.

---

# Kiểm chứng migration runner — 2026-10-30

Nhánh platform-migrations-v1. Kết quả thực tế trên VM và PostgreSQL disposable riêng.

- Plan validation qua: DAG order, duplicate/missing modules, cycle, version bounds và gap IDs.
- DB integration qua: sáu concurrent Apply chỉ ghi một lần; sửa checksum bị từ chối;
  SQL lỗi/hủy context rollback DDL+history; binary runner-aware cũ bị chặn khi schema mới hơn.
- Đã terminate PostgreSQL backend giữa transaction: DDL không commit; cùng migration chạy lại thành công.
- Profile fresh/adopt giữ nguyên locale/theme/timezone; schema không tương thích bị từ chối và không để ledger dở.
- Backup DB thật restore vào DB/network riêng qua psql ON_ERROR_STOP; profile upgrade qua và fingerprint
  toàn bộ users/profiles không thay đổi. Container rehearsal không publish port và đã được dọn.
- Docker build Desktop/Drive qua: Go tests/vet và Vue/TypeScript. Tests DB opt-in được chạy riêng;
  go test thông thường skip các tests không có biến môi trường, không coi skipped là passed.
- Root npm run check qua: foundation, schemas, generated types, strict TypeScript contract check, OpenAPI lint.
- Đã phát hiện/sửa lỗi cũ: generator xuất tên từ $id thay vì tên public, thêm newline cuối file không ổn định;
  kiểm tra mới xác nhận SDK Profile không phải any và enum không bị nới lỏng.
- Live profile smoke qua sau migration: schema thật, hai-user isolation, CSRF/Origin, Desktop/Drive profile.
  Files gateway smoke đủ bốn nhóm qua. Disposable users đã dọn theo IDs cụ thể.
- Dừng Desktop thật: Drive frontend/login/files/logout vẫn qua; Desktop đã khởi động lại.
- Runtime ledger có đúng profile v1 applied, runner_version=1; services Desktop/Drive/DB healthy.

Chưa triển khai identity adoption, nontransactional/backfill/destructive/down runner, app grants, jobs/upload
bền vững hay event bus. Không kiểm thử recovery volume Files, không chạy race detector Go hoặc kiểm thử UI mới.
CI GitHub đã qua toàn bộ workflow cho implementation commit `fbc07c4c6f16b5faf1bccff0ba53390856792079`,
bao gồm DB integration: [run 41003701847](https://github.com/minhtribtc/MinhAnOS/actions/runs/41003701847).

---

# Kiểm chứng Platform Contracts v1 — 2026-10-30

Nhánh platform-contracts-v1. Kiểm tra thực tế trên VM, chưa xác nhận trạng thái workflow GitHub.

- JSON Schema draft 2020-12: manifest/authz/errors/profile/file/ACL/job/upload/event compile qua Ajv strict.
  Fixtures hợp lệ qua; từ chối app ID sai, capabilities giả cấp, subject do client gửi,
  trạng thái job không biết, checksum sai, part trùng và ACL principal/quyền không hợp lệ.
- Manifest kiểm tra semver/version SDK 0.1.0, quyền đã đăng ký và version Files package; ^1.0.0 không tương thích.
- Shared-types sinh từ schema; generated drift check và OpenAPI/component-schema parity qua.
- Redocly lint OpenAPI 3.1 qua, không warning sau khi dùng server URL tương đối.
- oasdiff pinned digest: initial contract chưa có predecessor spec nên bootstrap self-compare.
  Negative test xóa GET /api/profile trả exit 1 và api-removed như mong đợi.
- Go test/vet và Docker build cả Desktop/Drive qua, gồm fixture wire error legacy dùng chung Go/JS
  (18 Go tests). Vue/TypeScript build qua sau tách API client và dùng generated Profile.
- npm run check qua cả foundation và contracts. Files client CSRF/base isolation test qua.
- Live profile smoke qua với Ajv validation trên response thật: profile + legacy errors, hai-user isolation,
  Desktop/Drive/profile đồng bộ, CSRF/Origin và logout. Đã xóa đúng hai tài khoản thử.
- Bản mới đã chạy trên VM; API legacy và DB schema không đổi trong đợt này.

Một lần chạy shell check trên VM lỗi do CRLF từ Windows; đã chuẩn hóa LF và chạy lại qua.
Không kiểm thử worker crash/resume, durable upload, app delegation, ACL storage, event dedupe/outbox hoặc
migration restore, vì các dịch vụ đó chưa được triển khai. Không có UI mới để nghiệm thu trong thay đổi này.
Contract tests/CI không thay thế kiểm chứng các dịch vụ planned khi chúng được xây.

---

# Kiểm chứng foundation preview — 2026-10-30

Nhánh platform-foundations-v1. Kiểm tra thực tế trên VM 10.1.1.20.

- Docker build Desktop và Drive thành công: vue-tsc, Vite; cả hai build chạy kiểm tra foundation.
- Go test ./... và go vet ./... qua trong Docker build, gồm 17 tests và coreprofile/apicontract mới.
- Foundation tests qua: ICU plural, vi/en formatting, timezone, namespace ownership/unregister,
  catalog key/ICU/parameter mismatch, provider isolation, unknown error fallback, CSRF client isolation và dependency boundaries.
- profile-smoke qua: hai user cô lập, phiên đăng nhập khác nhận lại locale/theme/timezone,
  Desktop/native gateway/Drive cùng DB chia sẻ profile; từ chối locale/timezone/body không hợp lệ,
  thiếu CSRF, Origin lạ; logout thu hồi quyền. Đã dọn đúng hai tài khoản thử.
- API smoke: 10 nhóm qua. Files gateway: 4 nhóm qua, gồm assert code CONFLICT trên copy trùng tên.
  Drive smoke: 10 nhóm qua, gồm file agent Debian và backup/concurrency. Đã dọn fixtures/users cụ thể.
- Action registry và Files client tests qua sau chuyển shared API client.
- Dừng Desktop thật: Drive frontend/login/files/logout vẫn qua; Desktop đã khởi động lại.
- Đã cập nhật host files-agent binary, restart systemd. Desktop/Drive/DB đều healthy.
- Browser: chọn English cập nhật menu/action labels, nhãn hồ sơ và đồng hồ; đổi dark cập nhật Files/menu portal.
  Drive standalone cũng đã kiểm tra dark theme thực tế trong browser.
  Bộ lọc "Tài" còn khi đổi locale/theme và khi Home → mở lại Files. Đã khôi phục vi-VN/light sau thử.
  Bằng chứng cục bộ ignored: artifacts/foundations-files-dark.jpg. Không phát sinh thao tác thay đổi tệp từ UI test.

- Kiểm tra 31 liên kết tài liệu nội bộ qua; git diff --check sạch sau chuẩn hóa newline.

Giới hạn: chưa kiểm tra đang upload khi đổi locale, draft editor/Terminal, contrast/keyboard audit toàn diện,
chưa kiểm tra fresh standalone DB lại trong đợt này; test độc lập dùng DB hiện có.
Workflow GitHub CI đã cấu hình; chưa coi là CI xanh khi chưa đọc kết quả trên GitHub.

Một lần api-smoke dùng default 127.0.0.1:8081 thất bại ECONNREFUSED vì VM chỉ bind 10.1.1.20;
đã chạy lại với TEST_URL=http://10.1.1.20:8081 và qua đủ 10 nhóm. Đây là lỗi cấu hình test invocation.
Một test fallback locale mới ban đầu giả định môi trường Node mặc định vi-VN và làm build dừng;
đã sửa assertion theo navigator languages thực tế, giữ đúng thứ tự fallback quy định và chạy lại build.
Chuyển đổi UI vẫn từng phần, không nghiệm thu toàn bộ Foundation/i18n V0.1.

---

# Kiểm chứng bản desktop 0.1

## Bổ sung: Fluent và focus workspace

- Build Go/tests/vet và Vue/TypeScript thành công sau cập nhật workspace mode.
- Chế độ focus là mặc định cho preferences cũ chưa có mode; window mode vẫn được hỗ trợ.
- Đã nhập form tạo user chưa submit: `draft_state_test` và tên hiển thị thử.
  Chuyển sang Overview rồi trở lại Settings giữ nguyên tab, form và hai giá trị.
- Kiểm tra DOM xác nhận Settings bị ẩn nhưng vẫn có đúng một instance mounted.
- Thu nhỏ/mở lại Settings vẫn giữ tên hiển thị nháp. Không submit form thử này.
- API smoke 10 nhóm kiểm tra chạy lại thành công; có thêm assertion lưu mode và
  default focus riêng của user mới.
- Calendar và launcher được triển khai bằng Vue, không dùng iframe.
- Terminal, upload và durable jobs chưa triển khai; xem docs/roadmap.md cho hợp đồng lifecycle.

Ngày: 29/10/2026. VM: 10.1.1.20; cổng: 8081.

## Build

- `vue-tsc --noEmit` và Vite production build: thành công.
- `go test ./...`: thành công; gồm password/salt, Origin, preferences không tin
  cậy, session bắt buộc và đăng ký router.
- `go vet ./...`: thành công.
- Docker multi-stage build: thành công. Base images được pin digest; npm/go có lockfiles.
- API container chạy UID/GID 10001, rootfs read-only, không có Docker socket.
- Desktop và PostgreSQL báo healthy. Bản tham chiếu cổng 8080 vẫn chạy.

## Kiểm thử tích hợp API

`tests/api-smoke.mjs` đã thực thi 10 nhóm kiểm tra:

1. Khách chưa đăng nhập không đọc được preferences hoặc quản trị user.
2. Password sai không đăng nhập được.
3. Admin đăng nhập bằng cookie HttpOnly/SameSite Strict.
4. Thiếu CSRF hoặc Origin khác bị từ chối khi ghi dữ liệu.
5. App ID không tồn tại không được lưu trong window state.
6. Trạng thái PostgreSQL/heap/filesystem lấy từ runtime; integrations unconfigured.
7. User thường không được GET/POST API quản trị user.
8. Preferences/notes tách biệt giữa admin và user, giữ qua session mới.
9. User thường chỉ thấy audit của tài khoản mình.
10. Logout vô hiệu session trên server.

Script khôi phục preferences admin; tài khoản e2e tạo riêng được dọn sau test.
Không ghi password/session token vào báo cáo JSON.

## Kiểm thử trong trình duyệt

- Đăng nhập admin và render overview có dữ liệu thật.
- Tìm `ghi chú` trong launcher và mở đúng module.
- Nhập ghi chú tiếng Việt, tự lưu; reload vẫn còn nội dung.
- Thu nhỏ ghi chú, mở lại từ taskbar.
- Resize bằng bàn phím: kích thước đổi từ 680×560 sang 660×540 trong lần thử.
- Kéo ghi chú: x đổi từ 303 sang 413; reload giữ x=413 và width=660.
- Phóng to, khôi phục, đóng ghi chú.
- Đổi theme tối và wallpaper midnight; reload xác nhận `platform midnight dark-mode`.
- Trả về theme sáng/mountain cho bàn giao.
- Viewport 390×844: cửa sổ hiển thị x=12, y=54, width=366, height=674, nằm hoàn
  toàn trong viewport; nội dung cuộn. Đã trả viewport về mặc định sau thử.
- API và PostgreSQL được recreate khi chuyển image digest; session/preferences
  vẫn có sau khi reload bản mới.
- Đã kiểm tra ảnh login, desktop sáng, settings tối và màn hình nhỏ.

Ảnh bàn giao: `artifacts/desktop-0.1.jpg` (local, được gitignore).
Kết quả API: `artifacts/api-smoke.json` (local, được gitignore).

## Giới hạn

Đây không phải pentest, kiểm thử tải, backup/restore drill hay xác nhận production.
Chưa kiểm tra SSO/NetBird/1Panel vì chưa cấu hình; File Station chưa triển khai.
Chưa kiểm thử nhiều thiết bị thực, mọi trình duyệt, screen reader đầy đủ hoặc
khôi phục sự cố disk/database. HTTP LAN dùng cookie không Secure theo cấu hình
test; phải đổi cùng HTTPS trước khi công bố ra ngoài.

## V2 iPadOS-inspired interface — 2026-10-30

- Docker frontend build passed vue-tsc and Vite. Backend go test ./... and go vet ./... passed during image build.
- Both Compose services healthy; desktop serves version 0.2.0 on port 8081.
- API smoke: all 10 checks passed (guest rejection, wrong password, cookies, CSRF/Origin, app validation,
  real metrics, admin restrictions, isolated preferences/audit, revoked logout session).
- Exact disposable e2e user removed after test (DELETE 1); no browser form submitted.
- Browser: Home, Dock app launch, Spotlight query/open, switcher resume, minimize/reopen passed.
- Draft display name and username remained after Home → switcher → app and minimize → Dock reopen.
- Control Center light/dark toggles worked; dark-mode persisted through reload; returned to light for handoff.
- Modal background is inert; initial focus moves to dialog and returns to trigger on dismissal.
- 390x844: Home and Settings inspected, document width 390 (no page overflow); Dock scrolls horizontally.
- 1024x768: Home inspected, document width 1024. Home content scrolls vertically when widgets exceed height.
- Browser viewport reset to default. No warn/error console entries during checks.
- Screenshot: artifacts/ipados-v2.jpg (local, ignored).

Remaining: full accessibility audit, real touch devices, Split View, durable jobs and service integrations.
Previous sections describe V1 verification history and should not be interpreted as V2 measurements.

## File Station design verification — 2026-10-30

Interactive prototype only: folder navigation, name filtering, selection/details,
new mock folder, mock trash, layout variants and upload pause checked in a browser.
An initial mock folder submission did not run in the sandbox; changed the action to an explicit local click handler and retested successfully.
No backend/source runtime behavior changed; Go/API tests were not repeated for documentation/mockup work.

## Drive 0.1 implementation verification — 2026-10-30

- Docker builds: standalone Drive frontend vue-tsc/Vite passed; desktop launcher frontend passed.
- Go: all 6 Drive tests and 6 existing platform tests passed; go vet ./... passed in build.
- Tests cover namespace isolation/auth, rooted traversal and symlink rejection, no-overwrite upload,
  trash restore collisions, stale text ETags/backups and explicit system-delete confirmation.
- A listing test caught descriptor-relative metadata lookup returning no files; fixed with openat/O_PATH on the directory descriptor.
- Live Drive integration: 10 checks passed, including real file bytes, two-user isolation, CSRF/Origin,
  trash ownership, optimistic edits, admin-only host access, root-owned fixture write/edit/backup,
  /etc/hostname read and blocked /proc access. Temporary /srv fixture removed.
- Existing platform smoke: all 10 checks passed after extracting shared authn primitives.
- Exact disposable users/private namespaces cleaned; platform test account removed.
- Desktop stopped during tests/drive-independent.mjs: frontend, independent login, personal list and logout passed; desktop restarted.
- Fresh standalone Compose + independent database on localhost:8083: bootstrap/login/files/logout passed without desktop or host agent.
  Isolation containers/network/test volumes removed after successful check.
- Host agent: systemd active; Unix directory 0750 root:10001, socket 0660 root:10001; restart retained access from Drive.
  Initial systemd start required creating reserved GID 10001; RuntimeDirectoryPreserve prevents stale bind mounts after restart.
- Browser: independent login, real folder creation, file-chooser upload, text preview/edit/save with backup,
  private trash, folder back navigation, grid/list and Debian root browse verified.
- Mobile 390x844: no horizontal document overflow; inspector reflows below browser. Viewport reset afterward.
- Persistent admin demo artifacts: personal folder Tài liệu and uploaded/edited Chào mừng Drive.txt; its initial backup is in private trash.
- No full accessibility, load, quota, disk crash/recovery or resumable-upload claim. See drive.md for remaining limits.
- Desktop Home/Dock launcher verified opening the independent Drive URL in a separate browser tab; popup-block fallback is available.
- Screenshot: artifacts/drive-v1.jpg (local, ignored). Browser override reset before handoff.

## Native File Station verification — 2026-10-30

- Both Docker frontend builds passed vue-tsc/Vite against the same packages/files-app source.
- Go tests/vet passed: 8 Drive tests and 7 platform tests, including copy/no-overwrite, optional adapter role selection and anonymous gateway rejection.
- tests/files-desktop-smoke.mjs passed 4 integration groups: existing desktop session, login/logout gateway denial,
  independent Drive-admin cookie cannot elevate desktop user, CSRF/Origin checks, streaming upload beyond 128 KiB,
  copied bytes/collision protection, per-user isolation and access revoked by desktop logout.
- Existing Drive smoke: all 10 groups passed, including real Debian host operations. Exact disposable users/namespaces cleaned.
- Action registry and per-instance client tests passed under Node strip-types; client API base/CSRF state do not leak across instances.
- Desktop stopped: tests/drive-independent.mjs passed. Drive remains independent; desktop was restarted afterward.
- Browser: Files opened natively on 8081 without Drive login; original private files visible. No iframe or navigation to 8082.
- Browser: right-click menu, overflow menu, disabled directory copy, Ctrl-click multi-selection with right-click preserving the group,
  disabled multi-rename, real copy/paste, real cut/paste, F2 rename and Escape menu dismissal verified.
- Browser: directory and clipboard persisted through Home/reopen. Unsaved UTF-8 editor draft persisted through Home/reopen and was not saved to disk.
- Narrow native window (~578px content): container query reflow; scrollWidth equals clientWidth (no horizontal overflow).
- Native confirm close-guard test encountered an automation/browser dialog deadlock; cancel/accept flow could not be completed by the browser tool.
  A fresh desktop preview was used afterward. This is an explicit verification limit, not a passing UI test.
- Ctrl+X key injection did not trigger in the automation browser; the same Cut command succeeded through overflow menu.
  F2 and Escape were verified; complete browser/OS keyboard accessibility remains pending.
- Demo file Hướng dẫn sử dụng.txt was created by copy then renamed/moved in admin's personal space; original Chào mừng Drive.txt unchanged.
- Backups: before-files-desktop.tgz and before-files-desktop.sql in /opt/minhanos-backups, root-only. No schema/data migration required.
- Final shared UI loaded in both desktop (?app=files on 8081) and standalone Drive (8082) using existing sessions.
- Evidence image: artifacts/files-in-desktop.jpg (local, ignored). No iframe, URL navigation to Drive, or second login required for the embedded view.

## Documentation foundations — 2026-10-30

Added user guide, documentation index and platform-foundations specification; updated agent rules and handoff.
The guide distinguishes shipped functionality from planned i18n/theme/SDK behavior and retains known test limitations.
No application code/schema/deployment changed, so runtime builds and API suites were not repeated for this documentation-only update.
- Documentation checks passed: 13 Markdown documents, 25 local links resolved; git diff --check passed.
