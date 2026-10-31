-- Storage allocations and organization-owned Shared Folders (ADR 0008). Runs after identity SQL 012.
-- An allocation is an organization's budget on one pool. A child's limit is carved out of its parent's
-- direct budget (limit - delegated). The ledger row (scope 'allocation') holds that direct budget and the
-- used/reserved bytes of the organization's own shares on the pool, which a trigger rolls up from the share
-- rows. A reservation that would pass the budget fails with STORAGE_ALLOCATION_EXCEEDED.
CREATE TABLE storage_allocations(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES identity_organizations(id) ON DELETE RESTRICT,
 volume text NOT NULL REFERENCES storage_volumes(name),
 parent_id uuid REFERENCES storage_allocations(id) ON DELETE RESTRICT,
 limit_bytes bigint CHECK(limit_bytes IS NULL OR limit_bytes>=0),
 delegated_bytes bigint NOT NULL DEFAULT 0 CHECK(delegated_bytes>=0),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,volume),
 UNIQUE(id,organization_id,volume),
 CHECK(parent_id IS NULL OR limit_bytes IS NOT NULL),
 CHECK(limit_bytes IS NULL OR delegated_bytes<=limit_bytes)
);
CREATE INDEX storage_allocations_parent ON storage_allocations(parent_id);

ALTER TABLE storage_shares ADD COLUMN organization_id uuid REFERENCES identity_organizations(id) ON DELETE RESTRICT;
ALTER TABLE storage_shares ADD COLUMN allocation_id uuid;
ALTER TABLE storage_quota DROP CONSTRAINT storage_quota_scope_type_check;
ALTER TABLE storage_quota ADD CONSTRAINT storage_quota_scope_type_check CHECK(scope_type IN ('share','home','folder','allocation'));

-- Adoption: the default organization gets an unlimited allocation on every ZFS/Btrfs pool (and on pools
-- that already host shares) and owns every ordinary share.
INSERT INTO storage_allocations(organization_id,volume)
 SELECT o.id,v.name FROM identity_organizations o CROSS JOIN storage_volumes v
 WHERE o.is_default AND (v.kind<>'directory' OR EXISTS(SELECT 1 FROM storage_shares s WHERE s.volume=v.name AND s.kind='share'));
UPDATE storage_shares s SET organization_id=a.organization_id,allocation_id=a.id
 FROM storage_allocations a JOIN identity_organizations o ON o.id=a.organization_id AND o.is_default
 WHERE s.kind='share' AND a.volume=s.volume;
INSERT INTO storage_quota(scope_type,scope_id,limit_bytes,used_bytes,reserved_bytes,measure_requested,measured_at)
 SELECT 'allocation',a.id::text,NULL,
  coalesce((SELECT sum(q.used_bytes) FROM storage_shares s JOIN storage_quota q ON q.scope_type='share' AND q.scope_id=s.id::text WHERE s.allocation_id=a.id),0),
  coalesce((SELECT sum(q.reserved_bytes) FROM storage_shares s JOIN storage_quota q ON q.scope_type='share' AND q.scope_id=s.id::text WHERE s.allocation_id=a.id),0),
  false,now()
 FROM storage_allocations a;
ALTER TABLE storage_shares ADD CONSTRAINT storage_shares_allocation FOREIGN KEY(allocation_id,organization_id,volume) REFERENCES storage_allocations(id,organization_id,volume) ON DELETE RESTRICT;
ALTER TABLE storage_shares ADD CONSTRAINT storage_shares_owner CHECK((kind='homes' AND organization_id IS NULL AND allocation_id IS NULL) OR (kind='share' AND organization_id IS NOT NULL AND allocation_id IS NOT NULL));
CREATE INDEX storage_shares_organization ON storage_shares(organization_id);

CREATE FUNCTION storage_allocation_rollup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 alloc uuid;
 du bigint := NEW.used_bytes-OLD.used_bytes;
 dr bigint := NEW.reserved_bytes-OLD.reserved_bytes;
 fits boolean;
