Phase 3A: system registry/file routes require an explicit enabled host-operator entry AND active System Admin. Later web admins gain no host access automatically. Both cookie realms share this check; an adapter without host policy denies. Existing operator root-agent scope remains broad. Drive independently hosts the same supported Identity v1 slice. [ADR 0005](adr/0005-identity-policy-and-host-operators.md).

# MinhAnOS Drive 0.1 / File Station

Drive is an independently runnable application, not a desktop component or iframe.
Test URL: http://10.1.1.20:8082. Desktop on 8081 renders the same Files UI inside its native application host.
Branch: file-station-desktop. Shared UI: packages/files-app. Standalone host: apps/drive. Backend: cmd/drive, internal/drive, internal/authn.

## Independence contract

- Shared files-app has no desktop/Pinia imports. apps/drive is an independent build/login host; desktop mounts the same component with explicit configuration.
- Backend does not import internal/platform or call desktop/platform HTTP APIs.
- Drive.Dockerfile builds without copying desktop frontend or platform backend sources.
- compose.drive.yaml runs Drive + PostgreSQL without a desktop service. An empty standalone DB bootstraps its own admin from the secret file.
- Current integrated deployment shares the users table/password format with MinhAnOS. Embedded mode uses the platform session through the authenticated gateway; standalone Drive keeps a separate browser session.
  Drive uses its own drive_sessions table and minhanos_drive_session cookie. Logging out of one app does not log out the other.
- IdentityStore is an adapter boundary for future OIDC or a separate identity database. Shared authn contains password/token primitives only.
- Drive does not require parent window messaging, a desktop token, shell state, localStorage credentials or desktop routes.
- Standalone account management is currently bootstrap-admin only; provisioning more users needs an identity/admin interface in a later release.

## Delivered

Private filesystem namespace per user; grid/list, hidden files, folder navigation/back/up,
name filtering in the current folder, metadata/UID/GID/mode, create folder, raw upload/download,
rename, same-filesystem move, regular-file copy without overwriting, trash and restore. Filename collisions return 409.
Text preview/edit for UTF-8 up to 256 KiB, optimistic concurrency and a backup next to the original file before atomic replacement.
Real upload progress, cancel and retry from the in-memory File; switching folders keeps transfers alive.

The admin-only Debian location uses a separate host process over a Unix socket. It supports list,
read/download, create/upload, rename/move, text edits with backup and explicit permanent removal
of regular files or empty directories. No recursive system deletion. No shell execution API.
Host editing refuses hard links, special modes and ACL/xattr files because this first editor cannot preserve their semantics.
Symlinks are shown but not followed. /proc, /sys, /dev and /run namespaces are blocked by path;
special files cannot be opened as ordinary files. Mode/chown/ACL editing is not implemented.

## Run integrated

Existing .env/secrets and database volumes must be preserved:

```sh
docker compose up -d --build
docker compose ps
```

