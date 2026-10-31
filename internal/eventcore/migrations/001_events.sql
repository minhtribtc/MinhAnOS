-- Platform event outbox (ADR 0010, contracts/events/events.v1.yaml). Events are written in the transaction of the
-- domain change; each named consumer receives every matching event at least once, recorded in deliveries in the same
-- transaction as its side effect (dedupe by consumer + event). Failed deliveries back off and become dead after the
-- attempt limit. A consumer starts from the latest event at its first registration. Events are kept 30 days.
CREATE TABLE platform_events(
 seq bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
 name text NOT NULL CHECK(name ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+\.v[1-9][0-9]*$' AND length(name)<=120),
 schema_version integer NOT NULL CHECK(schema_version>0),
 source text NOT NULL CHECK(length(source) BETWEEN 1 AND 64),
 subject text NOT NULL CHECK(length(subject) BETWEEN 1 AND 200),
 organization_id uuid,
 correlation_id text NOT NULL,
 data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX platform_events_name ON platform_events(name,seq);
CREATE INDEX platform_events_occurred ON platform_events(occurred_at);

CREATE TABLE platform_event_consumers(
 consumer text PRIMARY KEY CHECK(consumer ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)*$'),
 names text[] NOT NULL,
 since_seq bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE platform_event_deliveries(
 consumer text NOT NULL REFERENCES platform_event_consumers(consumer) ON DELETE CASCADE,
 event_seq bigint NOT NULL REFERENCES platform_events(seq) ON DELETE CASCADE,
 state text NOT NULL CHECK(state IN ('done','failed','dead')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 next_attempt_at timestamptz,
 last_error text CHECK(length(last_error)<=500),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(consumer,event_seq)
);
CREATE INDEX platform_event_deliveries_seq ON platform_event_deliveries(event_seq);
CREATE INDEX platform_event_deliveries_retry ON platform_event_deliveries(consumer,next_attempt_at) WHERE state='failed';
