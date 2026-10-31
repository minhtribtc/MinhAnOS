-- Sharing (ADR 0011). Grants share an item of the owner's personal home with another user of a common organization
-- (view or edit); links share it with anyone holding the URL (view, edit = view + upload/new folder, drop = upload
-- only), optionally with a password (hash) and an expiry. Paths are logical paths in the owner's files and follow
-- renames and moves; trashing the item ends its shares. Personal: removed with the owner or the recipient.
CREATE TABLE files_grants(
 id text PRIMARY KEY,
 owner_id text NOT NULL,
 path text NOT NULL CHECK (length(path) BETWEEN 1 AND 4096),
 dir boolean NOT NULL,
 grantee_id text NOT NULL,
 permission text NOT NULL CHECK (permission IN ('view','edit')),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_id, path, grantee_id),
 CHECK (owner_id <> grantee_id)
);
CREATE INDEX files_grants_grantee ON files_grants(grantee_id);
CREATE TABLE files_links(
 id text PRIMARY KEY,
 token text NOT NULL UNIQUE CHECK (length(token) >= 32),
 owner_id text NOT NULL,
 path text NOT NULL CHECK (length(path) BETWEEN 1 AND 4096),
 dir boolean NOT NULL,
 permission text NOT NULL CHECK (permission IN ('view','edit','drop')),
 password_hash text,
 expires_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 last_used_at timestamptz,
 uses bigint NOT NULL DEFAULT 0,
 CHECK (dir OR permission = 'view')
);
CREATE INDEX files_links_owner ON files_links(owner_id, path);
