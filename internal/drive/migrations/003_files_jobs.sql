-- Durable Files jobs (app standard row 20, contracts/jobs/jobs.v1.yaml): copying a folder or a file above the
-- request limit runs in the background with progress, cancel, bounded retries and a lease with a fencing token,
-- so a crashed or restarted Drive resumes the work instead of losing it. Source and destination are paths in
-- the user's own home or a Shared Folder the user may write; finished jobs are removed after 7 days.
CREATE TABLE files_jobs(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 user_id text NOT NULL,
 type text NOT NULL CHECK(type IN ('files.copy')),
 root text NOT NULL CHECK(root ~ '^(personal|share:[a-z0-9][a-z0-9-]{0,62})$'),
 source text NOT NULL CHECK(length(source) BETWEEN 1 AND 4096),
 destination text NOT NULL CHECK(length(destination) BETWEEN 1 AND 4096),
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
 completed bigint NOT NULL DEFAULT 0 CHECK(completed>=0),
 total bigint CHECK(total>=0),
 attempt integer NOT NULL DEFAULT 0 CHECK(attempt>=0),
 max_attempts integer NOT NULL DEFAULT 3 CHECK(max_attempts BETWEEN 1 AND 10),
 cancel_requested boolean NOT NULL DEFAULT false,
 lease_until timestamptz,
 fencing bigint NOT NULL DEFAULT 0,
 error_code text CHECK(error_code ~ '^[A-Z][A-Z0-9_]*$'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 CHECK((state IN ('succeeded','failed','cancelled'))=(finished_at IS NOT NULL))
);
CREATE INDEX files_jobs_pending ON files_jobs(created_at) WHERE state IN ('queued','running');
CREATE INDEX files_jobs_user ON files_jobs(user_id,created_at DESC);
CREATE INDEX files_jobs_finished ON files_jobs(finished_at) WHERE finished_at IS NOT NULL;
