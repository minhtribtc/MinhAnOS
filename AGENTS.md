# MinhAnOS development contract

Read README.md, docs/HANDOFF.md, docs/architecture.md and docs/roadmap.md before editing.
Also read docs/platform-foundations.md (accepted architecture/i18n/theme direction) and docs/user-guide.md.
Architecture target: Platform Core + shared Design System + App SDK + peer apps + replaceable Shells.
Do not treat planned packages/contracts as implemented; document actual migration status.
For file/Drive work also read docs/drive.md and docs/api.md. Current implementation branch is settings-central-v1; also read docs/extension-contracts.md.
Account Center UI is packages/accounts-app (native + standalone /accounts/); keep it free of Desktop imports.
Drive and packages/files-app must remain independently reusable. Never import desktop stores/components into apps/drive or packages/files-app,
or internal/platform into internal/drive/cmd/drive. Desktop renders packages/files-app natively using /api/drive; Drive uses the same package with /api. Never introduce an iframe dependency.

## Mandatory documentation on every change

- Update docs/HANDOFF.md with current behavior, verification, limitations and next steps before handing work back.
- Update docs/user-guide.md in the same commit whenever user-visible behavior, commands, settings, limits or troubleshooting change.
- Maintain docs/README.md as the documentation index; add links for new documents.
- Update CHANGELOG.md for user-visible changes. Update README.md when setup or usage changes.
- Update docs/architecture.md and docs/api.md when architecture or API contracts change.
- Record actual checks and failures in docs/validation.md. Never mark an unperformed test passed.
- Keep documentation in the same commit as the implementation. Git history is the source of commit identifiers.

## Architecture and state

Vue/TypeScript shell, Go API, PostgreSQL. The shell is independent of 1Panel.
Preserve AppContent instances when switching apps, returning Home or minimizing.
Use keyed v-for with v-show; closing intentionally destroys the instance and unsaved local form drafts.
Preferences persist layout/theme/notes; they do not persist arbitrary forms, terminal sessions or uploads.
Do not represent integration placeholders as working services or API process metrics as whole-host metrics.
Keep existing authentication, CSRF/Origin validation and per-user data isolation.
Host operations must use a future restricted agent API; never add arbitrary root shell execution endpoints.

## Repository and deployment

Branch desktop-v2-ipados is UI preview 0.2. desktop-v1 preserves the previous interface; main holds the kodbox reference.
VM test: 10.1.1.20; source /opt/minhanos-next; web port 8081. Original reference remains on 8080.
Read docs/HANDOFF.md before deployment. Preserve .env, secrets and database volumes.
Never commit secrets, credentials, private keys, node_modules or artifacts. Do not print bootstrap passwords in logs.
Do not run docker compose down -v on this VM.

## Verification

Frontend: root npm ci && npm run build (Node 22.12+); use the root lockfile only.
Backend: go test ./... and go vet ./... (Go 1.26+), also run during Docker build.
Integration: tests/api-smoke.mjs against the test VM only; creates a disposable user, restore preferences and remove that exact test user.
Browser: Home, app opening, search, switcher, Control Center, dark theme, mobile overflow and draft retention across Home/app changes.
Use real browser evidence; screenshots and generated runtime artifacts stay ignored.

## i18n and theme contract

- New UI text must use structured translation keys (e.g. files.actions.delete), never Vietnamese sentences as keys.
- Catalogs belong to their apps; keep Core/common catalogs minimal. Shared Files/Drive UI shares files.* catalogs.
- Use @minhanos/i18n and @minhanos/app-sdk; do not add per-app translation/provider workarounds.
- Persist locale through the user profile; support unauthenticated fallback and keep timezone separate from locale.
- Use ICU/Intl for messages, plural forms and locale formatting; do not concatenate translated sentence fragments.
- Add stable backend error codes; migrate compatibly from existing error strings. UI behavior must not parse translated messages.
- Theme provider/tokens and localization must not depend on Desktop. Apps must work under another Shell or standalone host.
- Migrate existing hardcoded strings/colors incrementally and record remaining debt. Do not claim i18n/theme completion prematurely.
- Run npm run check and normal builds (catalog/ICU/parameter/boundary checks); full hardcoded-text/token scanning is still pending.


## Platform contracts

Read contracts/README.md and docs/adr/0001-platform-contracts-v1.md before new integration work.
Use versioned schemas; regenerate shared-types with npm run contracts:generate and run npm run contracts:check.
Never treat manifest requested capabilities or a client appId as granted/verified app authority.
Current SDK package is 0.1.0; host protocol sdkVersion=1 is independent. Do not silently bump SDK to 1.
Preserve legacy error envelope on /api; nested v1 is for future versioned endpoints.
Do not advertise planned jobs/uploads/authz/events/fileId as implemented. Migration runner is transactional; profile and identity adoption are implemented. Other module adoption remains planned.


