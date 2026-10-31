# MinhAnOS Desktop — compliance review (2026-10-31, updated after the standard pass)

Checked against all 26 rows of the [app standard checklist](app-standard.md). Desktop is the Shell (launcher,
windows, Home, Control Center, Spotlight, bell). `tests/app-standard.mjs` (part of `npm run check`) guards the
static rules.

| # | Area | Status | Evidence |
|---|---|---|---|
| 1 | Structure | ✅ | `AppContent.vue` only hosts app packages (Settings, Account Center, Storage Manager, Notification Center, Log Center, Files, Notes); Notes moved to `packages/notes-app`; Overview, App Center, Connections, Activity and About removed (App Center is `planned` in the catalog) |
| 2 | Manifest | ✅ | `desktop.app.json` 0.3.0 (`navigation: []`, `lifecycle`), new `notes.app.json` (`com.minhanos.notes`) |
| 3 | Registry | ✅ | backend registry = `{id, icon, color}` only; names/descriptions from each manifest + catalog (`src/registry.ts`); unknown or removed IDs dropped from saved layouts (`knownApps`, legacy `notes` → `com.minhanos.notes`) |
| 4 | Settings | ✅ | Control Center opens Settings → Appearance instead of own theme tiles |
| 5 | i18n | ✅ | ~110 `desktop.*` keys, vi-VN = en-US; no Vietnamese literals in code except the "Tiếng Việt" endonym |
| 6 | Theme | ✅ | 0 hex/rgba colours in `style.css`/`ipados.css`/`fluent.css` (725 dead rules pruned); tokens incl. `--mn-color-on-media` and accent scales |
| 7 | SDK | ✅ | `host.useCatalog` for every launcher catalog, `session-ended`/`profile-changed`, `openSettings`, close guards |
| 8 | Errors | ✅ | every legacy `/api` error carries a code (checked); UI shows `host.errorMessage` / `SAVE_FAILED`, never server text |
| 9 | Authorization | ⚠️ | identity policy for app visibility; legacy `/api/admin/users` still served (role check only without the identity service) |
| 10 | Audit | ✅ | legacy `INSERT INTO audit` for sign-in/out removed; identity events are the only record (table kept for history) |
| 11 | Notifications | ✅ | bell + unread count + Notification Center |
| 12 | Logs | ✅ | `slog` JSON with request ID; private "Nhật ký" app and `GET /api/audit` removed; history in Log Center |
| 13 | Quota | n/a | preferences/notes only |
| 14 | Tests | ✅ | Go handler tests (`TestKnownAppsMigratesLegacyLayout`), api-smoke, app-standard check |
| 15 | API contract | ⚠️ | legacy `/api/preferences`, `/api/system` not in OpenAPI |
| 16 | Security | ✅ | CSRF/origin on writes; login screen no longer shows the server IP |
| 17 | Privacy | ⚠️ | notes in preferences per user; no export flow |
| 19 | Backup | ❌ | no per-app backup list |
| 21 | UX states | ✅ | Control Center no longer shows fake integrations; `/api/system` returns real values only |
| 22 | Accessibility | ⚠️ | windows are labelled `region`s, dialogs modal + labelled (checked); no focus/contrast tests |
| 23 | Naming | ✅ | "MinhAnOS Desktop"; preview version removed |
| 24 | Runtime | ✅ | docker-context check |
| 26 | Documentation | ✅ | this review, user guide, HANDOFF, CHANGELOG, validation |

Visual note: the token mapping approximates the old palette (largest colour difference about ΔE 29 on a few
gradients); light/dark come from the token set now.

Remaining work: retire legacy `/api/admin/users` (row 9), OpenAPI for preferences/system (row 15), backup
list and notes export (rows 17, 19).
