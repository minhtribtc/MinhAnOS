# Account Center Phase 3B — implementation report

2026-10-30. Branch account-center-phase3b; follows Phase 3A. Decisions: [ADR 0006](../adr/0006-identity-security-and-delegation-apis.md).

## Delivered

32 active Identity v1 operations, up from 14. Added own-session pagination/revoke/revoke-others, password change, credential metadata, group create/list/rename/delete/membership, custom role create, role-binding list/create/remove and effective-access reading for self/authorized users. OpenAPI flags, runtime inventory, schemas, strict TypeScript types and generated client agree. Both existing hosts register the same shared service; no Desktop dependency added to Drive.

Mutations keep verified actor/app/target checks, the common lock, optimistic versions, current-password freshness for policy administration, transactional audit and replay authorization. Password changes invalidate both cookie realms and recovery, including the current session. Session revocation is owner-only. Credential/session DTOs contain no raw tokens, CSRF or password material. Unknown device/last-seen information stays null.

Custom-role delegation is real for supported registered permissions, with a ceiling on policy-governance writes. Generic bindings cannot assign System Admin or implicit Standard User, and only exact system scope is accepted. Existing last-admin and explicit host-operator boundaries remain unchanged. Group membership/removal and bindings update effective access on subsequent requests. Effective access of an inactive target contains no active permissions/host entitlement.

## Verification actually performed

Used a separate source folder /opt/minhanos-phase3b-check; no browser interaction or Desktop stop/start trials. Go test ./..., go vet ./..., dedicated disposable-PostgreSQL integration and root contract/type/i18n/boundary checks passed. Existing full database auth/policy/migration regression passed once. Focused new cases covered owner isolation across both realms, revoke-others replay/current-session preservation, password change and stale credential denial, credential redaction, audit rollback, group/custom-role inheritance/removal, protected role/scope/ceiling rejection and stale group versions.

Added actual httptest requests for session pages, password change, group creation and target effective-access DTO. Targeted cases were rerun after extending those handlers; no interactive VM smoke loops. Corrected the group-deletion audit label and checked the persisted event in the affected database test. Generated client/TypeScript checks passed; one existing unused IdentityLink warning remains. CI is triggered on push; do not claim its result without observing it.

## Migration and deployment

No schema migration or seeded privilege change. Existing ledger/checksums remain identity 001–005 and profile 001. No user/file rekey, ownership changes, production password mutation, new root grant or secret/env modification. Desktop and Drive Docker builds passed. Deployed the existing services once after verification; Desktop, Drive and DB reported healthy. No new DB restore rehearsal or broad runtime smoke was needed for this schema-free increment; prior restore evidence stays in Phase 3A. New runtime behavior is validated by isolated integration/HTTP tests, not a claim of production certification.

## Remaining work

Build the shared native/standalone Account Center UI against these APIs. App-access/grant editing stays planned until Files/resource enforcement and registry support are real; do not build a control with no effect. User/storage scope, editing/deleting custom roles, complete audit retention/filtering and provisioning/OIDC/MFA/email reset remain outside this milestone. Current legacy Settings is still the only account UI.

## File inventory

- `AGENTS.md`
- `CHANGELOG.md`
- `README.md`
- `contracts/README.md`
- `contracts/api/openapi.yaml`
- `contracts/identity/README.md`
- `contracts/identity/access.v1.schema.json`
- `contracts/identity/group-member-page.v1.schema.json`
- `contracts/identity/index.v1.json`
- `contracts/identity/role-binding-page.v1.schema.json`
- `contracts/identity/status.v1.json`
- `docs/HANDOFF.md`
- `docs/README.md`
- `docs/account-center/IMPLEMENTATION_PLAN.md`
- `docs/account-center/PHASE3B_REPORT.md`
- `docs/account-center/README.md`
- `docs/adr/0006-identity-security-and-delegation-apis.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/roadmap.md`
- `docs/user-guide.md`
- `docs/validation.md`
- `internal/identity/http.go`
- `internal/identity/management.go`
- `internal/identity/phase3b_test.go`
- `internal/identity/policy.go`
- `internal/identity/policy_test.go`
- `internal/identity/read.go`
- `internal/identity/security.go`
- `packages/api-client/src/identity.generated.d.ts`
- `packages/api-client/src/identity.generated.js`
- `packages/shared-types/src/index.d.ts`
- `tests/shared-types.ts`
