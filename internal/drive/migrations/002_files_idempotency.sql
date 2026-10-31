-- Files API writes need an Idempotency-Key (app standard row 15). The response of a successful write is
-- kept for 24 hours so a retried request (same key, same request) gets the same answer instead of running
-- twice; the same key with a different request is refused. Only hashes of the key and request are stored,
-- never paths or file names. Expired rows are removed by the Drive retention loop.
CREATE TABLE files_idempotency(
 user_id text NOT NULL,
 key_hash text NOT NULL CHECK(key_hash ~ '^[0-9a-f]{64}$'),
 request_hash text NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 status integer NOT NULL CHECK(status BETWEEN 200 AND 299),
 response jsonb NOT NULL,
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(user_id,key_hash)
);
CREATE INDEX files_idempotency_expiry ON files_idempotency(expires_at);