Read docs/adr/0002-transactional-migration-runner.md before schema changes.
Never edit an applied embedded migration; append a contiguous step. Do not bypass checksum/history failures.
Run sh tests/migrations-integration.sh against its dedicated disposable DB, never the application database.


Read docs/adr/0003-app-naming-and-repository-policy.md for names and App IDs.
Validate first-party identity changes through contracts/apps/catalog.v1.json; never repurpose an existing App ID.
Files host workspace is minhanos-files; its source folder/API/compose service remain drive for compatibility.
Keep Shell variants classic/touch separate from appearance light/dark/system and workspace layout focus/windows.

## Account Center continuation

User approved Phase 1; read docs/account-center/PHASE1_REPORT.md. Identity SQL 001–003 are applied and immutable. UUID completeness is not API readiness. No Identity HTTP handlers/UI/RBAC grants are active yet. Next AuthCore must preserve both session realms and existing storage IDs; host privilege split and last-admin protection gate new admin grants. Never create a third users/session/password store.

## Phase 2 continuation (current)

User delegated routine architecture decisions with documentation. Read docs/account-center/PHASE2_REPORT.md and docs/adr/0004-shared-auth-core-and-recovery.md; supersedes Phase 1 next-step notes. Shared AuthCore is active on all legacy adapters, SQL identity 001–004 immutable. No v1 Identity HTTP/UI or RBAC grants yet. Root recovery CLI is Linux-local only; never expose local methods to HTTP. Phase 3 must enforce last-admin invariant and split host-root privilege before new administrative authority. Keep full same-commit docs/test evidence.

## Phase 3A continuation (current)

Read docs/account-center/PHASE3_REPORT.md and ADR 0005 before changes. Identity SQL 001–005/profile 001 immutable. Phase 3A is implemented, not full Phase 3. Fourteen v1 operations active; use contracts/identity/status.v1.json as inventory. users.role is still canonical System Admin authority; never add another bypassing binding/group path. Every policy mutation shares lock 681026 before user rows, last-admin and transaction-time actor/grant checks. Host operator is a separate one-time snapshot, never auto-granted to later web admins/startups. Keep legacy adapters under policy. Finish Phase 3B group/custom-role/app policy and session/password DTO adapters before claiming full Phase 3 or UI readiness. v1 must fail closed on incomplete UUID backfill; do not hide unmapped users.

## Verification preference

User requested avoiding time-consuming repeated interactive test loops. Prefer focused automated checks once per meaningful change; do not repeat browser/VM smoke or stop-start loops after suitable checks pass unless a concrete failure or new change requires it. Finish implementation, documentation and delivery promptly.

## Phase 3B continuation (current)

Read docs/account-center/PHASE3B_REPORT.md and ADR 0006; supersedes earlier active-operation counts. 32 operations active, no migration beyond 005. Custom role/group bindings cannot grant canonical System Admin or host operator; governance permissions are capped and system scope exact. Password changes revoke both realms/current session; DTOs expose no credential/session secrets. Effective access differs from feature availability. Next shared native/standalone Account Center UI; app policy editing remains gated until resource enforcement/registry support. Respect focused verification preference: no repeated interactive/browser/stop-start trials after suitable checks pass.

## Phase 4 Account Center UI continuation (current)

Read docs/account-center/UI_V1_REPORT.md; supersedes Phase 3B next-step notes. 33 operations active, no migration beyond 005. UI lives in packages/accounts-app, rendered natively by Desktop and standalone at /accounts/ on both hosts. Keep package-lock in sync when adding workspaces (npm ci must pass). App access policy editing stays hidden until Files/registry enforcement exists.

## Central Settings standard (mandatory for every app, from 2026-10-30)

