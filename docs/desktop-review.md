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
| 9 | Authorization | ✅ | identity policy for app visibility; legacy `/api/admin/users` and its role middleware removed (404, checked) |
| 10 | Audit | ✅ | legacy `INSERT INTO audit` for sign-in/out removed; identity events are the only record (table kept for history) |
| 11 | Notifications | ✅ | bell + unread count + Notification Center |
| 12 | Logs | ✅ | `slog` JSON with request ID; private "Nhật ký" app and `GET /api/audit` removed; history in Log Center |
| 13 | Quota | n/a | preferences/notes only |
| 14 | Tests | ✅ | Go handler tests (`TestKnownAppsMigratesLegacyLayout`), api-smoke, app-standard check |
| 15 | API contract | ✅ | all 9 Desktop routes in OpenAPI (`contracts/desktop`, generated client); preferences ETag/If-Match (428/412) |
| 16 | Security | ✅ | CSRF/origin on writes; login screen no longer shows the server IP |
| 17 | Privacy | ✅ | `GET /api/export` (layout, pinned, notes as JSON); preferences and legacy audit rows of deleted accounts removed hourly |
| 19 | Backup | ✅ | `contracts/data/desktop.data.json` lists tables, volume, config; `tests/backup-restore.sh` restores in isolation |
| 21 | UX states | ✅ | Control Center no longer shows fake integrations; `/api/system` returns real values only |
| 22 | Accessibility | ✅ | AA token contrast (light/dark); axe-core WCAG 2.1 AA, search dialog focus trap + Escape, 375 px without overflow (`tests/a11y-browser.mjs`) |
| 23 | Naming | ✅ | "MinhAnOS Desktop"; preview version removed |
| 24 | Runtime | ✅ | docker-context check |
| 26 | Documentation | ✅ | this review, user guide, HANDOFF, CHANGELOG, validation |

Visual note: the token mapping approximates the old palette (largest colour difference about ΔE 29 on a few
gradients); light/dark come from the token set now.

Remaining work: retire legacy `/api/admin/users` (row 9), OpenAPI for preferences/system (row 15), backup
list and notes export (rows 17, 19).