Integrated services share PostgreSQL but run separate non-root, read-only-rootfs app containers.
drive-files mounts at /data only in Drive. Do not mount the Debian root into the web container.
Default ports: desktop 8081, Drive 8082. DRIVE_PORT configures published port; VITE_DRIVE_URL overrides the desktop launcher URL at build time.
Desktop uses DRIVE_INTERNAL_URL on the server for /api/drive/*; the browser stays on port 8081. VITE_DRIVE_URL is retained only as legacy configuration and is not required for native embedding.

## Run standalone

```sh
# .env: POSTGRES_PASSWORD, MINHANOS_BIND_IP, DRIVE_PORT, COOKIE_SECURE
# secrets/bootstrap_password: >=12 characters, readable by container uid 10001
docker compose -f compose.drive.yaml up -d --build
```

Standalone uses independent drive-database/drive-files named volumes. It does not automatically copy users or data from the integrated deployment.
Its default configuration has no host agent; only personal storage is advertised.
To move existing data, migrate matching users/IDs and Drive sessions/audit as needed, plus the entire drive-files volume.
Back up DB and filesystem together; do not regenerate user IDs or copy only the visible files directory.

## Host agent (optional)

1. Build Drive image and extract /app/files-agent into /usr/local/libexec/minhanos-files-agent (root-owned, mode 0755).
2. Reserve/create host GID 10001 for socket access; e.g. getent group 10001 or groupadd --system --gid 10001 minhanos-files.
3. Install deploy/minhanos-files-agent.service under /etc/systemd/system; daemon-reload and enable --now.
4. Socket: /run/minhanos-files/agent.sock, owner root:10001, mode 0660; parent 0750.
5. Drive mounts only the socket directory read-only and sets DRIVE_AGENT_SOCKET. No TCP listener or shared root volume.

RuntimeDirectoryPreserve=yes keeps the mounted directory inode across agent restarts.
Restart agent after replacing the executable. Startup fails if a non-socket occupies the socket path.
Only the trusted Drive container/host group should reach this socket; access to it confers the agent's host file capabilities.
Agent requests are role-checked by Drive. Mutations require session CSRF and same origin, and a successful pre-operation audit write.
Drive records requests in drive_audit; host outcomes go to journald. Do not confuse request audit records with confirmed success.

## Storage and recovery

Private data: /data/users/{immutable-user-id}/files; trash: sibling trash/{id}/meta.json + content.
API paths cannot address sibling users, internal trash metadata or arbitrary host paths.
Rooted handles + openat/O_NOFOLLOW are used per component; renameat2(RENAME_NOREPLACE) prevents accidental overwriting.
Upload writes to a temporary file then fsync/finalize. Failed uploads remove their temp file on request cleanup.
Trash writes a manifest before moving content; incomplete manifests are hidden from the UI.
Full power-loss recovery/reconciliation and orphan cleanup are still pending. Never call this transactional DB/filesystem storage.
Operations serialize per user, and host operations serialize through the agent; long downloads/uploads can delay mutations in that namespace.

## Tests

```sh
go test ./...
go vet ./...
npm ci
npm run build --workspace=minhanos-files
# From repository root on test deployment only:
TEST_PASSWORD_FILE=secrets/bootstrap_password node tests/drive-smoke.mjs
sh tests/drive-cleanup.sh
```

Smoke creates two exact disposable users (recorded in artifacts/drive-test-users.json) and a temporary /srv/minhanos-drive-smoke-* directory.
Cleanup removes only those user IDs/private namespaces, never real accounts. Failed tests need fixture review before rerunning.
tests/drive-independent.mjs checks frontend/login/files/logout against Drive without desktop HTTP calls.
See validation.md for executed checks, including a fresh standalone DB and stopping the desktop service.

## Limits / next work

128 MiB per upload, no quota enforcement yet; 10,000 entries per directory; no recursive search,
recursive folder copy, ZIP, tabs, thumbnails/media/PDF preview, sharing/ACL UI or background durable jobs.
Files are real but transfers are browser-owned: closing/reloading interrupts them. No chunk resume or promise of survival after tab close.
Same-filesystem move only. UTF-8 text editor only; backups remain ordinary adjacent files until managed/moved to trash.
No public trash purge endpoint yet. No ETag-safe overwrite workflow for arbitrary uploads.
HTTP LAN preview only; use HTTPS + secure cookies and review authentication/host capability isolation before wider exposure.

## Native desktop hosting and context actions (0.2)

packages/files-app exports FilesApp and a per-instance client factory. Inputs: embedded, apiBase, active.
Desktop AppContent mounts it with embedded=true and apiBase=/api/drive. apps/drive mounts it with defaults (/api).
CSS is scoped under .files-app; desktop shell CSS is not reused or imported into the application.
Vite dedupes Vue/Lucide peer dependencies; TS path aliases resolve shared source. Both Dockerfiles copy the shared package.

Gateway: platform authenticates desktop cookie/CSRF, rewrites /api/drive/* to /api/* on DRIVE_INTERNAL_URL,
sets X-Drive-Auth-Mode=platform, preserves browser Host/Origin and strips Set-Cookie. Gateway login/logout are blocked.
Drive enables the SQL platform-session adapter only when ALLOW_PLATFORM_SESSIONS=true (integrated Compose only).
The adapter resolves user/role/CSRF from sessions/users in the DB; it does not trust identity headers or prefer a standalone Drive cookie.
Standalone Compose does not enable the adapter or require a platform sessions table.
Platform streaming timeouts accommodate uploads; Drive still enforces size limits. No CORS/token-in-URL or browser bearer secret is added.

Context actions: right-click file/folder/blank/trash, per-file ellipsis, toolbar overflow and shortcuts share actions.ts + dispatch.
Ctrl/Cmd+C/X/V/A, F2, Delete and Shift+F10; menu ArrowUp/Down/Home/End/Escape. Inputs/editors retain their native shortcuts.
Right-click preserves an existing group; Ctrl/Cmd/Shift selects multiple items. Filtering clears selection to avoid acting on hidden results.
Copy/cut clipboard stays in app memory; paste is same-root only, no overwrite, up to 100 items per batch. Regular-file copy max 128 MiB.
Batch operations are sequential browser requests, not durable jobs/atomic batches; errors are reported per item and successful cuts are removed from clipboard.
Folder recursive copy is deliberately disabled until durable jobs exist. No archive/sharing/permissions commands are faked.
