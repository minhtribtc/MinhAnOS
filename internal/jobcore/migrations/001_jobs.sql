-- Platform jobs and schedules (ADR 0010, contracts/jobs/jobs.v1.yaml): one durable table for the background work of
-- every app. Workers claim only the types they handle; dedupe_key is unique among unfinished jobs of a type, so a
-- schedule or a repeated request never queues the same work twice. Finished jobs are removed after 7 days.
CREATE TABLE platform_jobs(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 type text NOT NULL CHECK(type ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$' AND length(type)<=100),
 actor_id text,
 organization_id uuid,
 payload jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(payload)='object'),
 dedupe_key text CHECK(length(dedupe_key) BETWEEN 1 AND 200),
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
 unit text NOT NULL DEFAULT 'steps' CHECK(unit IN ('bytes','items','steps')),
 completed bigint NOT NULL DEFAULT 0 CHECK(completed>=0),
 total bigint CHECK(total>=0),
 attempt integer NOT NULL DEFAULT 0 CHECK(attempt>=0),
 max_attempts integer NOT NULL DEFAULT 3 CHECK(max_attempts BETWEEN 1 AND 20),
 run_after timestamptz NOT NULL DEFAULT now(),
 cancel_requested boolean NOT NULL DEFAULT false,
 lease_until timestamptz,
 fencing bigint NOT NULL DEFAULT 0,
 error_code text CHECK(error_code ~ '^[A-Z][A-Z0-9_]*$'),
 request_id text NOT NULL DEFAULT '',
 correlation_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 CHECK((state IN ('succeeded','failed','cancelled'))=(finished_at IS NOT NULL))
);
CREATE INDEX platform_jobs_ready ON platform_jobs(type,run_after) WHERE state='queued';
CREATE INDEX platform_jobs_leased ON platform_jobs(type,lease_until) WHERE state='running';
CREATE INDEX platform_jobs_actor ON platform_jobs(actor_id,created_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX platform_jobs_organization ON platform_jobs(organization_id,created_at DESC) WHERE organization_id IS NOT NULL;
CREATE INDEX platform_jobs_finished ON platform_jobs(finished_at) WHERE finished_at IS NOT NULL;
CREATE UNIQUE INDEX platform_jobs_dedupe ON platform_jobs(type,dedupe_key) WHERE dedupe_key IS NOT NULL AND state IN ('queued','running');

CREATE TABLE platform_schedules(
 key text PRIMARY KEY CHECK(key ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$'),
 job_type text NOT NULL CHECK(job_type ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$'),
 every_seconds integer NOT NULL CHECK(every_seconds BETWEEN 60 AND 31536000),
 next_run_at timestamptz NOT NULL DEFAULT now(),
 last_job_id uuid
);
