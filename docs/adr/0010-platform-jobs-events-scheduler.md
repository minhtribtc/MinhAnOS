# ADR 0010 — Platform jobs, events and scheduler

Status: accepted 2026-10-31. Implements `contracts/jobs/jobs.v1.yaml` and `contracts/events/events.v1.yaml`
(ADR 0001), which were `planned`. Review that led here: [platform-capabilities-review](../platform-capabilities-review.md).

## Context

Only Files had durable background work (`files_jobs`, copy only). Storage operations ran inside requests, every
service grew its own `time.After` loop for retention, and no app could react to another app's change. Each new
app (SMB, NFS, Backup, App Center) would otherwise build its own queue, cron and notification glue.

## Decision

Three platform packages, all PostgreSQL-backed (no extra broker to operate on a NAS), each its own migration module:

**`internal/jobcore` (module `jobs`) — one job table for every app.**
- `platform_jobs`: `type` (`<app>.<verb>`, e.g. `files.copy`), `actor_id` (who may see/cancel it; NULL = system),
  `organization_id` (ADR 0008 scope for admin views), `payload` jsonb (app-defined, versioned by the type name),
  state machine of `jobs.v1`, progress with unit, attempts, `run_after` (backoff and delayed start), lease + fencing,
  cooperative cancel, error code, request/correlation IDs, and `dedupe_key` unique among unfinished jobs of a type.
- A `Worker` per service claims only the types it registered (`type = ANY(...)`, `FOR UPDATE SKIP LOCKED`), so
  Desktop, Files and future app services share the table without stealing work. Handler outcome: nil → succeeded;
  `ErrCancelled` → cancelled; an error with a stable `JobCode()` → failed with that code; any other error → retried
  with exponential backoff and jitter only if the handler is registered `Retry: true` (contract: never retry host side
  effects blindly), else failed `JOB_FAILED`. An expired lease beyond `maxAttempts` fails `JOB_INTERRUPTED`.
- **Scheduler** is part of the same module: `platform_schedules(key, job_type, every, next_run_at)`. A due schedule is
  advanced and its job enqueued in one transaction with `dedupe_key = schedule key`, so several instances never
  double-run it and a slow run never piles up. Ad hoc loops (Files audit retention, deleted-user removal) move here;
  jobcore schedules its own `jobs.retention`. High-frequency kick-driven loops (notification delivery every 10 s,
  quota measurer) stay loops: they are delivery pumps, not tasks.

**`internal/eventcore` (module `events`) — transactional outbox with per-consumer delivery.**
- `Publish(ctx, tx, Event)` requires the caller's transaction: the event exists iff the domain change commits.
  Envelope = `event.v1` (eventId, name `<domain>.<thing>.<verb>.v<N>`, schemaVersion, source, subject,
  organizationId, correlationId, data). No secrets or file content in `data`.
- A consumer is a named, durable subscription (`platform_event_consumers`) created at its first start from the then
  latest event (no replay of history by default). Delivery is at-least-once: the handler runs in a savepoint of the
  same transaction that records `(consumer, event)` as done, so side effect and dedupe commit together. Failures back
  off; after 8 attempts the delivery is `dead` (kept for diagnosis, visible to operators later). A consumer's
  watermark (`since_seq`) only passes events that are done or dead and whose inserting transaction (`xid`, events SQL
  002) is older than the snapshot xmin, so an event committed late by a long transaction is never skipped.
- Events never carry authority: security-critical effects (session revocation on disable) stay synchronous in the
  domain transaction. Events are for caches, follow-up work and external services.
- First event: `identity.user.disabled.v1` on disable; first consumer: Files cancels the user's unfinished jobs.

## Migration of existing data

`files` step 6 copies `files_jobs` rows into `platform_jobs` keeping their IDs (open Files windows keep polling the
same job). `files_jobs` is no longer written; dropping it is a later contract step (migration policy). Files declares
a dependency on `jobs` with range [1,100) so future additive `jobs` steps stay compatible.

## Consequences

- New apps add background work by registering a handler and calling `jobcore.Enqueue` — no table, no loop.
- Jobs and events are retained 7 / 30 days by scheduled jobs.
- Deferred, with the schema already able to carry them: a cross-app Jobs view (`actor_id`, `organization_id` exist),
  operator view of dead deliveries, `files.*` change events (published when the first consumer — search/sync — exists),
  moving storage operations (share creation, snapshot) onto jobs when they become long-running.
