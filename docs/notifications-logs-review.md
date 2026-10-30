# Review — Codex Notification Center & Log Center brief (2026-10-30)

Review only; nothing implemented. Notification Center and Log Center are **planned**.

## Current facts (checked in source)

| Area | What exists |
|---|---|
| Audit stores | Three separate tables: `audit` (platform: `auth.login`, `auth.logout`, `user.create`; username + free-text detail), `drive_audit` (Drive actions, pre-operation audit), `identity_admin_events` (Identity v1: UUID actor, action, target, outcome, request_id, details; written in the policy transaction) |
| Failure/denied audit | Identity v1 denials written best-effort outside the transaction (`_, _ =` in read.go). Failed logins are throttled but not audited |
| Retention | None for any audit table |
| Event contract | `contracts/events/events.v1.yaml` + `event.v1.schema.json`, status **planned**: envelope `eventId, name, schemaVersion, occurredAt, source, subject, correlationId, data`; transactional outbox, at-least-once, dedupe by eventId+consumer |
| Request IDs | `X-Request-ID` only on `/api/v1/identity`; no correlation ID elsewhere |
| App logging | Go stdlib `log`, unstructured; containers log through Docker's logging driver, not journald |
| Notifications | None; Desktop has only transient toasts |
| Existing log UIs | Desktop "activity" app reads `/api/audit`; Account Center has an Audit tab reading `identity_admin_events` |
| Permission naming | Capability pattern `^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$` (no underscores); `identity.audit.read` already exists |

## Verdict

Sound brief; adopt with corrections. Agree with: separating domain event / audit record / notification /
operational log; transactional outbox instead of an event bus; no Elasticsearch/Kafka/Loki; modules inside the
existing Go backends; restricted agent for journald; not every audit becomes a notification; mandatory security
alerts cannot be muted; secrets never in events/logs; Phase A (audit for Account Center) first; survey before code.

## Required corrections

1. **Settings rule is outdated.** The brief says "Settings only manages Desktop and system". The user chose the
   hybrid model (ADR 0007): Notification Center owns `notifications.*` preferences and also shows that panel in
   Settings; Log Center retention policy is its own panel.
2. **Extend the existing event contract, do not create a second one.** Keep the planned envelope field names
   (`name`, `source`, `subject`) or migrate them once while still planned. `severity`/`result`/`actor` belong to a
   separate **AuditRecord v1** schema, not to every domain event.
3. **Consolidate three audit stores, do not add a fourth.** Evolve `identity_admin_events` (best structured) into
   one platform audit store; expose legacy `audit` and `drive_audit` through read adapters/views first, no data
   rewrite. Desktop "activity" becomes a legacy alias of Log Center; Account Center's Audit tab becomes an
   identity-filtered view of the same store (no duplicate data).
4. **Failure and denial audit.** Rolled-back business transactions cannot carry the audit, so failures need their
   own short transaction — but must be bounded/aggregated (e.g. failed logins counted per account/IP window),
   otherwise brute force turns into an audit write flood. Replace the current best-effort denial insert with an
   explicit, tested path.
5. **Honest tamper resistance.** Same-PostgreSQL storage cannot stop root/DBA. Realistic v1: application role has
   INSERT/SELECT only on the audit table, retention runs as a separate privileged job, optional hash chain, later
   export to independent/immutable storage.
6. **Who may publish notifications.** There is no verified app principal/delegation yet (GAP P0). In v1 only
   backend modules publish, in process; no browser-facing publish API and no `notifications.publish` for apps.
7. **Admin audiences.** Notifications for "admins" must be resolved by permission at read time (or re-checked),
   so a demoted admin stops seeing them; per-recipient rows only for user-targeted notices.
8. **Permission names must match the pattern:** `notifications.own.read`, `notifications.own.manage`,
   `logs.audit.read`, `logs.application.read`, `logs.system.read`, `logs.export`, `logs.retention.manage`.
   Define how `identity.audit.read` (domain view) relates to `logs.audit.read` (all domains).
9. **Application logs are not in journald today.** Switch to `log/slog` JSON with request/correlation IDs, then
   either set Docker's journald logging driver or read container logs through the agent with a unit allowlist,
   bounded time ranges and redaction.
10. **Realtime.** Start with cheap polling of the unread count; add SSE later. WebSocket not needed.
11. **Retention now.** Define defaults at schema time (example: audit 365 days, notifications 90 days, journald
    size caps) and make them visible in Log Center.
12. **Catalog/naming.** Register `com.minhanos.notifications` and `com.minhanos.logs` in the app catalog
    (ADR 0003) with manifests, `nameKey` and `settingsIntegration`, per the Settings standard.

## Recommended Phase A (for Account Center)

1. Contracts: revise Event envelope v1 (still planned) and add AuditRecord v1 + permission entries.
2. Platform audit store + writer (in-transaction for success, bounded separate path for failure/denial),
   request/correlation ID middleware on both hosts, INSERT/SELECT-only role, retention job.
3. Account Center integration: login success/failure (aggregated), logout/revoke, password change, user/group/
   role changes; legacy tables via read adapters.
4. Durable `notification_outbox` rows for the few security notices (password changed, admin granted/revoked,
   account disabled) — no UI yet.
5. Integration tests: rollback leaves no success audit, failure audit survives, flood is bounded, no secrets.

Phases B (Notification Center) and C (Log Center) as in the brief, after Phase A is accepted.
