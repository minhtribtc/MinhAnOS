# MinhAnOS app standard — review checklist

Every app (Desktop shell included) is reviewed against these rows. Rows 15–26 were added on 2026-10-31
after a gap review; earlier app reviews covered rows 1–14 only. Sources: AGENTS.md, ADR 0007 (Settings),
[notifications/logs review](notifications-logs-review.md), [Phase A](account-center/PHASE_A_REPORT.md).

| # | Area | Requirement |
|---|---|---|
| 1 | Structure | UI package `packages/<name>-app` without Shell imports; standalone host when the app has one; peer app, not code inside another app |
| 2 | Manifest | App ID from the catalog, real `entrypoint`, `nameKey`, `navigation`, `lifecycle`, `settingsIntegration` (or `[]`) |
| 3 | Registry | Shell lists the app from the manifest/catalog, filtered by the identity policy — not hard-coded tabs |
| 4 | Settings | Configuration only as panels in central Settings; no own selectors for Platform-owned settings (locale, time zone, theme) inside a Shell |
| 5 | i18n | Every UI string is a structured key `<app>.*`, vi-VN and en-US parity, ICU parameters |
| 6 | Theme | Colours/spacing from `@minhanos/ui` tokens; light/dark from the token set, no parallel hex palettes |
| 7 | SDK | `host.useCatalog`, host events (`session-ended`, `profile-changed`), `openSettings`, `canClose`, plain-HTTP safe APIs |
| 8 | Errors | Stable backend error codes; UI never shows or parses server prose |
| 9 | Authorization | Every request through the identity policy (`CheckApp`): entitlement AND app grant AND deny rules; resource rules after |
| 10 | Audit | Security/admin actions written to the structured, append-only store with request/correlation IDs (identity events, storage events); no new free-text audit tables |
| 11 | Notifications | User-relevant events (security, quota thresholds, failures) queued in a transactional outbox with severity and `mandatory`; shown through the Notification Center/bell, never only as transient toasts |
| 12 | Logs | Operational errors logged structured with request ID; user-visible history comes from the Log Center sources, not a private app log viewer |
| 13 | Quota/storage | Writes that add data go through the soft-quota ledger (reserve → commit/release) |
| 14 | Tests | Backend contract/integration tests for every rule above that the app touches; frontend build + boundary/standard checks |
| 15 | API contract | Every HTTP API in versioned OpenAPI/JSON Schema with generated types/client, stable error codes, cursor pagination with limits, `If-Match` versions and `Idempotency-Key` for writes; no undocumented endpoints |
| 16 | Security baseline | Same-origin + CSRF on writes, strict input validation (unknown fields rejected), reauthentication for sensitive admin writes, rate limits where abuse is possible, CSP-compatible UI, no secrets in URLs, logs, events or DTOs; host/root work only through restricted agents |
| 17 | Privacy & data ownership | Per-user isolation by default; admins do not read personal data implicitly; documented retention for every table the app owns; user data export/removal path; PII minimised in audit/notifications |
| 18 | Data & migrations | Schema only through the migration runner as an owned module; applied steps immutable; expand/contract for breaking changes; backup before applying on real data; DB/filesystem consistency rules written down |
| 19 | Backup & restore | The app lists what must be backed up (DB tables, files, config) and restore is tested in isolation; no state only in browser storage |
| 20 | Long-running work | Operations that can exceed a request (moves, scans, imports) run as durable jobs with progress, cancel, retry and audit (jobs contract) |
| 21 | UX states & honesty | Loading, empty, error, offline/degraded (e.g. agent down) and first-run states designed; planned features never shown as working; destructive actions confirmed |
| 22 | Accessibility & responsive | Keyboard reachable, visible focus, labelled controls and dialogs (focus trap, Escape), sufficient contrast in light/dark; usable at phone width without horizontal overflow |
| 23 | Naming & branding | Display names and IDs from ADR 0003 (e.g. "MinhAnOS Files"); no legacy or preview names in UI text |
| 24 | Runtime & packaging | Non-root, read-only rootfs, dropped capabilities, health check; image build copies every package it needs (docker-context check); pinned dependencies and image digests |
| 25 | Performance & limits | Bounded queries and payloads, pagination, request timeouts, upload/text limits declared; no unbounded scans on request paths; bundle growth watched |
| 26 | Documentation | Same commit updates user guide, HANDOFF, CHANGELOG, API/architecture docs and validation evidence (AGENTS mandatory documentation) |

Notification Center and Log Center exist as minimal v1 (2026-10-31, [design](notification-log-center.md)):
an app satisfies rows 11–12 by writing its outbox/audit correctly and registering it with
`notifycore.Deliver` / `logcore.Sources`. Today only identity (notices + audit) and storage (audit) are wired.

## Known violations of rows 15–26 (2026-10-31)

| Row | Where | Gap |
|---|---|---|
| 15 | Drive `/api/files/*` | Not in OpenAPI (storage API fixed 2026-10-31: `/api/v1/storage` in OpenAPI with generated client, If-Match, Idempotency-Key) |
| 20 | Storage, Files | Jobs contract still planned; needed for measuring, moving homes to a new share, recursive copy |
| 19 | All apps | No per-app backup list; backups so far were manual `pg_dump` before migrations; Files/ZFS/Btrfs data restore untested |
| 17 | All apps | No user data export/removal flow; retention documented only for identity audit and notices |
| 22 | Desktop, Files, Storage Manager | No accessibility checks in the test suite (Account Center dialogs have a focus trap) |
| 23 | Files | "MinhAnOS Drive", "File Station · Preview" in UI |
| 11–12 | Files, Storage | Files sends no notices (quota thresholds) and its `drive_audit` is not a Log Center source; storage audit has no retention policy |
| 12 | Desktop | Legacy "Nhật ký" app still reads the legacy `audit` table; to be replaced by a per-user activity view |
