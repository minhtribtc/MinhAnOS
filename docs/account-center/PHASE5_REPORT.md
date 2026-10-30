# Account Center — Phase 5 report: Files authorization and app access (2026-10-30)

Branch `settings-central-v1`. Completes the plan up to Phase 5 (see [Phase A report](PHASE_A_REPORT.md) for
the audit/event foundation and admin session work delivered on the way).

## Delivered

- **Files authorized by the identity policy** — identity SQL 009 (`files_policy`) registers `files.read`,
  `files.write`, `files.system.read`, `files.system.write`, grants them to the Files app
  (`com.minhanos.files`) and to roles: every account (standard-user, user-admin, system-admin) keeps
  `files.read/write` on its own namespace; `files.system.*` only System Admin. Drive checks every request
  (`/api/roots` and `/api/files/*`, standalone and through the Desktop gateway) with
  `Service.CheckApp`: active user AND app grant AND no deny rule AND entitlement. Resource rules stay in Files:
  own namespace by immutable storage ID, system root additionally needs an enabled host operator and the agent.
  `IdentityStore.Authorize` is a required interface method, so a new identity adapter cannot skip it.
- **App access rules** — `GET /applications` (`identityListApplications`, now implemented),
  `GET|POST /applications/{appId}/access`, `DELETE /applications/{appId}/access/{ruleId}` (40 operations).
  Deny rules target a user, group or role; deny always wins and also applies to admins. Only System Admin with
  `identity.apps.write`, reauthentication and idempotency; audited `app.access.denied` / `app.access.removed`.
  Account Center itself is governed by roles only (`IDENTITY_PROTECTED_POLICY`) so no rule can lock every
  administrator out. One rule per app and subject (unique index).
- **Effective access** now lists Files permissions and drops them when a deny rule applies.
- **Desktop launcher** hides Files for users who may not read it.
- **UI** — Account Center tab *Apps*: apps and whether the viewer may use them; for Files the deny rules with
  subject names, "Block access" (user search, group or role) and "Allow again".

## Not in scope / later

- Allow rules, per-folder ACLs and shared folders (see [storage review](../storage-model-review.md)); Files still
  has only private namespaces plus the operator system root.
- Policy for apps other than Files waits until those apps call `CheckApp`.
- Phase 6 POSIX provisioning, MFA.

## Verification

See [validation](../validation.md).
