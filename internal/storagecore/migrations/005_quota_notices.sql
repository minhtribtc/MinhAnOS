-- Quota threshold notices (app standard row 11). A trigger on the ledger compares each scope's usage with
-- its limit after every change (writes, measurements, deletes, new limits) and queues one notice when the
-- scope reaches a higher level (80, 95 or 100 %) than the last one notified. Falling below a level lowers
-- the stored level silently, so crossing it again notifies again. The outbox row is written in the same
-- transaction as the ledger change; the Notification Center worker moves it into the inbox.
-- Recipients: a home notifies its owner (source Files); a Shared Folder or quota folder notifies the
-- active System Admins (source Storage Manager). Params carry numbers and names only, never file paths of
-- a personal home. Retention: delivered outbox rows are removed after 30 days by the worker.
CREATE TABLE storage_quota_alerts(
 scope_type text NOT NULL,
 scope_id text NOT NULL,
 level smallint NOT NULL CHECK(level IN (0,80,95,100)),
 PRIMARY KEY(scope_type,scope_id)
);
CREATE TABLE storage_notification_outbox(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 recipient_id uuid NOT NULL,
 source text NOT NULL CHECK(source IN ('com.minhanos.files','com.minhanos.storage')),
 kind text NOT NULL CHECK(kind IN ('quota.warning','quota.critical','quota.full')),
 severity text NOT NULL CHECK(severity IN ('warning','critical')),
 details jsonb NOT NULL CHECK(jsonb_typeof(details)='object'),
 request_id text NOT NULL,
 correlation_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 delivered_at timestamptz
);
CREATE INDEX storage_notification_pending ON storage_notification_outbox(created_at) WHERE delivered_at IS NULL;
CREATE INDEX storage_notification_delivered ON storage_notification_outbox(delivered_at) WHERE delivered_at IS NOT NULL;

CREATE FUNCTION storage_quota_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 lvl smallint := 0;
 prev smallint;
 pct int;
 kind text;
 params jsonb;
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
  params := params || (SELECT jsonb_build_object('share', s.name) FROM storage_shares s WHERE s.id::text = NEW.scope_id);
 ELSE
  params := params || (SELECT jsonb_build_object('share', s.name, 'folder', f.path) FROM storage_quota_folders f JOIN storage_shares s ON s.id = f.share_id WHERE f.id::text = NEW.scope_id);
 END IF;
 INSERT INTO storage_notification_outbox(recipient_id, source, kind, severity, details, request_id, correlation_id)
  SELECT u.public_id, 'com.minhanos.storage', kind, CASE WHEN lvl = 100 THEN 'critical' ELSE 'warning' END, coalesce(params, '{}'), req, corr
  FROM users u WHERE u.role = 'admin' AND u.status = 'active';
 RETURN NEW;
END $$;

CREATE TRIGGER storage_quota_notice AFTER INSERT OR UPDATE OF used_bytes, limit_bytes ON storage_quota
 FOR EACH ROW EXECUTE FUNCTION storage_quota_notice();
