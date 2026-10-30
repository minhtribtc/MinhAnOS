# ADR 0006 — Security and bounded delegation APIs

Accepted 2026-10-30, branch account-center-phase3b. Continues the user's authorized implementation with focused automated verification, no interactive test loops.

## Decisions

Reuse the existing schema and the shared transaction wrapper. No migration 006, new credential store, IAM service or deployment dependency. Applied identity 001–005/profile 001 stay immutable. Runtime inventory grows from 14 to 32 operations, including paginated own sessions/credential references, password change, group CRUD/membership, custom role creation, binding list/create/removal and effective access for self/authorized targets.

Password verification and replacement hashing happen under the existing two-slot KDF limit outside the policy/user-row transaction. The committing transaction revalidates the verified bearer, current permissions and the exact prior hash; it changes the hash/epoch, clears recovery via the existing trigger, removes both session realms, appends auth/admin events and records the sanitized idempotency result atomically. Wrong/current stale credentials never authorize replacement. New password requires 12 Unicode characters, maximum 256 UTF-8 bytes. A successful change revokes the current session too; a lost-response retry with that old cookie returns 401, not an automatic second change. No password plaintext/hash/salt appears in credential DTOs or cached responses. Request fingerprints remain HMACs.

Session pages combine Desktop/Files handles, derive owner from the verified cookie and reveal no bearer/token hash/CSRF. createdAt can be unknown for historical rows; lastSeenAt/deviceHint remain explicit NULL rather than invented device information. Revoke-one filters by owner. Revoke-others preserves only the current session of its realm, removes the user's other sessions in both realms and caches an accepted result. Replaying that key does not turn into a later sweep of newly created sessions. This is not an immediate abort of already authorized in-flight transfers.

Policy administration requires System Admin, its registered app grant and recent password verification, under the shared mutation lock. Protected System Admin and Standard User cannot be assigned via generic bindings. System Admin remains canonical in users.role and only the existing guarded role endpoint changes it. Thus group/custom-role operations cannot create a second last-admin authority or grant host operator. User Admin bindings and custom bindings support exact system scope only; user/storage/unknown scopes are rejected rather than widened.

Custom roles can delegate registered read/self/bounded user-management permissions. They cannot contain identity.roles.write, identity.groups.write or identity.apps.write; governance cannot be delegated transitively. Other unknown permissions fail the existing registered app-grant check. User-management target ceilings still protect accounts with direct/group bindings. Group membership changes use the group's If-Match version; binding removal uses its own version. Group deletion only applies to user-owned groups and removes their relations through existing FKs; it does not change canonical System Admin flags or users/files ownership. Permission changes are observed on subsequent requests and all replay authorization, without a stale cached grant list.

Effective-access DTO separates informational baseRole, permission prerequisites and hostOperator. Disabled/deleted targets have no effective permissions or host entitlement. Permission alone does not mean a feature is implemented: clients must also use the operation inventory. These DTOs preserve UUID boundaries rather than making UI join legacy storage IDs.

## Deliberate boundaries

Do not activate app-access/grant mutation before all affected apps enforce it. Files still needs its resource/ACL integration and the app registry needs an authoritative installation model; publishing a control that claims to deny Files while Files ignores that deny would be misleading. Keep those endpoints planned. User/storage scope, custom-role editing/deletion, protected group administration, chronological/unified audit retention and Linux/OIDC/MFA/email integrations remain separate work.

Account Center frontend is the next milestone: one peer package mounted natively in Desktop plus an independent host. This release adds real backend APIs and generated client methods, not a new launcher application or fake screens. Keep old Settings operational and document limitations.
