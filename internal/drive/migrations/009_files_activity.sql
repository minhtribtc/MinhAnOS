-- Files activity log levels (ADR 0012). The System Admin picks how much Files records:
--   minimal  writes only, no path or name in homes and Shared Folders (the former and default behaviour)
--   writes   writes with paths and names (from → to)
--   access   also reads: folders opened, files opened/downloaded, ZIPs, searches; denied and failed attempts
--   full     also client address, browser, duration and bytes, and who has what open right now (in memory)
CREATE TABLE files_settings(
 id int PRIMARY KEY DEFAULT 1 CHECK(id=1),
 audit_level text NOT NULL DEFAULT 'minimal' CHECK(audit_level IN ('minimal','writes','access','full')),
 version int NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid,
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO files_settings(id) VALUES(1);

-- Events now carry their outcome (Log Center: success, denied, failure); an anonymous visitor of a public link has no
-- actor. Paths may be recorded in every root when the level asks for it, so the system-root-only rule is dropped.
ALTER TABLE files_events ADD COLUMN outcome text NOT NULL DEFAULT 'success' CHECK(outcome IN ('success','denied','failure'));
DO $$
DECLARE c record;
BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='files_events'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%path%' LOOP
  EXECUTE format('ALTER TABLE files_events DROP CONSTRAINT %I', c.conname);
 END LOOP;
END $$;
CREATE INDEX files_events_actor ON files_events(actor_id,occurred_at DESC,id DESC) WHERE actor_id IS NOT NULL;
