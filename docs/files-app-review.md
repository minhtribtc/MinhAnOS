# MinhAnOS Files — compliance review (2026-10-31, updated after the standard pass)

Checked against all 26 rows of the [app standard checklist](app-standard.md). The first review (rows 1–14)
found the app layer non-compliant; the standard pass of 2026-10-31 fixed it. `tests/app-standard.mjs`
(part of `npm run check`) guards the static rules.

| # | Area | Status | Evidence |
|---|---|---|---|
| 1 | Structure | ✅ | `packages/files-app` (no Shell imports, checked) hosted by Desktop and by `apps/drive` |
| 2 | Manifest | ✅ | `files.app.json` 0.3.0: `navigation` (personal, shared, trash, system), `lifecycle` (foreground, background, requestClose), `settingsIntegration: []` |
| 3 | Registry | ✅ | launcher name from `files.app.name` in the Files catalog; backend registry carries the ID only |
| 4 | Settings | ✅ | no own language/theme selectors; no Files preferences persisted yet, so no panel |
| 5 | i18n | ✅ | ~195 `files.*` keys, vi-VN = en-US, no Vietnamese literals in code; dates through `host.i18n.formatDate` |
| 6 | Theme | ✅ | 0 hex colours in `style.css`; only `@minhanos/ui` tokens (`color-mix` for alpha) |
| 7 | SDK | ✅ | `host.useCatalog`; `session-ended` on `AUTH_REQUIRED` when embedded; `canClose` |
| 8 | Errors | ✅ | every Drive `fail`/`problem` carries a code (FILES_NOT_FOUND, FILES_EXISTS, STORAGE_FULL, CSRF_INVALID, …); UI maps `files.errors.<CODE>` then `common.errors` |
| 9 | Authorization | ✅ | every `/api/roots`, `/api/files/*` through `IdentityStore.Authorize` + share permissions |
| 10 | Audit | ✅ | `files_events` (migration module `files` step 1): actor public ID, action `files.<op>`, root, share key, request/correlation ID; append-only trigger; paths stored only for the system root; legacy `drive_audit` no longer written (kept for history) |
| 11 | Notifications | ❌ | quota thresholds 80/95/100 % and failed uploads are not queued yet |
| 12 | Logs | ✅ | `slog` JSON with request/correlation ID, no usernames or paths; Files audit is the Log Center source `files` (365 days) |
| 13 | Quota | ✅ | reserve → commit/release on upload, copy, save-text; share and quota-folder limits |
| 14 | Tests | ✅ | Go engine/authorization/quota/audit (`TestFilesAuditPrivacyAndAppendOnly`), node action/client tests, app-standard check |
| 15 | API contract | ❌ | `/api/files/*` not in OpenAPI yet |
| 16 | Security | ✅ | same-origin + CSRF on writes, host work only through the restricted agent |
| 17 | Privacy | ⚠️ | per-user homes, no paths of personal files in audit; no export/removal flow |
| 18 | Migrations | ✅ | `files_events` through the runner; backup `before-files-events.sql` before deploy |
| 19 | Backup | ❌ | no per-app backup list; restore untested |
| 20 | Long-running work | ❌ | jobs contract planned (recursive copy, moving homes) |
| 21 | UX states | ✅ | loading, empty, no-storage, errors, confirmations for destructive actions |
| 22 | Accessibility | ⚠️ | dialogs `aria-modal` + `aria-labelledby` (checked); no automated focus/contrast tests |
| 23 | Naming | ✅ | "MinhAnOS Files" in header and page title; "Drive"/"File Station · Preview" removed (checked) |
| 24 | Runtime | ✅ | docker-context check; non-root image |
| 25 | Limits | ✅ | text and upload limits, bounded listings |
| 26 | Documentation | ✅ | this review, user guide, HANDOFF, CHANGELOG, validation |

Remaining work: quota notices (row 11), OpenAPI for `/api/files/*` (row 15), jobs (row 20), backup list and
export (rows 17, 19).

## First-run rule (user requirement 2026-10-31)

On a brand-new system the only account is the System Admin and **no home area exists**. After the first sign-in
the admin must create a Shared Folder that hosts homes; only then do the admin and later users get a home.

Consequences for the Q1 design (to change in Q2):
1. Migration `storage` 001 registers the old Files data directory as volume `files` + share `homes`. Keep that
   only for upgrades that really have data: Drive decides at startup with filesystem knowledge — if
   `DataDir/users` holds no user data, the legacy registration is removed (fresh install); otherwise it stays as
   a **legacy homes** share until the admin moves homes to a new share.
2. A share gets "hosts user homes" when created. Homes live inside it (`<share>/<user storage ID>`), created
   lazily through the storage agent as their own dataset/subvolume (ready for optional hard quota); a plain
   directory driver covers ext4/XFS volumes (soft quota only, no snapshots).
3. Drive resolves each home from the registry instead of `DataDir/users`; managed volume roots are
   bind-mounted into the Drive container (`rslave` for ZFS).
4. First-run UX: after the admin's first sign-in Desktop shows "Set up storage" → Storage Manager wizard
   (volume, name, quota, "hosts user homes"). Files without a home shows "No storage yet" (admins get a button
   to Storage Manager). Creating users stays possible; their home becomes ready once a homes share exists.
5. Volumes themselves (zpool/mkfs) still come from installation/agent configuration; disk/pool creation in
   the UI is a separate, later feature.
