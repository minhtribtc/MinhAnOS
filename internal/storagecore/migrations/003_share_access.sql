-- Storage Q2: share permissions for Files, one share hosting user homes.
-- At most one share hosts homes: the legacy directory share of upgrades or one created on ZFS/Btrfs.
CREATE UNIQUE INDEX storage_shares_one_homes ON storage_shares(kind) WHERE kind='homes';
-- Who may use a Shared Folder in Files. Deny wins over write, write over read; no entry means no access.
-- Subjects are identity public IDs (users.public_id or identity_groups.id), checked when the list is saved.
CREATE TABLE storage_share_access(
 share_id uuid NOT NULL REFERENCES storage_shares(id) ON DELETE CASCADE,
 subject_type text NOT NULL CHECK(subject_type IN ('user','group')),
 subject_id uuid NOT NULL,
 access text NOT NULL CHECK(access IN ('read','write','deny')),
 PRIMARY KEY(share_id,subject_type,subject_id)
);
CREATE INDEX storage_share_access_subject ON storage_share_access(subject_type,subject_id);
