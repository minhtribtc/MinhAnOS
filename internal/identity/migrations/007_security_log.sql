-- Phase A: one structured security log for identity, correlation IDs, append-only guard and a
-- transactional outbox of security notices for the future Notification Center.
ALTER TABLE identity_admin_events ADD COLUMN correlation_id text;
-- Move the separate AuthCore ledger into the structured store once. Rows already recorded by the
-- policy path (same action and user within 5 seconds) are not duplicated.
INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,occurred_at)
 SELECT u.public_id,e.action,u.public_id::text,'success','migrated:'||e.id::text,e.created_at
 FROM identity_auth_events e LEFT JOIN users u ON u.id=e.user_id
 WHERE NOT EXISTS(SELECT 1 FROM identity_admin_events a WHERE a.action=e.action AND (a.target_id=u.public_id::text OR a.actor_id=u.public_id) AND abs(extract(epoch FROM a.occurred_at-e.created_at))<5);
-- The old ledger stays for history but must not receive new rows.
CREATE FUNCTION identity_auth_events_retired() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'identity_auth_events is retired; write identity_admin_events'; END $$;
CREATE TRIGGER identity_auth_events_retired BEFORE INSERT ON identity_auth_events FOR EACH ROW EXECUTE FUNCTION identity_auth_events_retired();
-- Append-only: no updates; deletes only by the retention routine and only for rows older than 365 days.
-- This stops application bugs, not a database superuser (documented trust boundary).
CREATE FUNCTION identity_admin_events_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND current_setting('minhanos.audit_retention',true)='on' AND OLD.occurred_at<now()-interval '365 days' THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'identity audit is append-only';
END $$;
CREATE TRIGGER identity_admin_events_append_only BEFORE UPDATE OR DELETE ON identity_admin_events FOR EACH ROW EXECUTE FUNCTION identity_admin_events_guard();
-- Security notices written in the same transaction as the change; a Notification Center consumes them later.
CREATE TABLE identity_notification_outbox(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 recipient_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('password.changed','password.reset','role.changed','sessions.revoked','login.failures','recovery.consumed')),
 severity text NOT NULL CHECK(severity IN ('info','warning','critical')),
 mandatory boolean NOT NULL DEFAULT false,
 details jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(details)='object' AND details-'role'-'realm'-'count'='{}'::jsonb),
 request_id text NOT NULL,
 correlation_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 delivered_at timestamptz
);
CREATE INDEX identity_notification_outbox_pending ON identity_notification_outbox(created_at) WHERE delivered_at IS NULL;
CREATE INDEX identity_notification_outbox_recipient ON identity_notification_outbox(recipient_id,created_at DESC);
