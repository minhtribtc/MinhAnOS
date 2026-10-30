# Notification Center and Log Center — minimal v1 (2026-10-31)

Two peer apps that close app standard rows 11 (notifications) and 12 (user-visible history). Deliberately small:
they show what the modules already write, so every existing and future app has one place to send notices and
one place where its audit trail is read. Later phases add more sources and features, not a new design.

## Notification Center (`com.minhanos.notifications`)

| Part | v1 |
|---|---|
| Producers | Backend modules only, through their transactional outbox (today: `identity_notification_outbox`, security notices). No browser publish API, no `notifications.publish` |
| Delivery | Desktop host worker every 10 s: moves a batch of 100 outbox rows into `notifications` in one transaction; inbox id = outbox id, so redelivery never duplicates |
| Inbox | `notifications` (module `notifications`, SQL 001): recipient public UUID, source app ID, kind, severity, mandatory, params, request/correlation ID, created/read time. Retention 90 days (worker purges in batches) |
| Texts | Never stored. The UI renders `notifications.kind.<kind>.title/body` with params in the user's language; unknown kinds show a generic notice |
| API | `GET /api/v1/notifications/inbox?limit≤50&cursor&unread=true`, `POST /api/v1/notifications/inbox/read` with `{ids}` or `{upTo}` (idempotent by itself; Idempotency-Key and CSRF required) |
| Permissions | `notifications.own.read`, `notifications.own.manage` (identity SQL 011, every account through standard-user). Users see only their own notices |
| UI | Bell with unread badge in the Desktop status bar (polled every 60 s while visible), panel with the 8 latest notices, "Mark all as read", "Open Notification Center"; full app with All/Unread filter and paging |
| Settings | `settingsIntegration: []` — no preferences yet. Mandatory notices can never be muted |

## Log Center (`com.minhanos.logs`)

| Part | v1 |
|---|---|
| Model | Read-only view; owns no data. Each source keeps its table, write path and retention |
| Sources | `identity` (`identity_admin_events`, 365 days) and `storage` (`storage_events`, no retention policy yet), listed by `GET /api/v1/logs/sources` |
| Not sources | Files `drive_audit` (personal file paths: admins must not read them implicitly, row 17) and the legacy `audit` table (duplicates identity sign-in events) |
| API | `GET /api/v1/logs/events?source&outcome&limit≤100&cursor`: newest first by (time, id); each source is limited before the union, so a page never scans more than two pages of rows |
| Permissions | `logs.audit.read` (System Admin, identity SQL 011) |
| UI | Table (time, source, action code, actor, target, outcome) with source/outcome filters, expandable row (request/correlation ID, details), paging, retention line |

## How an app plugs in (standard rows 11–12)

- **Notify:** write a row into your module's outbox in the same transaction as the change (kind, severity,
  mandatory, flat params, request/correlation ID); add a consumer branch in `notifycore.Deliver` and
  `notifications.kind.<kind>.*` texts in vi-VN and en-US.
- **Audit:** keep an append-only, structured table with actor UUID, action code, target, outcome,
  request/correlation ID; add it to `logcore.Sources` and the union in `logcore.Events`.

## Not in v1 (later)

Notification preferences (non-mandatory only) as a Settings panel; SSE instead of polling; more producers
(Files quota thresholds, failed jobs); per-user activity view replacing the Desktop "Nhật ký" app;
Files/legacy audit through privacy-safe adapters; application and system logs (slog JSON, journald via agent);
export and retention management (`logs.export`, `logs.retention.manage`); storage audit retention.
