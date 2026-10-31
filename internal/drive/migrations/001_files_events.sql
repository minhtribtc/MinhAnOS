-- Files audit (app standard rows 10, 12, 17): structured, append-only, request/correlation IDs.
-- Personal homes record no path or file name; Shared Folders record the share key only; the host
-- system root (System Admin with host operator) records the path. The legacy drive_audit table is
-- kept read-only for history and no longer written.
CREATE TABLE files_events(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 actor_id uuid,
 action text NOT NULL CHECK(action ~ '^files\.[a-z_]+$'),
 root text NOT NULL CHECK(root IN ('personal','share','system')),
 share_key text,
 request_id text NOT NULL,
 correlation_id text,
 details jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(details)='object'),
 occurred_at timestamptz NOT NULL DEFAULT now(),
 CHECK((root='share')=(share_key IS NOT NULL)),
 CHECK(root='system' OR NOT details ? 'path')
);
CREATE INDEX files_events_recent ON files_events(occurred_at DESC,id DESC);
-- Deletes only by the retention routine (365 days), like identity_admin_events.
CREATE FUNCTION files_events_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND current_setting('minhanos.audit_retention',true)='on' AND OLD.occurred_at<now()-interval '365 days' THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'files audit is append-only';
END $$;
CREATE TRIGGER files_events_append_only BEFORE UPDATE OR DELETE ON files_events FOR EACH ROW EXECUTE FUNCTION files_events_guard();
