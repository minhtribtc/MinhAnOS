# MinhAnOS development contract

Read README.md, docs/HANDOFF.md, docs/architecture.md and docs/roadmap.md before editing.
Also read docs/platform-foundations.md (accepted architecture/i18n/theme direction) and docs/user-guide.md.
Architecture target: Platform Core + shared Design System + App SDK + peer apps + replaceable Shells.
Do not treat planned packages/contracts as implemented; document actual migration status.
For file/Drive work also read docs/drive.md and docs/api.md. Current implementation branch is platform-migrations-v1; also read docs/extension-contracts.md.
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
Do not advertise planned jobs/uploads/authz/events/fileId as implemented. Migration runner is transactional profile-first; other module adoption remains planned.


Read docs/adr/0002-transactional-migration-runner.md before schema changes.
Never edit an applied embedded migration; append a contiguous step. Do not bypass checksum/history failures.
Run sh tests/migrations-integration.sh against its dedicated disposable DB, never the application database.
