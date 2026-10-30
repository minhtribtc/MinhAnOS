# File Station (MinhAnOS Files) — compliance review (2026-10-31)

Checked against AGENTS.md (app structure, central Settings, i18n/theme, SDK, error codes, identity, audit).

| Area | Status | Evidence |
|---|---|---|
| Structure: UI package + standalone host, no Desktop imports | ✅ | `packages/files-app`, `apps/drive`; boundary test passes |
| Authorization (Phase 5) | ✅ | every `/api/roots`, `/api/files/*` through `IdentityStore.Authorize` |
| Soft quota (Q1) | ✅ | reserve/commit on upload, copy, save-text; quota bar |
| Plain-HTTP safety | ✅ | insecure-context test |
| Close guard (`canClose`) | ✅ | exposed and used by Desktop |
| Manifest | ⚠️ | `nameKey`, `settingsIntegration: []` ok; `entrypoint /apps/files` is not a real URL; no `navigation`, no `lifecycle` |
| Central Settings | ⚠️ | own language/theme selectors hidden inside Desktop (ok); no panel for Files preferences (default view, hidden files) — fine while none are persisted |
| i18n | ❌ | 23 keyed strings vs 37 Vietnamese text nodes + 21 Vietnamese attributes in the template + 19 in script (sidebar, trash, login, dialogs) |
| Theme tokens | ❌ | 100 hard-coded hex colours vs 38 token uses in `style.css` |
| SDK catalog | ❌ | `host.i18n.registerCatalog('files')` instead of reference-counted `host.useCatalog` (throws if Files is mounted twice, e.g. app + a future Settings panel) |
| SDK host events | ❌ | expired Desktop session inside Files shows Files' own login instead of `host.emit('session-ended')` |
| Stable error codes | ❌ | 59 Drive errors carry only Vietnamese text (23 `fail` + 36 `problem` without code); the UI maps unknown codes to a generic "request failed", so users lose the real reason and nothing is translatable |
| Audit | ❌ | `drive_audit` (username + free text, no request/correlation ID, not append-only) instead of the Phase A structured store |
| Naming (ADR 0003) | ❌ | UI shows "MinhAnOS Drive", "File Station · Preview 0.1", "Drive độc lập"; product name is **MinhAnOS Files** |
| Storage model | ❌ | homes fixed at `DataDir/users/{id}`; cannot open Shared Folders; contradicts the first-run rule below |
| Tests | ⚠️ | Go engine/authorization/quota tests and node action/client tests exist; no component test of the Vue app |

Verdict: secure core (authorization, quota, path safety) is compliant; the app layer is not yet. Priority fixes:
storage model + share roots (needed for the first-run rule), stable error codes + i18n, `useCatalog` and
`session-ended`, structured audit, naming, theme tokens, manifest entrypoint/navigation/lifecycle.

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
