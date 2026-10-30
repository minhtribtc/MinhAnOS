# Identity contracts v1

Source authority: index.v1.json lists public schemas/types; OpenAPI operations remain planned. status.v1.json declares runtime boundaries. Regenerate with `npm run contracts:generate`; verify with `npm run contracts:check`.

No credential secrets, legacy storage IDs or raw bearer tokens belong in public DTOs. API client export `@minhanos/api-client/identity` is transport scaffolding, not evidence of enabled endpoints. SDK stays 0.1.0; host protocol 1 is separate.

See [Phase 1 report](../../docs/account-center/PHASE1_REPORT.md) for migration policy, bounded backfill, actual tests, security gaps and deployment commands.
