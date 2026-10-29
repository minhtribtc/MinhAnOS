# MinhAnOS Drive 0.1 / File Station

Drive is an independently runnable application, not a desktop component or iframe.
Test URL: http://10.1.1.20:8082. Desktop on 8081 launches its URL in a new tab, with same-tab navigation when the browser blocks popups.
Branch: file-station-v1. Source: apps/drive, cmd/drive, internal/drive, internal/authn.

## Independence contract

- Frontend has its own package, lockfile, build, login and client API; no imports from apps/desktop or desktop Pinia.
- Backend does not import internal/platform or call desktop/platform HTTP APIs.
- Drive.Dockerfile builds without copying desktop frontend or platform backend sources.
- compose.drive.yaml runs Drive + PostgreSQL without a desktop service. An empty standalone DB bootstraps its own admin from the secret file.
- Current integrated deployment shares the users table/password format with MinhAnOS, not its browser session.
  Drive uses its own drive_sessions table and minhanos_drive_session cookie. Logging out of one app does not log out the other.
- IdentityStore is an adapter boundary for future OIDC or a separate identity database. Shared authn contains password/token primitives only.
- Drive does not require parent window messaging, a desktop token, shell state, localStorage credentials or desktop routes.
- Standalone account management is currently bootstrap-admin only; provisioning more users needs an identity/admin interface in a later release.

## Delivered

Private filesystem namespace per user; grid/list, hidden files, folder navigation/back/up,
name filtering in the current folder, metadata/UID/GID/mode, create folder, raw upload/download,
rename, same-filesystem move without overwriting, trash and restore. Filename collisions return 409.
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
Desktop defaults to the same hostname on port 8082. A changed published port needs a matching launcher build configuration.

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
cd apps/drive && npm ci && npm run build
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
multi-select, copy, ZIP, tabs, thumbnails/media/PDF preview, sharing/ACL UI or background durable jobs.
Files are real but transfers are browser-owned: closing/reloading interrupts them. No chunk resume or promise of survival after tab close.
Same-filesystem move only. UTF-8 text editor only; backups remain ordinary adjacent files until managed/moved to trash.
No public trash purge endpoint yet. No ETag-safe overwrite workflow for arbitrary uploads.
HTTP LAN preview only; use HTTPS + secure cookies and review authentication/host capability isolation before wider exposure.
