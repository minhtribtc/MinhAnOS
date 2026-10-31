-- Favorites (Files roadmap phase 1): items a user pinned in the Files sidebar. Root is personal, share:<key> or
-- system; path is the logical path inside it. Personal: removed with the account (RemoveDeletedUsers).
CREATE TABLE files_favorites(
 user_id text NOT NULL,
 root text NOT NULL CHECK (length(root) BETWEEN 1 AND 200),
 path text NOT NULL CHECK (length(path) BETWEEN 1 AND 4096),
 dir boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id, root, path)
);