User rule: every app must integrate uniformly with the central Settings app (com.minhanos.settings), like Settings on iOS/macOS/Android. Status: thin slice implemented (ADR 0007): manifest-driven Settings shell, SDK providers/events/opener, tests/settings-standard.mjs. Generic per-app settings storage API is still PLANNED.
- One place: system-wide and per-app preferences live in Settings. Apps may keep in-context controls, but must not ship their own separate global settings page; "Settings" buttons inside apps open the central Settings at that app's panel.
- Declare: each app manifest lists its panels in settingsIntegration (id, labelKey, path, requiredPermission). Settings builds its navigation from manifests of installed apps, never from hardcoded tabs.
- Provide: the app package exports its settings panel components; Settings renders them natively through app-sdk (no iframe, no Desktop imports). The same panel works in the app's standalone host.
- Store: settings use a versioned schema per app, scope user or system, validated and permission-checked on the backend; system-scope changes are audited. No localStorage/ad-hoc tables for real settings.
- Look: only @minhanos/ui building blocks (section, row, toggle, select, danger zone) and theme tokens; i18n keys <app>.settings.*, vi-VN and en-US parity.
- Navigate/search: deep link to Settings/<appId>/<path>; every entry indexed for Settings search. Hidden entries for missing permission are cosmetic; backend must still enforce.
- Definition of done for any app: manifest settingsIntegration (or explicit empty list), panels, backend schema/validation, tests, docs/user-guide entry.
- Remaining debt: Appearance panel text is hardcoded Vietnamese; generic settings storage API and system-wide sections are planned. Read docs/adr/0007-central-settings.md before adding an app.
- Ownership (hybrid, user decision 2026-10-30, ADR 0007 amendment): each setting has exactly one owning module/namespace (desktop.*, system.*, files.*, identity.*, storage.*); writes only through that owner's API and permission checks, never a global setSetting. Platform owns cross-app settings (locale, time zone, theme, date format, notifications): edited once in Settings, read reactively via app-sdk; apps must not add their own selectors for them when a Settings host exists (canOpenSettings), standalone hosts may keep a fallback. The app owns its settings UI/API; Settings renders the same exported component. Personal account entries live in Settings; user/group administration stays in Account Center. System sections (Network, Power, Updates...) appear only with a real backend.

## Account Center usable v1 (current)

Read docs/account-center/USABLE_V1_REPORT.md. Identity SQL 001–011 are applied and immutable; append 012+. Storage module SQL 001–004 applied; append 005+. Notifications module SQL 001 applied; append 002+. Notices reach users only through a module outbox consumed by notifycore.Deliver (no browser publish API); audit tables are read by Log Center through logcore.Sources (docs/notification-log-center.md). Module APIs live under /api/v1/<module> with contracts/<module> schemas and a client generated by scripts/generate-identity-client.mjs (generateClient). Files authorization: Drive must call IdentityStore.Authorize (identity CheckApp) before resource rules; only apps in governedApps get editable access rules, never Account Center. Use requestid.From(ctx) for request/correlation IDs; never write identity_auth_events; security notices go to identity_notification_outbox via notify(). 34 operations active. Sign-in success/logout audit is transactional; failures are sampled (1/5/10/20) for existing accounts only. Web admins must never reset a host operator's password. Audit lists are newest first by (occurred_at,id).
- Plain HTTP LAN access is a supported deployment: never call secure-context-only APIs (crypto.randomUUID, navigator.clipboard, crypto.subtle) unguarded in frontend code; use createIdempotencyKey from app-sdk. tests/insecure-context.mjs enforces the UUID part.
- Drive.Dockerfile copies Go packages one by one; tests/docker-context.mjs fails when a new internal package is missing.

## Storage layer (current)

Read docs/storage-model-review.md and docs/storage-agent.md. Managed data volumes are ZFS and Btrfs only (ext4 = system disk). Only cmd/storage-agent runs zfs/btrfs commands (exec with validated arguments, never a shell, no delete endpoint); web services authorize and audit before calling it. Never add mount-namespace sandboxing (PrivateTmp, Protect*, ProtectSystem) to the storage agent unit. Quota tests must write random data (volumes compress). Test VM: pool mnzfs on scsi1, Btrfs /srv/minhanos/btrfs on scsi2; never touch sdc (system disk).
- Soft quota: every Files write that adds data must Reserve → write → Commit/Release through storagecore under the user lock; never enforce by scanning. Storage Manager API requires storage.manage and reauthentication for writes.
- First-run rule (user, 2026-10-31): a fresh system has no default home area; the first System Admin must create a Shared Folder that hosts homes before anyone has a home. Keep the legacy Files data registration only on upgrades that actually have data. See docs/files-app-review.md.
- Shared Folders (Q2, docs/storage-q2.md): access only through storage_share_access (deny > write > read, no entry = none); Files checks the identity grant AND share access on every request; homes on ZFS/Btrfs are agent units homes/<storage ID>, never paths built by hand. Exactly one share hosts homes.
- Quota folders (Q3, docs/storage-q3.md): any Files mutation in a Shared Folder must go through planFolders/settleFolders (reserve on enclosing folders, rewrite paths on rename/move, protect from trash); never compute folder usage by scanning on a request path.
- Review every app against docs/app-standard.md (26 rows incl. notifications, logs, API contract, security, privacy, backup, jobs, accessibility) before calling it compliant; reviews: docs/files-app-review.md, docs/desktop-review.md.
