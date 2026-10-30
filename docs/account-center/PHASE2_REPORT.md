# Account Center — Phase 2 report

2026-10-30, branch account-center-phase2. User delegated routine architecture choices; accepted decisions are in [ADR 0004](../adr/0004-shared-auth-core-and-recovery.md).

## Implemented

Shared AuthCore handles Desktop login/session/logout, standalone Drive and the native Files platform-session adapter. No duplicate credentials or new session realm. It enforces active account state/auth_epoch, rotates login cookies, exposes internal own-session list/revoke/revoke-all and reauthenticated password-change primitives. Shared vi/en error catalogs cover AUTH_RATE_LIMITED/AUTH_BUSY. Public identity endpoints/UI remain planned. Password change and recovery force login again on both hosts.

Migration 004 adds session handles/epoch, local recovery references, bounded account throttles and auth mutation events. Legacy password/status writers trigger epoch advancement; revocation cannot be undone by enabling a previously disabled account. No admin status mutation endpoints are exposed yet because last-admin/policy/host privilege split are Phase 3 gates.

Linux OS-root CLI identity-recovery issues/consumes a 15-minute, hashed, single-use token through private files. It does not promote users or enable disabled accounts. Credentials remain solely in users.password_hash. Local recovery runbook is in [user guide](../user-guide.md).

## Security and migration review

See ADR for user-lock ordering, post-KDF race checks, transaction rollback on audit failure, account/IP caps and overload behavior. New password policy matches existing minimum 12 and maximum 256 UTF-8 bytes. Existing PBKDF2 hash format and verification are unchanged. HTTP adapters preserve CSRF/Origin, HttpOnly/SameSite cookies and existing legacy JSON; AUTH_RATE_LIMITED and AUTH_BUSY are stable codes. No public reset token or new unauthenticated account endpoint.

Applied identity 004 on test VM after private backup /opt/minhanos-backups/before-identity-phase2.sql and .tgz (0600). Restored real pre-upgrade DB in an isolated PostgreSQL container and ran profile/Identity upgrade tests before deployment. VM schema now identity 1–4 + profile 1. Freeze SQL 001–004. Session UUID population can rewrite session tables; current small VM test does not establish large-database upgrade performance. No Files rekey/chown, new grant seeds or production administrator password change. Full Files-volume recovery remains untested.

## Actual verification

- go test ./... and go vet ./...: PASS; Docker builds of Desktop/Drive include both checks. Root npm run check: PASS, one pre-existing reserved IdentityLink warning.
- Dedicated PostgreSQL integration: PASS adoption/legacy invariants, bounded backfill, account state, legacy hash verification, realm separation, login rotation, session ownership, own revoke/revoke-all, password changes, recovery expiration/supersession/single-use/concurrent consume, login racing epoch revocation, audit-failure rollback, account throttle sharing/expiry/cap and KDF concurrency cap.
- HTTP adapter test: PASS fresh bootstrap plus both cookie realms, native Files gateway, cross-origin/CSRF, fixation protection and both-realm revocation. Added to normal dedicated DB CI script.
- CLI file tests: PASS private-file checks including public permissions, symlink, directory and oversize rejection.
- Restored real pre-upgrade DB: PASS. Existing Desktop and Drive cookies captured privately before deploy remained valid after migration; gateway worked; exact sessions logged out and private fixture file removed.
- On VM: profile smoke, embedded Files smoke, Drive smoke (including real Debian agent fixture) and API smoke: PASS. Removed exactly six disposable users/private namespaces, remaining users=1. Drive works with Desktop stopped; Desktop restarted healthy.
- Fresh standalone project minhanos-authcore-fresh on loopback 8083: PASS bootstrap/login, real root recovery CLI issue/consume, old session/password denied, new password accepted, replay denied and non-root CLI denied. Exact project/volumes/private fixtures removed.

One deployment-test command initially tried docker cp into a read-only container and was rejected. Used root exec/stdin into existing /tmp tmpfs with umask 077 instead; kept read-only rootfs. A read-only diagnostic SQL command had a shell-quoting syntax error; corrected without mutation.

## Small comparative benchmark

Same Debian VM, 12 vCPU exposed AMD Ryzen 5 5600X, 7.765 GiB RAM; Go1.26, PostgreSQL18, Node22 client, same admin account and PBKDF2 cost. Eight sequential login/logout samples per host, existing warmed services, network LAN/host test, no capacity claim:

| Host | Phase 1 p50/p95 ms | Phase 2 p50/p95 ms |
|---|---|---|
| Desktop | 82 / 133 | 83 / 137 |
| Drive | 81 / 89 | 81 / 86 |

Docker reported memory after each sample: Phase 1 Desktop 14.17 MiB/Drive 13.63 MiB; Phase 2 7.223 MiB/5 MiB. Different process ages/GC mean these are observations, not a memory improvement result or RSS-isolated Identity measurement. No cold/warm startup timings, production load/p95 certification or full resource budget established.

## Remaining work

Phase 3 business authorization, protected roles/last-admin invariant, host-root privilege split, v1 HTTP controllers, optimistic concurrency/idempotency and administrative audit. Phase 4 shared Account Center UI/standalone host. Session metadata enrichment/pagination, recovery retention/cleanup policy, edge flood controls, HTTPS and long-transfer commit-time reauthorization need subsequent work. No MFA/email reset/OIDC/POSIX provisioning enabled.

## File inventory

- `AGENTS.md`
- `CHANGELOG.md`
- `Dockerfile`
- `Drive.Dockerfile`
- `README.md`
- `cmd/identity-recovery/main.go`
- `cmd/identity-recovery/main_test.go`
- `contracts/README.md`
- `contracts/identity/README.md`
- `contracts/identity/status.v1.json`
- `docs/HANDOFF.md`
- `docs/README.md`
- `docs/account-center/IMPLEMENTATION_PLAN.md`
- `docs/account-center/PHASE2_REPORT.md`
- `docs/account-center/README.md`
- `docs/adr/0004-shared-auth-core-and-recovery.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/roadmap.md`
- `docs/user-guide.md`
- `docs/validation.md`
- `internal/drive/auth.go`
- `internal/drive/server.go`
- `internal/identity/auth.go`
- `internal/identity/auth_test.go`
- `internal/identity/migrations/004_auth_core.sql`
- `internal/identity/schema.go`
- `internal/platform/auth_integration_test.go`
- `internal/platform/server.go`
- `packages/app-sdk/locales/common.json`
- `tests/identity-login-benchmark.mjs`
- `tests/identity-recovery-fresh.mjs`
- `tests/identity-upgrade-session.mjs`
- `tests/migrations-integration.sh`
