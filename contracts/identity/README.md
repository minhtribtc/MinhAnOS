Phase 3A: the exact active operation list is in status.v1.json; OpenAPI x-implementation matches it. User-management subset is real; other generated methods remain planned and unsupported. [Report](../../docs/account-center/PHASE3_REPORT.md) and [ADR 0005](../../docs/adr/0005-identity-policy-and-host-operators.md).

Runtime Phase 2 now uses shared AuthCore for legacy authentication. Identity v1 HTTP remains planned. [ADR 0004](../../docs/adr/0004-shared-auth-core-and-recovery.md) records realm mapping, recovery policy and security boundaries.

# Identity contracts v1

Source authority: index.v1.json lists public schemas/types; OpenAPI operations remain planned. status.v1.json declares runtime boundaries. Regenerate with `npm run contracts:generate`; verify with `npm run contracts:check`.

No credential secrets, legacy storage IDs or raw bearer tokens belong in public DTOs. API client export `@minhanos/api-client/identity` is transport scaffolding, not evidence of enabled endpoints. SDK stays 0.1.0; host protocol 1 is separate.

See [Phase 1 report](../../docs/account-center/PHASE1_REPORT.md) for migration policy, bounded backfill, actual tests, security gaps and deployment commands.
