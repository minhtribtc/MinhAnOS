-- Authentication audit in the existing structured identity store (no new audit table).
-- 'failure' records failed sign-ins of existing accounts; details may carry realm and attempt count.
ALTER TABLE identity_admin_events DROP CONSTRAINT identity_admin_events_outcome_check;
ALTER TABLE identity_admin_events ADD CONSTRAINT identity_admin_events_outcome_check CHECK(outcome IN ('success','denied','failure'));
ALTER TABLE identity_admin_events DROP CONSTRAINT identity_admin_events_details_check;
ALTER TABLE identity_admin_events ADD CONSTRAINT identity_admin_events_details_check CHECK(jsonb_typeof(details)='object' AND details-'reasonCode'-'changedFields'-'realm'-'count'='{}'::jsonb);
-- Newest-first listing and per-user activity on the user detail page.
CREATE INDEX identity_admin_events_recent ON identity_admin_events(occurred_at DESC,id DESC);
CREATE INDEX identity_admin_events_target ON identity_admin_events(target_id,occurred_at DESC);
CREATE INDEX identity_admin_events_actor ON identity_admin_events(actor_id,occurred_at DESC);
