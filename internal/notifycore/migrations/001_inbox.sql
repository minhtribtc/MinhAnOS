-- Notification Center inbox (minimal v1). Rows are copied from module outboxes by the delivery worker;
-- the id is the outbox row id, so a repeated delivery never duplicates a notice. Texts are not stored:
-- the UI renders i18n keys from source + kind with params. Retention: 90 days.
CREATE TABLE notifications(
 id uuid PRIMARY KEY,
 recipient_id uuid NOT NULL,
 source text NOT NULL CHECK(source ~ '^com\.minhanos\.[a-z][a-z-]*$'),
 kind text NOT NULL CHECK(kind ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$'),
 severity text NOT NULL CHECK(severity IN ('info','warning','critical')),
 mandatory boolean NOT NULL DEFAULT false,
 params jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(params)='object'),
 request_id text NOT NULL,
 correlation_id text,
 created_at timestamptz NOT NULL,
 read_at timestamptz
);
CREATE INDEX notifications_inbox ON notifications(recipient_id,created_at DESC,id DESC);
CREATE INDEX notifications_unread ON notifications(recipient_id) WHERE read_at IS NULL;
CREATE INDEX notifications_age ON notifications(created_at);
