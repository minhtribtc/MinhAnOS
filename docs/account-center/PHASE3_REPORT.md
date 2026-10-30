# Account Center — Phase 3A implementation report

2026-10-30, branch account-center-phase3. Phase 3A completes the protected Users/My Account slice; the full Phase 3 roadmap still has remaining work. Decisions: [ADR 0005](../adr/0005-identity-policy-and-host-operators.md).

## Shipped scope

- Shared backend policy for verified actor + registered Account Center app grant + user/target permission, with conservative application deny and exact supported scopes.
- System Admin, delegated User Admin and Standard User behavior. User Admin cannot manage privileged direct/group subjects, grant a role or use host root. System Admin is still canonically users.role=admin during this transition; protected catalog and delegated bindings do not create another System Admin authority.
- Race-safe last-active-admin invariant, optimistic versions, request IDs, sanitized transactional mutation audit, bounded signed cursors and idempotent mutation replay with reauthorization.
- Separate host-operator entitlement captured once for existing operators. New System Admin has no host file authority in either cookie realm or native gateway.
- Current-password freshness for sensitive writes. Existing Settings admin creation has a vi/en current-password field, clears the transient value, and keeps the legacy request shape. Deleted users are omitted from the legacy list.
- Readiness gate for remaining historical UUIDs; successful login backfills only the verified row, no storage rekey.

## Implemented operations

GET me; PATCH me; POST reauthenticate; GET/POST users; GET/PATCH/DELETE users/{userId}; POST users/{userId}/disable; POST users/{userId}/enable; PUT users/{userId}/role; GET roles; GET permissions; GET audit-events. Prefix /api/v1/identity. Inventory is authoritative in contracts/identity/status.v1.json and OpenAPI x-implementation. SDK remains 0.1.0; client/types regenerated, including PUT support.

Other spec operations return unsupported rather than fake data. Group CRUD/membership, custom role/binding mutation, application policy, full v1 session/password/credential adapters, Linux identity status/provisioning and Account Center UI are not shipped in this slice.

## Security and migration review

All enabled policy mutations share lock 681026 before user rows and use transaction-time actor/target checks. Last-admin count covers disable, demote and soft-delete; group/custom-role routes cannot confer System Admin here. Role changes, disable and delete invalidate both realms. Soft-delete retains the users row, physical key, data ownership and profile; it cannot be re-enabled by the enable endpoint.

App IDs/subject headers never confer authority. User profile input is an allowlist. Role grants require identity.roles.write, System Admin and freshness; User Admin has a ceiling. Invalid/empty scopes do not authorize. Operator access requires a distinct enabled row plus active System Admin, not just role. Repeated initialization does not grant it to later admins. Demotion permanently revokes that entry until an explicit local operator workflow.

Private backups: /opt/minhanos-backups/before-identity-phase3.sql and .tgz, 0600. Restored the actual pre-upgrade DB into an isolated PostgreSQL container and applied migration 005 before deployment. No applied migration was edited. Identity 001–005 and profile 001 are now frozen. No filesystem rekey/chown/root-mount changes or administrator password change. Full Files-volume recovery and large-db/performance guarantees remain outside this rehearsal.

## Verification

