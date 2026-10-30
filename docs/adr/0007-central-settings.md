# ADR 0007 — Central Settings app and app settings contract

Status: accepted 2026-10-30 (user rule). Thin slice implemented on branch `settings-central-v1`.

## Context

The user requires every MinhAnOS app to integrate uniformly with one central Settings app, like Settings on
iOS/macOS/Android. Before this ADR Settings tabs were hardcoded in `apps/desktop/.../AppContent.vue`, and
"My account"/"Users" duplicated Account Center. Manifest `settingsIntegration` existed but nothing read it.

## Decision

- `@minhanos/settings-app` (App ID `com.minhanos.settings`) is the only Settings UI. Its sidebar is built from
  manifest `settingsIntegration` entries; groups are named by the new optional manifest field `nameKey`.
- Each app package exports a provider via `defineSettingsProvider({manifest, panels})` from app-sdk. Panels are
  Vue components with prop `active`, optionally exposing `canClose()`. Rendered natively, no iframe.
- The Shell registers installed providers (`apps/desktop/src/settings.ts`); an app is listed when the backend
  registry contains its App ID or a catalogued legacy registry ID. The Shell itself (`com.minhanos.desktop`)
  always provides Appearance.
- Entries with `requiredPermission` are listed only when `/api/v1/identity/me/access` returns that permission.
  This is cosmetic; panels' APIs keep enforcing authorization.
- app-sdk adds `useCatalog` (reference-counted catalog registration, so an app and its Settings panel can be
  mounted together), host events `session-ended`/`profile-changed`, and `setSettingsOpener`/`openSettings`
  for deep links (`false` in standalone hosts).
- Panels stay mounted after first visit so drafts survive panel switching; closing Settings asks every panel.
- `tests/settings-standard.mjs` (part of `npm run check`) fails the build when a manifest lacks
  `settingsIntegration`/`nameKey`, a label is missing in vi-VN or en-US, a declared panel has no provider,
  an app is not registered with the Settings host, or hardcoded Settings tabs return.

## Deferred

- Generic per-app settings storage API (versioned schema, user/system scope, audit). Current panels persist
  through existing validated APIs (profile, preferences, Identity). Build it when the first app needs new settings.
- System-wide sections (network, storage, updates), standalone Settings host, runtime manifest loading for
  non-compiled apps.
- Appearance panel text is still hardcoded Vietnamese (existing Desktop i18n debt); only its label is translated.

## Consequences

Adding an app now requires: manifest `nameKey` + `settingsIntegration` (or `[]`), catalog keys in both locales,
a provider export when it has panels, and registration in the Shell's provider list.

## Amendment: hybrid ownership model (accepted by user 2026-10-30)

User chose the hybrid model after reviewing Codex "Settings Ownership Policy V1". Implemented: Language & Region panel (com.minhanos.settings, Platform-owned locale/time zone), admin Users panel removed from Settings, Account Center shows a Settings link instead of its own locale/theme selectors when embedded (standalone keeps them), display name "MinhAnOS Settings". Codex rule 7 is replaced by the hybrid rule below.

Adopt (compatible with this ADR):
- Each setting has one owning module and namespace (`desktop.*`, `system.*`, `files.*`, `identity.*`, `storage.*`);
  the future Configuration Service stores them but writes go only through the owner's API/permission. No global
  `setSetting` for every namespace.
- Cross-app settings (locale, theme, date/time format, notification rules) are Platform-owned, edited once in
  Settings, read reactively by every app via app-sdk. Today locale/theme also have selectors inside Account Center
  (`profilePreference`) — now replaced by a Settings link when embedded; Language & Region panel added.
- Settings' own system menu as a roadmap: Appearance, Language & Region, Date & Time, Notifications, Network,
  Power, Updates, About — each shown only when a real backend exists (Network/Power need the restricted host agent).
- Display name "MinhAnOS Settings"/"Cài đặt" instead of "Control Center" (App ID unchanged).

Resolved conflicts:
- Codex rule 7 ("do not require apps to register settings in Settings") contradicts the user's rule that every app
  integrates into central Settings. Hybrid: the app owns its settings UI/API and also shows it in its
  own preferences; Settings renders the same exported component (no duplication, no access to app internals).
  Every manifest still declares `settingsIntegration` (empty allowed), so the rule stays test-enforced.
- Users: keep personal account entries (profile, security, sessions) in Settings
  as Account Center panels, like the account section in iOS/Windows Settings, and move admin "Users" back to
  Account Center only.
