# ADR 0004 — Shared AuthCore and local recovery

Accepted 2026-10-30, account-center-phase2. Người dùng cho phép chọn phương án phù hợp định hướng và ghi đầy đủ quyết định.

## Decision

Use internal/identity.AuthCore as a shared Go module in Desktop and standalone Drive. Keep users.password_hash, legacy storage IDs, PBKDF2-SHA256 600000 and the two canonical tables sessions/drive_sessions. Public realm names are desktop/files; the Go DriveRealm constant maps files to drive_sessions. No IAM daemon, third session store, Redis, JWT or OIDC provider.

Migration identity 004 adds session UUID handles/epoch/nullable creation metadata, recovery metadata on users, bounded hashed-key auth attempts and sanitized auth-event rows. Both canonical realm tables exist even on standalone installations so installing the second host later does not bypass migration history. Existing session creation times remain NULL; do not invent historical timestamps. New sessions get server timestamps. Auth method/device/last-seen enrichment is deferred; OwnSessions is an internal bounded projection, not the complete public Session DTO/paginated API.

Authentication checks active status and equality of session/user auth_epoch on every adapter. Credential verification happens outside user-row locks; issuance locks the user then rechecks the exact hash, epoch and status. Mutations use the same users-before-session lock order. Password/status triggers increase epoch for legacy DB writers; decreasing epoch is rejected. A password change increments epoch, revokes both tables, clears recovery and appends an event in the same transaction. RevokeAll also invalidates current session; in-flight work still needs resource authorization before commit in later Files work.

Keep old HTTP cookie/CSRF/Origin and response shapes. Drive login rotates a supplied old Drive cookie, matching Desktop. New /api/v1/identity endpoints remain off until business authorization is ready. Legacy login/logout audit entries remain compatible but are separate adapter operations; credential/recovery/revoke mutation events are transactional. This is not a full administrative audit reader/retention solution.

## Abuse controls

Two concurrent authentication KDF operations per AuthCore instance; fail fast with AUTH_BUSY/503 when full. PostgreSQL reserves per-normalized-account attempts before hashing, shared across binaries/realms: 20 attempts per fixed ten-minute window, successful login clears the account counter. Store SHA256(normalized name), not raw names; hashes are pseudonyms, not irreversible anonymization. Table cap 10000; bounded expired-row cleanup of 100 per call. Table-full rejects new keys; existing IP controls remain 20/ten minutes and each map is capped at 4096 peers. Use real RemoteAddr only, not untrusted forwarded headers. No permanent account disable from failed login.

Fixed-window account throttling can temporarily deny a targeted account. Distributed flood/edge mitigation and proxy trust policy remain production hardening tasks. Same-origin trusted applications are not isolated from one another by appId alone.

## Recovery

Only the Linux OS-root identity-recovery CLI invokes IssueRecoveryLocal/ConsumeRecoveryLocal. No HTTP reset/email endpoint. Token: standard existing 256-bit cryptographic random primitive, only SHA256 stored, 15-minute database TTL, bound to auth_epoch, one outstanding token per user. Issuing another token supersedes the previous one without changing password/sessions. Only active existing users; never enable/promote/provision automatically.

Consume requires root-owned regular private files, rejects symlink final components and oversized input, never accepts a password via command arguments or prints secrets. Database locking and conditional consume make concurrent use single-winner. Password change/status transition/revoke-all invalidates an older recovery token. Audit failure rolls back credential mutation and revocation. Operator holds root/DBA trust; the Go local methods are not an authorization boundary for hostile in-process code.

## Compatibility and limits

Identity migrations 001–004 are immutable after deployment. Migration 004 assigns UUIDs to existing session rows (possible table rewrite), not an unbounded user backfill. Tested on this small VM with short-lived session tables; large existing installations need session counts, maintenance/lock planning and restore rehearsal. No claim of zero-downtime large-database upgrade. Never revert to binaries that insert sessions with default epoch 1 after credentials have changed: they cannot honor new state correctly. Restore is version/data coordinated, not an automatic down migration.

No changes to Files storage paths, host mounts, legacy role grants or host-root authority. Last-admin protection and host privilege split remain gates before administrative APIs. No UI, MFA, email delivery, SSO or POSIX provisioning enabled here.

## References

Consulted official [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) and [OWASP Forgot Password](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html): rotate session identifiers, invalidate sessions on credential changes, and use expiring single-use random reset tokens stored securely. This implementation has not undergone an independent security audit.
