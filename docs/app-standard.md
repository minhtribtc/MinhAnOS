# MinhAnOS app standard — review checklist

Every app (Desktop shell included) is reviewed against these rows. Sources: AGENTS.md, ADR 0007 (Settings),
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

Notification Center and Log Center are still **planned**: rows 11–12 are satisfied today by writing the
outbox/audit correctly; showing them needs those apps (Phase B/C).
