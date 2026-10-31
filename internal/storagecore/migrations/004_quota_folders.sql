-- Storage Q3: quota folders inside Shared Folders (docs/storage-q3.md). A quota folder is a registered path
-- with a stable ID; Files rewrites the path when the folder (or a parent) is renamed or moved. Its ledger is a
-- storage_quota row of scope 'folder'; every write must fit the folder, any enclosing quota folder and the share.
CREATE TABLE storage_quota_folders(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 share_id uuid NOT NULL REFERENCES storage_shares(id) ON DELETE CASCADE,
 path text NOT NULL CHECK(length(path) BETWEEN 1 AND 4096 AND path !~ '(^|/)\.{0,2}(/|$)'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(share_id,path)
);
ALTER TABLE storage_quota DROP CONSTRAINT storage_quota_scope_type_check;
ALTER TABLE storage_quota ADD CONSTRAINT storage_quota_scope_type_check CHECK(scope_type IN ('share','home','folder'));
-- Folder scopes a reservation also holds (empty for homes and folder-less writes).
ALTER TABLE storage_reservations ADD COLUMN folder_ids text[] NOT NULL DEFAULT '{}';