BEGIN
 IF du=0 AND dr=0 THEN RETURN NEW; END IF;
 SELECT allocation_id INTO alloc FROM storage_shares WHERE id::text=NEW.scope_id;
 IF alloc IS NULL THEN RETURN NEW; END IF;
 UPDATE storage_quota SET used_bytes=greatest(0,used_bytes+du),reserved_bytes=greatest(0,reserved_bytes+dr)
  WHERE scope_type='allocation' AND scope_id=alloc::text
  RETURNING dr<=0 OR limit_bytes IS NULL OR used_bytes+reserved_bytes<=limit_bytes INTO fits;
 IF fits IS FALSE THEN
  RAISE EXCEPTION 'STORAGE_ALLOCATION_EXCEEDED';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER storage_allocation_rollup AFTER UPDATE OF used_bytes,reserved_bytes ON storage_quota
 FOR EACH ROW WHEN (NEW.scope_type='share') EXECUTE FUNCTION storage_allocation_rollup();

-- Leaving an organization removes the account from its shares' permission lists (identity SQL 012 removes
-- it from the organization's groups).
CREATE FUNCTION storage_membership_left() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 DELETE FROM storage_share_access WHERE subject_type='user' AND subject_id=OLD.user_id
  AND share_id IN (SELECT id FROM storage_shares WHERE organization_id=OLD.organization_id);
 RETURN OLD;
END $$;
CREATE TRIGGER storage_membership_left AFTER DELETE ON identity_memberships FOR EACH ROW EXECUTE FUNCTION storage_membership_left();

-- Audit rows carry the organization of their target (share, quota folder or allocation).
ALTER TABLE storage_events ADD COLUMN organization_id uuid;
CREATE INDEX storage_events_organization ON storage_events(organization_id,occurred_at DESC,id DESC) WHERE organization_id IS NOT NULL;
CREATE FUNCTION storage_events_organization() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.organization_id IS NULL AND NEW.target IS NOT NULL THEN
  NEW.organization_id := coalesce(
   (SELECT organization_id FROM storage_shares WHERE id::text=NEW.target),
   (SELECT s.organization_id FROM storage_quota_folders f JOIN storage_shares s ON s.id=f.share_id WHERE f.id::text=NEW.target),
   (SELECT organization_id FROM storage_allocations WHERE id::text=NEW.target));
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER storage_events_organization BEFORE INSERT ON storage_events FOR EACH ROW EXECUTE FUNCTION storage_events_organization();

-- Quota notices: shares, quota folders and allocations notify the direct admins of the owning organization;
-- the homes share keeps notifying the System Admins.
ALTER TABLE storage_notification_outbox ADD COLUMN organization_id uuid;
CREATE OR REPLACE FUNCTION storage_quota_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 lvl smallint := 0;
 prev smallint;
 pct int;
 kind text;
 params jsonb;
 org uuid;
 req text := coalesce(nullif(current_setting('minhanos.request_id', true), ''), 'storage-ledger');
 corr text := nullif(current_setting('minhanos.correlation_id', true), '');
