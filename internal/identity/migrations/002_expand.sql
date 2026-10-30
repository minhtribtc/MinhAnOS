-- Add nullable UUID columns first: no volatile-default backfill/table rewrite at startup.
ALTER TABLE users ADD COLUMN public_id uuid;
ALTER TABLE users ALTER COLUMN public_id SET DEFAULT gen_random_uuid();
ALTER TABLE users ADD COLUMN password_credential_id uuid;
ALTER TABLE users ALTER COLUMN password_credential_id SET DEFAULT gen_random_uuid();
ALTER TABLE users ADD COLUMN status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled','deleted'));
ALTER TABLE users ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE users ADD COLUMN auth_epoch bigint NOT NULL DEFAULT 1 CHECK(auth_epoch>0);
ALTER TABLE users ADD COLUMN password_scheme text NOT NULL DEFAULT 'pbkdf2-sha256-600000-v1';
ALTER TABLE users ADD COLUMN password_changed_at timestamptz;
ALTER TABLE users ALTER COLUMN password_changed_at SET DEFAULT now();
CREATE UNIQUE INDEX identity_users_public_id_uq ON users(public_id);
CREATE UNIQUE INDEX identity_users_password_credential_uq ON users(password_credential_id);
CREATE INDEX identity_users_uuid_pending_idx ON users(id) WHERE public_id IS NULL OR password_credential_id IS NULL;
CREATE FUNCTION identity_preserve_user_keys() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id THEN RAISE EXCEPTION 'legacy identity/storage key is immutable'; END IF;
 IF OLD.public_id IS NOT NULL AND NEW.public_id IS DISTINCT FROM OLD.public_id THEN RAISE EXCEPTION 'public user identity is immutable'; END IF;
 IF OLD.password_credential_id IS NOT NULL AND NEW.password_credential_id IS DISTINCT FROM OLD.password_credential_id THEN RAISE EXCEPTION 'password credential reference is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER identity_immutable_user_keys BEFORE UPDATE OF id,public_id,password_credential_id ON users FOR EACH ROW EXECUTE FUNCTION identity_preserve_user_keys();
