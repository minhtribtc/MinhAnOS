# MinhAnOS development contract

Read README.md, docs/HANDOFF.md, docs/architecture.md and docs/roadmap.md before editing.
For file/Drive work also read docs/drive.md and docs/api.md. Current implementation branch is file-station-v1.
Drive must remain independently buildable/runnable. Never import desktop stores/components into apps/drive,
or internal/platform into internal/drive/cmd/drive. Desktop launches a URL; do not replace this with an iframe dependency.

## Mandatory documentation on every change

- Update docs/HANDOFF.md with current behavior, verification, limitations and next steps before handing work back.
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

Frontend: apps/desktop, npm ci && npm run build (Node 22.12+).
Backend: go test ./... and go vet ./... (Go 1.26+), also run during Docker build.
Integration: tests/api-smoke.mjs against the test VM only; creates a disposable user, restore preferences and remove that exact test user.
Browser: Home, app opening, search, switcher, Control Center, dark theme, mobile overflow and draft retention across Home/app changes.
Use real browser evidence; screenshots and generated runtime artifacts stay ignored.