BEGIN
 IF NEW.limit_bytes IS NOT NULL AND NEW.limit_bytes > 0 THEN
  pct := least(100, NEW.used_bytes * 100 / NEW.limit_bytes);
  lvl := CASE WHEN NEW.used_bytes >= NEW.limit_bytes THEN 100 WHEN pct >= 95 THEN 95 WHEN pct >= 80 THEN 80 ELSE 0 END;
 END IF;
 SELECT level INTO prev FROM storage_quota_alerts WHERE scope_type = NEW.scope_type AND scope_id = NEW.scope_id FOR UPDATE;
 IF NOT FOUND THEN
  INSERT INTO storage_quota_alerts(scope_type, scope_id, level) VALUES(NEW.scope_type, NEW.scope_id, lvl);
  prev := 0;
 ELSIF prev <> lvl THEN
  UPDATE storage_quota_alerts SET level = lvl WHERE scope_type = NEW.scope_type AND scope_id = NEW.scope_id;
 END IF;
 IF lvl <= prev THEN
  RETURN NEW;
 END IF;
 kind := CASE lvl WHEN 100 THEN 'quota.full' WHEN 95 THEN 'quota.critical' ELSE 'quota.warning' END;
 params := jsonb_build_object('scope', NEW.scope_type, 'percent', pct, 'usedBytes', NEW.used_bytes, 'limitBytes', NEW.limit_bytes);
 IF NEW.scope_type = 'home' THEN
  INSERT INTO storage_notification_outbox(recipient_id, source, kind, severity, details, request_id, correlation_id)
   SELECT u.public_id, 'com.minhanos.files', kind, CASE WHEN lvl = 100 THEN 'critical' ELSE 'warning' END, params, req, corr
   FROM users u WHERE u.id = NEW.scope_id AND u.status = 'active';
  RETURN NEW;
 END IF;
 IF NEW.scope_type = 'share' THEN
  SELECT jsonb_build_object('share', s.name), s.organization_id INTO params, org FROM storage_shares s WHERE s.id::text = NEW.scope_id;
  params := jsonb_build_object('scope', NEW.scope_type, 'percent', pct, 'usedBytes', NEW.used_bytes, 'limitBytes', NEW.limit_bytes) || coalesce(params, '{}');
 ELSIF NEW.scope_type = 'folder' THEN
  SELECT jsonb_build_object('share', s.name, 'folder', f.path), s.organization_id INTO params, org FROM storage_quota_folders f JOIN storage_shares s ON s.id = f.share_id WHERE f.id::text = NEW.scope_id;
  params := jsonb_build_object('scope', NEW.scope_type, 'percent', pct, 'usedBytes', NEW.used_bytes, 'limitBytes', NEW.limit_bytes) || coalesce(params, '{}');
 ELSE
  SELECT jsonb_build_object('organization', o.name, 'volume', a.volume), a.organization_id INTO params, org FROM storage_allocations a JOIN identity_organizations o ON o.id = a.organization_id WHERE a.id::text = NEW.scope_id;
  params := jsonb_build_object('scope', NEW.scope_type, 'percent', pct, 'usedBytes', NEW.used_bytes, 'limitBytes', NEW.limit_bytes) || coalesce(params, '{}');
 END IF;
 IF org IS NULL THEN
  INSERT INTO storage_notification_outbox(recipient_id, source, kind, severity, details, request_id, correlation_id)
   SELECT u.public_id, 'com.minhanos.storage', kind, CASE WHEN lvl = 100 THEN 'critical' ELSE 'warning' END, params, req, corr
   FROM users u WHERE u.role = 'admin' AND u.status = 'active';
 ELSE
  INSERT INTO storage_notification_outbox(recipient_id, source, kind, severity, details, request_id, correlation_id, organization_id)
   SELECT u.public_id, 'com.minhanos.storage', kind, CASE WHEN lvl = 100 THEN 'critical' ELSE 'warning' END, params, req, corr, org
   FROM identity_memberships m JOIN users u ON u.public_id = m.user_id WHERE m.organization_id = org AND m.role = 'admin' AND u.status = 'active';
 END IF;
 RETURN NEW;
END $$;

-- A newly registered ZFS/Btrfs pool starts with an unlimited allocation for the default organization, so a
-- single NAS needs no allocation step; Root may lower or remove it.
CREATE FUNCTION storage_volume_default_allocation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE alloc uuid;
BEGIN
 IF NEW.kind<>'directory' THEN
  INSERT INTO storage_allocations(organization_id,volume) SELECT id,NEW.name FROM identity_organizations WHERE is_default
   ON CONFLICT DO NOTHING RETURNING id INTO alloc;
  IF alloc IS NOT NULL THEN
   INSERT INTO storage_quota(scope_type,scope_id,measure_requested,measured_at) VALUES('allocation',alloc::text,false,now());
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER storage_volume_default_allocation AFTER INSERT ON storage_volumes FOR EACH ROW EXECUTE FUNCTION storage_volume_default_allocation();
