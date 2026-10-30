# MinhAnOS Desktop — compliance review (2026-10-31)

Checked against the [app standard checklist](app-standard.md). Desktop is the Shell (launcher, windows, Home,
Control Center) plus six small built-in apps rendered inside `AppContent.vue`.

| # | Area | Status | Evidence |
|---|---|---|---|
| 1 | Structure | ❌ | Overview, App Center, Connections, Activity, Notes and About live inside `AppContent.vue` (8 `v-else-if` branches) instead of peer app packages |
| 2 | Manifest | ⚠️ | `desktop.app.json` has `nameKey` + Appearance panel; `entrypoint /apps/desktop` is not a real URL; built-in apps have no manifests |
| 3 | Registry | ❌ | Go `registry` list + `v-else-if` + `icons.ts` map (hard-coded); only Files and Storage Manager are filtered by policy |
| 4 | Settings | ✅ | central Settings host, Appearance/Language panels, password-change screen; Appearance panel text is still hard-coded |
| 5 | i18n | ❌ | 4 keyed strings vs ~182 Vietnamese literals (App.vue 49, AppContent 86, HomeScreen 14, AppSwitcher 11, AppearancePanel 21) |
| 6 | Theme | ❌ | 557 hex colours in `style.css`/`ipados.css`/`fluent.css` vs 4 token uses; dark mode is a parallel palette |
| 7 | SDK | ✅ | host bridge: `useCatalog`, `session-ended`/`profile-changed`, `openSettings`, close guards |
| 8 | Errors | ⚠️ | legacy `/api` handlers: 19 errors without a code, 11 with; UI often shows `error.message` text |
| 9 | Authorization | ⚠️ | identity policy for Files/Storage visibility and Account Center; legacy `/api/admin/users` and `/api/audit` still use role checks + legacy adapters |
| 10 | Audit | ❌ | sign-in/out written twice: legacy `audit` (username, free text) and `identity_admin_events`; Activity app reads the legacy table |
| 11 | Notifications | ❌ | no bell, panel or unread count; identity security notices are queued in `identity_notification_outbox` but **nothing shows them**; only transient toasts |
| 12 | Logs | ❌ | Activity app is a private log viewer over the legacy table; server logs are unstructured `log.Print` without request IDs (IDs exist in headers/audit only) |
| 13 | Quota/storage | n/a | Desktop stores preferences/notes only |
| 14 | Tests | ⚠️ | build/boundary/standard checks + Go handler tests; Playwright e2e script exists but is not run in checks |

Honest-UI rules (AGENTS) are respected: Connections shows "not connected", overview metrics are labelled as
process/container metrics.

## Recommended order

1. **Notification Center Phase B** (biggest functional gap): bell + unread count in the Desktop status bar,
   panel, Notification Center app reading the identity outbox (mandatory security notices first), later Files
   quota thresholds.
2. **Log Center Phase C** replaces the Activity app; stop writing sign-in/out to the legacy `audit` table once
   the Log Center reads identity events (keep the table for history).
3. Manifest-driven registry; move built-in apps (Overview, App Center, Notes, About) into packages.
4. i18n + tokens migration of the Shell (largest mechanical task; can run in parallel per component).
5. Stable codes for legacy `/api` errors.
