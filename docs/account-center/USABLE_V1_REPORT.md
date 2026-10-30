# Account Center — usable v1 (2026-10-30)

Branch `settings-central-v1`. Scope chosen by the user: admin password reset, user detail page, sign-in audit
(first part of the Notification/Log review Phase A). Builds on UI v1 and the central Settings slice.

## Delivered

- **Admin password reset** — `POST /api/v1/identity/users/{userId}/password` (`identityResetUserPassword`,
  34 active operations). Needs `identity.users.write`, reauthentication within 5 minutes, `If-Match` and
  `Idempotency-Key`; same target ceiling as other user changes (User Admin: standard users only). Never the
  caller (`IDENTITY_USE_OWN_PASSWORD`) and never an enabled host operator (`IDENTITY_HOST_OPERATOR_PROTECTED`,
  otherwise a web admin could take over host access). Hashing happens before the policy lock. Revokes the
  target's sessions in both realms; audited as `user.password.reset`; the response never echoes credentials.
- **User detail page** — Users tab → click a name: profile, status, effective access (role, host operator,
  permissions), groups (`GET groups?member=`), recent activity (`GET audit-events?target=`), and actions:
  edit, role, reset password, disable/enable, soft delete. Leaving the tab closes it.
- **Sign-in audit** — identity SQL step 006 (`auth_audit`) widens `identity_admin_events` (outcome `failure`;
  details `realm`, `count`) and adds newest-first, target and actor indexes. No new audit table.
  - `auth.login` success commits in the same transaction as the session.
  - `auth.logout` is one statement with the session delete.
  - `auth.login.failed` for existing accounts only, sampled at attempts 1, 5, 10, 20 of the 10-minute throttle
    window (bounded; unknown usernames never recorded). Reason `INVALID_CREDENTIALS` or `ACCOUNT_INACTIVE`.
- **Audit list fix** — events were ordered by random UUID; now newest first by `(occurred_at, id)` with a signed
  composite cursor. The Audit tab shows the attempt count for sampled failures.

## Known limits

- Sign-in audit request IDs are generated per event; there is no request/correlation middleware yet.
- Sign-in failures are best-effort (logged if the insert fails); success/logout audit is transactional, so a
  missing audit table now blocks sign-in by design.
- Legacy `audit` (Desktop activity) and `drive_audit` tables are unchanged; consolidation is later Phase A work.
- No audit retention job yet; no forced password change at next sign-in.

## Verification

See [validation](../validation.md). Browser acceptance needs the user to sign in.
