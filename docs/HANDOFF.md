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
