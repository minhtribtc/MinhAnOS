-- Storage API v1 contract: If-Match versions on settings, idempotent POSTs, background measurement queue.
ALTER TABLE storage_settings ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
INSERT INTO storage_settings(key,value) VALUES('default_home_quota_bytes','null'::jsonb) ON CONFLICT DO NOTHING;
CREATE TABLE storage_idempotency(
 actor_id uuid NOT NULL,
 key_hash text NOT NULL CHECK(key_hash ~ '^[0-9a-f]{64}$'),
 request_hash text NOT NULL,
 status integer NOT NULL,
 response jsonb NOT NULL,
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,key_hash)
);
CREATE INDEX storage_idempotency_expiry ON storage_idempotency(expires_at);
-- storage_quota.version now changes only when the limit changes (If-Match for admins), not on every write.
CREATE INDEX storage_quota_measure ON storage_quota(scope_type) WHERE measure_requested;
