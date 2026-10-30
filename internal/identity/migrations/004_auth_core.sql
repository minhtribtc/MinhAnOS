-- Two canonical realms, including when the other host is installed later.
CREATE TABLE IF NOT EXISTS sessions(token_hash text PRIMARY KEY,user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,csrf_token text NOT NULL,expires_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS drive_sessions(token_hash text PRIMARY KEY,user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,csrf_token text NOT NULL,expires_at timestamptz NOT NULL);
ALTER TABLE sessions ADD COLUMN public_id uuid NOT NULL DEFAULT gen_random_uuid(), ADD COLUMN auth_epoch bigint NOT NULL DEFAULT 1 CHECK(auth_epoch>0), ADD COLUMN created_at timestamptz;
ALTER TABLE drive_sessions ADD COLUMN public_id uuid NOT NULL DEFAULT gen_random_uuid(), ADD COLUMN auth_epoch bigint NOT NULL DEFAULT 1 CHECK(auth_epoch>0), ADD COLUMN created_at timestamptz;
ALTER TABLE sessions ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE drive_sessions ALTER COLUMN created_at SET DEFAULT now();
CREATE UNIQUE INDEX identity_sessions_public_id ON sessions(public_id);
CREATE UNIQUE INDEX identity_drive_sessions_public_id ON drive_sessions(public_id);
CREATE INDEX identity_sessions_user ON sessions(user_id);
CREATE INDEX identity_drive_sessions_user ON drive_sessions(user_id);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS drive_sessions_expiry ON drive_sessions(expires_at);
ALTER TABLE users ADD COLUMN recovery_token_hash text, ADD COLUMN recovery_expires_at timestamptz, ADD COLUMN recovery_auth_epoch bigint;
ALTER TABLE users ADD CONSTRAINT identity_recovery_shape CHECK ((recovery_token_hash IS NULL AND recovery_expires_at IS NULL AND recovery_auth_epoch IS NULL) OR (recovery_token_hash ~ '^[0-9a-f]{64}$' AND recovery_expires_at IS NOT NULL AND recovery_auth_epoch IS NOT NULL));
-- No passwords, tokens, usernames, IPs or arbitrary detail payloads in this audit table.
CREATE TABLE identity_auth_events(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id text REFERENCES users(id) ON DELETE SET NULL,action text NOT NULL CHECK(action IN ('password.changed','recovery.issued','recovery.consumed','sessions.revoked')),created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX identity_auth_events_user ON identity_auth_events(user_id,created_at);
CREATE TABLE identity_auth_attempts(key_hash text PRIMARY KEY CHECK(key_hash ~ '^[0-9a-f]{64}$'),attempts integer NOT NULL CHECK(attempts>0),expires_at timestamptz NOT NULL);
CREATE INDEX identity_auth_attempts_expiry ON identity_auth_attempts(expires_at);
-- A legacy credential/status writer must also invalidate already issued sessions.
CREATE FUNCTION identity_auth_state_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.auth_epoch < OLD.auth_epoch THEN RAISE EXCEPTION 'auth epoch cannot decrease'; END IF;
 IF NEW.password_hash IS DISTINCT FROM OLD.password_hash OR NEW.status IS DISTINCT FROM OLD.status THEN
  NEW.auth_epoch := greatest(NEW.auth_epoch,OLD.auth_epoch+1);
  NEW.version := greatest(NEW.version,OLD.version+1);
  NEW.recovery_token_hash := NULL; NEW.recovery_expires_at := NULL; NEW.recovery_auth_epoch := NULL;
  IF NEW.password_hash IS DISTINCT FROM OLD.password_hash THEN NEW.password_changed_at := now(); END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER identity_auth_state_guard BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION identity_auth_state_guard();
