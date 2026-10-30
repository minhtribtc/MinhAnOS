# ADR 0001 — Contract-first platform integration

Status: accepted, 2026-10-30. Branch: platform-contracts-v1.

Use versioned JSON Schema/OpenAPI contracts shared by peer apps and replaceable Shells.
Seven areas are required: manifest/SDK, authorization, APIs, migrations, jobs/uploads, file resources and events.
Machine-readable sources live in contracts; generated TypeScript lives in @minhanos/shared-types.
The transport/error parser lives in @minhanos/api-client, independently of Vue and Desktop.

Compatibility decisions:

- Keep current SDK package 0.1.0; schema protocol version 1 is independent. Do not claim SDK 1 stability.
- Stable canonical app IDs use reverse DNS. Preserve old registry aliases/layout IDs during migration.
- Manifest capabilities request access; backend ANDs user rights with verified app grants and resource scope.
  Neither client subject nor client appId establishes identity. Same-origin native apps are trusted code, not sandboxed tenants.
  Before third-party apps, add origin/sandbox isolation, audience/resource-scoped delegation, revocation and grant storage.
- Current /api keeps `{code,error:string}`. New /api/v1 adopts nested errors with requestId and details;
  client supports both now. No invented requestId is added on the client. Existing backend is not falsely upgraded.
- Opaque cursor pagination, bounded limits and stable ordering; cursors confer no access rights.
- Idempotency records bind verified user/app/route/key + canonical request hash and original result.
  Reauthorize replay, reject changed payload; durable atomic reservation prevents concurrent duplicate commits.
- File IDs must survive rename/move; never derive stable identity from path. Persistent mapping, versions/strong ETags,
  ACL inheritance and events require a real migration before v1 file resource API is enabled.
- Jobs are at-least-once: lease/heartbeat/fencing, bounded retries and staged atomic output. Do not promise exactly-once
  execution or retry arbitrary commands. Cancellation is cooperative and cannot undo an already committed result.
- Upload parts and sessions persist outside the browser. Enforce whole-file/part checksums, quotas, expiry,
  scoped authorization, commit reconciliation and cleanup that cannot delete published destination files.
- Events use transactional outbox, per-aggregate ordering/version, consumer dedupe and dead letters;
  events are notifications, not authority. No file contents or secrets in payloads.
- Stable API removal requires a new major, at least 180 days announced deprecation after stable launch,
  adoption evidence and a tested migration. Preview contracts may evolve only through reviewed compatibility changes.

No new worker, upload service, ACL store, event bus, manifest router or authorization endpoint is delivered here.
Implement these behind explicit capabilities/readiness; schema presence must never mark a feature operational.

Next implementation: migration runner + identity/app grants → persistent file identity → durable uploads/job worker
→ outbox/event consumers → native viewer integration. Finish the existing i18n migration in parallel with those tasks.
