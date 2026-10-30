-- Storage registry and application-level (soft) quota ledger. Enforcement uses usage + reservations in
-- transactions; filesystem walks only repair drift. Hard quota on ZFS/Btrfs is a later, optional layer.
CREATE TABLE storage_volumes(
 name text PRIMARY KEY CHECK(name ~ '^[a-z][a-z0-9-]{0,31}$'),
 kind text NOT NULL CHECK(kind IN ('zfs','btrfs','directory')),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE storage_shares(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 key text NOT NULL UNIQUE CHECK(key ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 volume text NOT NULL REFERENCES storage_volumes(name),
 kind text NOT NULL CHECK(kind IN ('share','homes')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now()
);
-- One ledger row per scope: a share (total) or a home (user storage key). NULL limit = unlimited.
CREATE TABLE storage_quota(
 scope_type text NOT NULL CHECK(scope_type IN ('share','home')),
 scope_id text NOT NULL,
 limit_bytes bigint CHECK(limit_bytes IS NULL OR limit_bytes>=0),
 used_bytes bigint NOT NULL DEFAULT 0 CHECK(used_bytes>=0),
 reserved_bytes bigint NOT NULL DEFAULT 0 CHECK(reserved_bytes>=0),
 measure_requested boolean NOT NULL DEFAULT true,
 measured_at timestamptz,
 version bigint NOT NULL DEFAULT 1,
 PRIMARY KEY(scope_type,scope_id)
);
-- In-flight writes. Expired rows are released by the next reservation, so a crash cannot leak space forever.
CREATE TABLE storage_reservations(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 home_id text NOT NULL,
 share_id text NOT NULL,
 bytes bigint NOT NULL CHECK(bytes>=0),
 expires_at timestamptz NOT NULL
);
CREATE INDEX storage_reservations_expiry ON storage_reservations(expires_at);
CREATE TABLE storage_settings(key text PRIMARY KEY CHECK(key IN ('default_home_quota_bytes')), value jsonb NOT NULL);
-- Append-only audit of storage administration (read later by the Log Center).
CREATE TABLE storage_events(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 actor_id uuid,
 action text NOT NULL,
 target text,
 request_id text NOT NULL,
 correlation_id text,
 details jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(details)='object'),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX storage_events_recent ON storage_events(occurred_at DESC);
CREATE FUNCTION storage_events_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'storage audit is append-only'; END $$;
CREATE TRIGGER storage_events_append_only BEFORE UPDATE OR DELETE ON storage_events FOR EACH ROW EXECUTE FUNCTION storage_events_guard();
-- Existing private files stay where they are: Drive's data directory is registered as a plain volume and its
-- per-user areas form the default "homes" share. Moving homes to ZFS/Btrfs is a later explicit job.
INSERT INTO storage_volumes(name,kind) VALUES('files','directory');
INSERT INTO storage_shares(key,name,volume,kind) VALUES('homes','Homes','files','homes');
INSERT INTO storage_quota(scope_type,scope_id) SELECT 'share',id::text FROM storage_shares WHERE key='homes';
