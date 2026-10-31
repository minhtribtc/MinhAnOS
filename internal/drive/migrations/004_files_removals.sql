-- User data removal (app standard row 17, contracts/data/files.data.json). Account Center deletes are final;
-- Drive records when it first saw a deleted account, drops its jobs and replay rows at once and removes its
-- home (files, trash, job staging) after a 30-day grace period, so an admin can still rescue data from a
-- backup or the disk. Audit rows (files_events) keep their own 365-day retention and hold no personal paths.
CREATE TABLE files_removals(
 user_id text PRIMARY KEY,
 seen_at timestamptz NOT NULL DEFAULT now(),
 removed_at timestamptz
);
CREATE INDEX files_removals_pending ON files_removals(seen_at) WHERE removed_at IS NULL;
