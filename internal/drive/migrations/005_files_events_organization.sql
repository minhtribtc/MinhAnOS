-- Files audit carries the organization of the Shared Folder (ADR 0008), so organization admins see the
-- Files events of their organization in the Log Center. Personal and system roots have none.
ALTER TABLE files_events ADD COLUMN organization_id uuid;
CREATE INDEX files_events_organization ON files_events(organization_id,occurred_at DESC,id DESC) WHERE organization_id IS NOT NULL;
-- Backfill once inside the migration; the append-only guard is paused for this statement only.
ALTER TABLE files_events DISABLE TRIGGER files_events_append_only;
UPDATE files_events f SET organization_id=s.organization_id FROM storage_shares s WHERE f.root='share' AND s.key=f.share_key;
ALTER TABLE files_events ENABLE TRIGGER files_events_append_only;
