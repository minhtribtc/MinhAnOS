# Platform capabilities review (2026-10-31)

Checks an external proposal (14 shared platform capabilities; "Event Bus + Job Engine + App Lifecycle first")
against the code on `settings-central-v1`. **Update 2026-10-31:** Job Engine, Scheduler and Event Bus are implemented — see [ADR 0010](adr/0010-platform-jobs-events-scheduler.md).

Verdict: the direction is right and mostly already decided in
`contracts/` (ADR 0001), but several items are contracts marked `"implementation": "planned"`, not running code.

| Capability | Proposal | Actual state in code | Gap |
|---|---|---|---|
| Identity & Account | done | Done: users, groups, organizations (ADR 0008), sessions, first-run setup (ADR 0009) | — |
| Authorization & Policy | done | Done: roles/permissions/bindings, app access + grants, org roles, host operators (ADR 0005/0006) | Per-resource ACL only for Shared Folders |
| Logging & Audit | done | Done: `identity_admin_events`, `storage_events`, `files_events`, auth events, Log Center | No single write API for third-party apps |
| Notification | done | Done: per-domain outboxes → `notifycore` worker (`FOR UPDATE SKIP LOCKED`) | Producers are internal only |
| Resource & Quota | P0 | Done for storage: volumes, carve-out allocations, quotas, reservations, alerts | No CPU/RAM/app quotas (not needed yet) |
| Event Bus | P0 | Contract only: `contracts/events/events.v1.yaml` (at-least-once, transactional outbox, dedupe by eventId+consumer, dead letter), 4 events, `planned`. No publisher, no consumer | Real gap |
| Job Engine | P0 | Contract `contracts/jobs/jobs.v1.yaml` (`planned`, lease + fencing). One real implementation: `files_jobs` (copy only) with claim, fencing, retry, cancel, progress | Not generic; storage work is synchronous |
| App Lifecycle & Registry | P0 | Static: `contracts/apps/catalog.v1.json`, manifests + schema, backend registry, per-app access. `lifecycle.v1.yaml` `planned` | No install/upgrade/uninstall — fine while all apps are built in |
| Settings & Configuration | P0 | Central Settings app (ADR 0007), profile prefs, `storage_settings`. Generic versioned per-app settings API explicitly **deferred** in ADR 0007 | Deferred on purpose |
| Secrets & Credentials | P0 | Only file secrets (`secrets/`, `*_FILE` env), Argon2 password hashes | No store — but no SMB/NFS/Mail/Backup app needs one yet |
| Storage Access | P0 | Done: Storage Manager owns shares/volumes; Files goes through the storage agent; Shared Folder ACL | — |
| Scheduler | P1 | Ad hoc tickers (notify delivery, retention cleanup, Files jobs, quota) | No shared scheduler |
| Search & Indexing | P1 | Only list filters in identity APIs | Not started |
| Health & Metrics | P1 | `GET /api/health` on desktop and drive, compose healthchecks | No metrics |
| Backup/Restore (cross-cutting) | — | `tests/backup-restore.sh` restores a pg_dump and checks it | No user-facing Backup app |
| API versioning / migrations | — | `/api/v1`, versioned contracts + generated clients, transactional migration runner (ADR 0002), migration policy | — |

## Where the proposal is wrong or overstated

- **"user.disabled must fan out to SMB/Files/NFS via the bus"**: disabling already revokes every session in the same
  transaction (`identity.revoke`), and each request re-checks state, roles and org membership. The proposal itself
  says sensitive revocation must not rely on event delivery — that is how it works now. The bus would only be for
  caches and external services (SMB/NFS) that do not exist yet.
- **"Tenant Context" as P0**: ADR 0008 deliberately has no session tenant; the organization is a per-request selector
  checked server-side. Adding a tenant context would reintroduce what ADR 0008 rejected.
- **Secrets Manager / App Lifecycle as P0**: they become necessary with the first app that stores external credentials
  (SMB, Backup to cloud, Mail) or the first app not built into the image. Building them earlier means designing
  without a real consumer.
- `tenant_id` on events: here the field is `organizationId` (ADR 0008), and not every event belongs to one.

## Recommended order

1. **Generic Job Engine** — promote `files_jobs` into a platform `jobs` table + worker that implements `jobs.v1.yaml`
   (it already has lease/fencing/retry/cancel), then move storage operations (create share, quota recalculation,
   snapshot) onto it. Highest value: NAS work is long-running.
2. **Event outbox + consumers** — one platform outbox implementing `events.v1.yaml`, reusing the notification-outbox
   pattern; first consumers Log Center and Notifications. Do it together with the first app that needs it (SMB/NFS).
3. **Shared scheduler** — fold the ad hoc tickers into scheduled jobs on the engine from step 1.
4. **Secrets API, App Lifecycle, per-app settings API** — with their first real consumer (SMB/NFS or App Center).