- go test ./... and go vet ./...; Docker Desktop/Drive builds: PASS.
- Dedicated PostgreSQL tests: PASS existing AuthCore/adoption/backfill plus policy concurrency, last admin, new-admin host denial, User Admin ceiling, group-derived protection/deny, role/scope rejection, self-only profile, ETag/precondition, replay/conflict/reauthorization after grant revoke, audit rollback, soft-delete/session revocation, cursor tampering/filter/route binding and historical UUID gate.
- Actual HTTP adapter test: PASS legacy creation freshness, new web admin no host grant even after reinitialization, legacy and v1 grant revocation, CSRF/Origin/rotation and both-realm auth regression.
- Contracts/generation/strict TypeScript/OpenAPI validation: PASS; one unchanged unused IdentityLink warning. Additive OpenAPI breaking comparison against Phase 2: PASS.
- Real pre-upgrade DB restore: PASS. Pre-deploy cookies in both realms remained valid after upgrade and were logged out; exact private cookie fixture removed.
- Runtime Identity smoke on VM: PASS schema-validated user/role/permission/audit DTOs, new System Admin denied host in Drive AND native gateway, delegated ceiling, replay, ETag, cursor pages, disable/enable/soft-delete. Exact three fixture rows removed after each run.
- Existing profile, embedded Files, Drive/real Debian agent and API smoke: PASS; fixture users/private namespaces removed, actual operator still works. No production account credential/status/role changes.
- Browser: verified the existing Settings form in a real IAB tab; selecting admin shows the new current-password field and operator-access explanation. No privileged form submission through the browser; API/integration tests verified submission. Fixed a stretched action button observed in the screenshot. Other legacy labels/i18n debt remain.

The first replay test failed because jsonb reordered keys; canonicalized original/cached JSON and reran successfully. Additional review closed legacy admin-read grant bypass and inherited-role ceiling/deny gaps before release. No claim that new UI, complete RBAC, audit retention or Files ACL/private-path isolation is finished.

- Fresh standalone project minhanos-policy-fresh: PASS login/public UUID/v1 me/roles/ETag/reauth without Desktop. Exact test containers/volumes were removed. Production Drive also worked with Desktop stopped; Desktop was restarted healthy.
- Final UUID readiness-gate regression, build and contract checks: PASS. No further test rounds after the user requested stopping repeated interactive trials.

## Remaining Phase 3 work

Phase 3B must complete group/custom-role/app-access administration with permission ceilings, role assignment reading, supported scope semantics and last-admin proof before enabling them. Add the remaining v1 session/password/credential adapters with transactional replay/audit and full public DTO pagination. Then Phase 4 builds the shared native/standalone Account Center app. Do not enable third-party apps, OIDC, email reset, MFA or POSIX provisioning from this milestone alone.

## File inventory

- `AGENTS.md`
- `CHANGELOG.md`
- `README.md`
- `apps/desktop/src/components/AppContent.vue`
- `apps/desktop/src/style.css`
- `contracts/README.md`
- `contracts/api/openapi.yaml`
- `contracts/identity/README.md`
- `contracts/identity/index.v1.json`
- `contracts/identity/reauthenticate.v1.schema.json`
- `contracts/identity/set-role.v1.schema.json`
- `contracts/identity/status.v1.json`
- `docs/HANDOFF.md`
- `docs/README.md`
- `docs/account-center/IMPLEMENTATION_PLAN.md`
- `docs/account-center/PHASE3_REPORT.md`
- `docs/account-center/README.md`
- `docs/adr/0005-identity-policy-and-host-operators.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/drive.md`
- `docs/migration-policy.md`
- `docs/roadmap.md`
- `docs/user-guide.md`
- `docs/validation.md`
- `internal/drive/auth.go`
- `internal/drive/files_test.go`
- `internal/drive/server.go`
- `internal/identity/auth.go`
- `internal/identity/http.go`
- `internal/identity/migrations/005_policy.sql`
- `internal/identity/policy.go`
- `internal/identity/policy_test.go`
- `internal/identity/read.go`
- `internal/identity/schema.go`
- `internal/platform/auth_integration_test.go`
- `internal/platform/server.go`
- `packages/api-client/src/identity.generated.d.ts`
- `packages/api-client/src/identity.generated.js`
- `packages/app-sdk/locales/common.json`
- `packages/shared-types/src/index.d.ts`
- `scripts/generate-identity-client.mjs`
- `tests/contracts.mjs`
- `tests/identity-contracts.mjs`
- `tests/identity-phase3-cleanup.sh`
- `tests/identity-phase3-smoke.mjs`
- `tests/identity-standalone-fresh.mjs`
- `tests/shared-types.ts`
