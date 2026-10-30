# Account Center UI v1 — Phase 4 report (2026-10-30)

Branch `account-center-ui-v1`, built on `account-center-phase3b` (42b8916).

## Delivered

- `@minhanos/accounts-app` 0.1.0 (`packages/accounts-app`): one shared Vue component for Account Center.
  Tabs: Profile, Users, Groups, Roles, Sessions, Security, Audit. It only calls the generated
  `@minhanos/api-client/identity` client (`<apiBase>/v1/identity`) plus legacy `/login` and `/logout`.
  Catalog keys are `accounts.*` (vi-VN/en-US) registered through `@minhanos/app-sdk`; no Desktop imports.
- Native Desktop app `com.minhanos.account-center` (registry + catalog + manifest). Desktop renders the same
  component with `embedded`, keeps the instance on app switch and asks before closing with unsaved drafts.
- Standalone host `apps/accounts` (`minhanos-accounts`), served at `/accounts/` by **both** existing backends
  (Desktop 8081, Drive 8082) through `identity.MountAccounts`. No new service, port or iframe. It authenticates
  against the realm of the host serving it. Static root is `ACCOUNTS_STATIC_DIR` (default `/app/accounts`);
  missing assets return 404, not an SPA fallback.
- New operation `identityGetGroup` (`GET /api/v1/identity/groups/{groupId}`, ETag = version):
  33 active Identity operations. `contracts/identity/status.v1.json` now reports `accountCenterUIEnabled: true`.
- Session expiry: password change or revoking the current session emits `session-ended`; Desktop calls
  `expireSession()` (no second authenticated logout). Any `AUTH_REQUIRED` from the current auth epoch also expires
  the Desktop session. A session epoch prevents late saves of the previous account from touching the new session.

## Not delivered / limits

- App access policy editing stays hidden: Files/registry resource enforcement is not wired yet.
- Scopes other than exact system scope, MFA, SSO, password reset by admin and POSIX provisioning are not in the UI.
- Legacy Settings → Users remains; both use the same backend policy.
- Many Desktop strings outside Account Center are still hardcoded Vietnamese (existing i18n debt).

## Verification

See [validation](../validation.md#account-center-ui-v1--2026-10-30). No repeated browser/stop-start trial loop,
per the user's verification preference.
