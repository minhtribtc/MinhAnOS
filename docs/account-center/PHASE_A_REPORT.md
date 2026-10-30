# Account Center — Phase A (audit/event foundation) report (2026-10-30)

Goal: make Account Center ready for the Notification Center and Log Center (see
[review](../notifications-logs-review.md)). Branch `settings-central-v1`.

## Delivered

- **Request and correlation IDs** — `internal/requestid` middleware wraps both hosts (8081, 8082). Every request gets
  a server-generated `X-Request-ID`; a safe client `X-Correlation-ID` (8–64 of `[A-Za-z0-9._-]`) links several
  requests of one action, otherwise it equals the request ID. Both are stored with every identity event,
  including sign-in/sign-out from the legacy `/api/login` paths. The client cannot choose the request ID.
- **One identity security log** — identity SQL 007 (`security_log`): `identity_admin_events` gains
  `correlation_id`; existing `identity_auth_events` rows are copied once (no duplicates of policy-path rows) and
  that table now rejects inserts. AuthCore primitives (self password change, revoke all, revoke own session,
  local recovery issue/consume) write the shared store; `revoke()` no longer audits, so nothing is recorded twice.
- **Append-only audit** — trigger blocks UPDATE and DELETE on `identity_admin_events`; only the retention routine
  may delete, and only rows older than 365 days. Protects against application bugs, not a database superuser.
- **Retention** — bounded batches (100 rows) run inside every policy mutation: audit 365 days, undelivered
  security notices 90 days.
- **Security notice outbox** — `identity_notification_outbox`, written in the same transaction as the change:
  `password.changed` (warning), `password.reset` (critical), `role.changed` (warning), `sessions.revoked` (info),
  `login.failures` at attempts 10 and 20 (warning), `recovery.consumed` (critical). Mandatory notices are flagged.
  Contract: `contracts/notifications/security-notice.v1.schema.json`.
- **Contracts** — audit DTO exposes `correlationId`; planned permissions `notifications.own.read|manage`,
  `logs.audit.read`, `logs.application.read`, `logs.system.read`, `logs.export`, `logs.retention.manage`.

## Deliberately not changed

- Legacy `audit` (Desktop activity: login/logout/user.create) and `drive_audit` (Files operations) are still
  written for the current Activity app and Drive. The Log Center will read them through adapters and replace
  the Activity app; until then they are compatibility data, not the source of truth for identity events.
- `events.v1` envelope stays planned; the outbox is the only implemented event path.

## Verification

See [validation](../validation.md).
