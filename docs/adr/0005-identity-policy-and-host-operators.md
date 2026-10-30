# ADR 0005 — Account Center policy slice and explicit host operators

Accepted 2026-10-30. User delegated routine choices with full documentation. Branch account-center-phase3.

## Scope and authority

This release is Phase 3A, the protected Users/My Account policy/API slice. It does not complete the whole Phase 3 roadmap. Fourteen OpenAPI operations are implemented; all others keep x-implementation=planned. No placeholder Group/Session/Application response is advertised as working. Account Center UI remains Phase 4; the existing Settings creation form remains usable.

Use one shared identity.Service in Desktop and Drive. Server selects com.minhanos.account-center as the registered endpoint context; no client subject/appId/header grants authority. AuthCore verifies the real cookie realm, active state and epoch. Every enabled policy route checks a registered enabled app grant with exact system scope, user entitlement and target ceiling. Matching application deny wins; unsupported allow/scope data never widens permission. Same-origin native apps remain trusted code, not sandboxed identities.

During this bounded transition, users.role=admin is the single canonical System Admin flag and legacy mirror. The protected system-admin role catalog describes that flag; it is not a second independent grant path. Do not add a generic System Admin binding endpoint before migrating this authority consistently. Direct/group user-admin bindings with exact system scope delegate only standard-user management. Unknown scopes/custom role inheritance do not authorize this slice. Existing identity schemas reserve richer RBAC; group/custom-role/app-policy mutation remains closed.

Default active users can read/update their own safe profile when the app grant permits. User Admin can list/create/change standard users, never privileged direct/group delegates or System Admin, and cannot assign roles. Only System Admin with identity.roles.write can set the three supported web roles. Group-derived privilege blocks setting Standard User until that membership is resolved; do not silently promise a demotion while keeping inherited privilege. Self profile updates accept displayName only.

## Last administrator and concurrency

All enabled policy mutations, including legacy creation, take advisory transaction lock 681026 before user rows. Actor is revalidated from bearer/session in the transaction, then target/version/role and active System Admin count are checked. Disable/demote/soft-delete cannot remove the final active System Admin. Role changes/disable/delete invalidate target sessions in both realms; a deleted account cannot be enabled through the enable endpoint. Soft-delete requires disabled state and retains storage/profile ownership rows. No hard-delete API.

The invariant covers every currently enabled mutation path. Group/custom-role changes cannot bypass it because they are not enabled and do not grant System Admin. OS root/DBA remain trusted operators who can bypass application policy directly; this is not protection against database owners or old binaries run outside supported startup gates.

## Host privilege split

Identity migration 005 captures existing legacy admins once in identity_host_operators, keyed by unchanged storage ID. This preserves their previous test-VM operator entitlement, with origin pre-policy-operator. Later web admins receive no entry; restarting either host never snapshots them again.

Drive system-root registry and every system file route require active admin AND an enabled explicit host-operator entry. An IdentityStore without the host-policy adapter denies by default. Both standalone and native platform-session adapters use the same policy. Demotion to a non-System-Admin role or soft deletion revokes the operator entry; later promotion does not restore it automatically. No web endpoint grants host access; separate local operator tooling/review is needed for a future grant workflow.

This narrows web-admin authority; it does not redesign the existing broad root files-agent, protected paths or file ACLs. Explicit legacy operators still have their existing broad agent access. Commit-time authorization for long transfers and private/secret path policy remain Phase 5/hardening work.

## API safety

Administrative v1 writes and legacy admin creation require a current-password reauthentication marker, five-minute TTL on the verified session. Standard legacy creation keeps its existing workflow; all legacy admin user routes still check the same registered user-management grants. The Settings form requests the current operator password only for admin creation, calls reauthenticate, clears the local field and submits the existing legacy request. The extra field has vi/en messages, no credential persistence or Desktop-store dependency added.

Mutations validate strict JSON fields, stable errors, bounded bodies and quoted-decimal If-Match versions. POST state/create and PUT role use idempotency keys: actor + hashed key, HMAC of route/body/precondition, 24-hour TTL, maximum 1000 retained entries per actor and bounded expired cleanup. Cached responses are sanitized DTOs; credential input is never stored. Replays recheck actor, current grant/access and freshness before returning the cached result. JSON canonicalization prevents PostgreSQL jsonb key order from changing replay bytes. A self-revoking mutation can make a retry return 401 until login/reauthentication again.

Cursor pages are bounded 1–100; signed HMAC cursors bind public actor UUID, auth epoch, collection, search and last key, TTL 15 minutes. No storage IDs in cursor payload. Lists are ordered by stable ID/key, including audit events; chronological audit filtering is not implemented yet. Public user responses contain no role/hash/storage key. A persistent random signing key is generated using the existing cryptographic primitive, stored privately in the same DB, not in source/env/frontend.

v1 waits with 503/IDENTITY_NOT_READY if any historical user lacks public UUID/credential reference. Successful login lazily assigns only that verified user's missing references under its existing row lock; operator bulk backfill stays bounded. Do not hide unmapped users from lists as if migration were complete.

The v1 adapter has its own nested error envelope/request IDs, CSRF, strict JSON media type, Origin host/scheme and fetch-site checks. No trust of forwarded origin headers. Current deployment is direct LAN HTTP; HTTPS termination in a reverse proxy needs an explicit trusted-origin/proxy design before rollout. Legacy origin handling remains an older implementation to consolidate in hardening.

## Audit, migration and limits

Successful mutations, sanitized events and cached results commit atomically; audit failure rolls back the mutation. Denied requests have best-effort sanitized events, not a guaranteed independent compliance sink. Request IDs are generated by the server; audit details allow only reasonCode/changedFields. Legacy create keeps its old audit entry when that table exists. Login/logout auth events remain the Phase 2 mechanism; no claim of a unified full audit reader/retention policy.

Migration 005 is additive and identity-owned: operator snapshot, session freshness, policy signing key storage, idempotency/audit tables and protected role/permission/app grant seeds. Existing users/Files IDs and data paths do not change. An unexpected reserved-key conflict fails migration instead of overwriting a custom policy. SQL 001–005 and profile 001 are immutable after deployment; no down migration. Restore must coordinate database, source and later writes. Large-db/lock/performance certification and Files-volume recovery are not established by this small VM test.

Reference consulted: official [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html), particularly least privilege, default denial, checks on every request and authorization regression tests. This is not an independent security audit.
