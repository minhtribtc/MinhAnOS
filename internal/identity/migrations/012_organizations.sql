-- Organizations and memberships (ADR 0008). Expand only: one default organization adopts every existing
-- account and user group, so a single NAS keeps working as one organization.
CREATE TABLE identity_organizations(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 1 AND 100),
 parent_id uuid REFERENCES identity_organizations(id) ON DELETE RESTRICT,
 -- Ancestors from the top-level organization down to this one (set by trigger, immutable).
 path uuid[] NOT NULL DEFAULT '{}',
 depth smallint NOT NULL DEFAULT 1 CHECK(depth BETWEEN 1 AND 8),
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended')),
 allow_children boolean NOT NULL DEFAULT false,
 is_default boolean NOT NULL DEFAULT false,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(NOT is_default OR parent_id IS NULL)
);
CREATE UNIQUE INDEX identity_organizations_default ON identity_organizations(is_default) WHERE is_default;
-- Sibling names are unique (case-insensitive); top-level names share one namespace.
CREATE UNIQUE INDEX identity_organizations_sibling_name ON identity_organizations(coalesce(parent_id,'00000000-0000-0000-0000-000000000000'::uuid),lower(btrim(name)));
CREATE INDEX identity_organizations_parent ON identity_organizations(parent_id);
CREATE INDEX identity_organizations_path ON identity_organizations USING gin(path);

CREATE FUNCTION identity_organizations_tree() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parent_path uuid[];
BEGIN
 IF TG_OP='UPDATE' THEN
  IF NEW.parent_id IS DISTINCT FROM OLD.parent_id OR NEW.path<>OLD.path OR NEW.depth<>OLD.depth OR NEW.id<>OLD.id OR NEW.is_default<>OLD.is_default THEN
   RAISE EXCEPTION 'organization position is immutable';
  END IF;
  RETURN NEW;
 END IF;
 IF NEW.parent_id IS NULL THEN
  NEW.path := ARRAY[NEW.id];
 ELSE
  SELECT path INTO parent_path FROM identity_organizations WHERE id=NEW.parent_id FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'parent organization missing'; END IF;
  NEW.path := parent_path||NEW.id;
 END IF;
 NEW.depth := cardinality(NEW.path);
 RETURN NEW;
END $$;
CREATE TRIGGER identity_organizations_tree BEFORE INSERT OR UPDATE ON identity_organizations FOR EACH ROW EXECUTE FUNCTION identity_organizations_tree();

CREATE TABLE identity_memberships(
 organization_id uuid NOT NULL REFERENCES identity_organizations(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(public_id) ON DELETE CASCADE,
 role text NOT NULL DEFAULT 'member' CHECK(role IN ('admin','member')),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,user_id)
);
CREATE INDEX identity_memberships_user ON identity_memberships(user_id);

-- Effectively active: the organization and every ancestor are active.
CREATE FUNCTION identity_org_active(p_org uuid) RETURNS boolean STABLE LANGUAGE sql AS $$
 SELECT EXISTS(SELECT 1 FROM identity_organizations o WHERE o.id=p_org)
  AND NOT EXISTS(SELECT 1 FROM identity_organizations o JOIN identity_organizations a ON a.id=ANY(o.path) WHERE o.id=p_org AND a.status<>'active')
$$;
-- Organization authority of one account (public ID): admin (direct, organization active), manager (admin of an
-- active ancestor that allows children), member (organization active) or NULL. System Admin is decided in Go.
CREATE FUNCTION identity_org_role(p_user uuid, p_org uuid) RETURNS text STABLE LANGUAGE sql AS $$
 SELECT CASE
  WHEN EXISTS(SELECT 1 FROM identity_memberships m WHERE m.organization_id=p_org AND m.user_id=p_user AND m.role='admin') AND identity_org_active(p_org) THEN 'admin'
  WHEN EXISTS(SELECT 1 FROM identity_organizations o JOIN identity_organizations a ON a.id=ANY(o.path) AND a.id<>o.id AND a.allow_children
    JOIN identity_memberships m ON m.organization_id=a.id AND m.user_id=p_user AND m.role='admin' WHERE o.id=p_org AND identity_org_active(a.id)) THEN 'manager'
  WHEN EXISTS(SELECT 1 FROM identity_memberships m WHERE m.organization_id=p_org AND m.user_id=p_user) AND identity_org_active(p_org) THEN 'member'
 END
$$;

-- Groups may belong to an organization; keys are unique per organization (platform groups: organization NULL).
ALTER TABLE identity_groups ADD COLUMN organization_id uuid REFERENCES identity_organizations(id) ON DELETE CASCADE;
ALTER TABLE identity_groups DROP CONSTRAINT identity_groups_key_key;
CREATE UNIQUE INDEX identity_groups_scoped_key ON identity_groups(coalesce(organization_id,'00000000-0000-0000-0000-000000000000'::uuid),key);
CREATE INDEX identity_groups_organization ON identity_groups(organization_id);
CREATE FUNCTION identity_group_members_tenant() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM identity_groups g WHERE g.id=NEW.group_id AND g.organization_id IS NOT NULL
   AND NOT EXISTS(SELECT 1 FROM identity_memberships m WHERE m.organization_id=g.organization_id AND m.user_id=NEW.user_id)) THEN
  RAISE EXCEPTION 'IDENTITY_NOT_MEMBER';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER identity_group_members_tenant BEFORE INSERT OR UPDATE ON identity_group_members FOR EACH ROW EXECUTE FUNCTION identity_group_members_tenant();
-- Leaving an organization leaves its groups.
CREATE FUNCTION identity_memberships_left() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 DELETE FROM identity_group_members WHERE user_id=OLD.user_id AND group_id IN (SELECT id FROM identity_groups WHERE organization_id=OLD.organization_id);
 RETURN OLD;
END $$;
CREATE TRIGGER identity_memberships_left AFTER DELETE ON identity_memberships FOR EACH ROW EXECUTE FUNCTION identity_memberships_left();

-- Audit carries the organization a change belongs to (no FK: the audit outlives the organization).
ALTER TABLE identity_admin_events ADD COLUMN organization_id uuid;
CREATE INDEX identity_admin_events_organization ON identity_admin_events(organization_id,occurred_at DESC,id DESC) WHERE organization_id IS NOT NULL;

ALTER TABLE identity_notification_outbox DROP CONSTRAINT identity_notification_outbox_kind_check;
ALTER TABLE identity_notification_outbox ADD CONSTRAINT identity_notification_outbox_kind_check CHECK(kind IN ('password.changed','password.reset','role.changed','sessions.revoked','login.failures','recovery.consumed','organization.joined','organization.left','organization.role'));

ALTER TABLE identity_notification_outbox DROP CONSTRAINT identity_notification_outbox_details_check;
ALTER TABLE identity_notification_outbox ADD CONSTRAINT identity_notification_outbox_details_check CHECK(jsonb_typeof(details)='object' AND details-'role'-'realm'-'count'-'organization'='{}'::jsonb);

-- Adoption.
INSERT INTO identity_organizations(name,is_default,allow_children) VALUES('MinhAnOS',true,true);
INSERT INTO identity_memberships(organization_id,user_id,role)
 SELECT o.id,u.public_id,CASE WHEN u.role='admin' THEN 'admin' ELSE 'member' END FROM users u CROSS JOIN identity_organizations o
 WHERE o.is_default AND u.public_id IS NOT NULL AND u.status<>'deleted';
UPDATE identity_groups SET organization_id=(SELECT id FROM identity_organizations WHERE is_default) WHERE kind='user';
-- A legacy account that receives its public ID later (first sign-in) joins the default organization then.
CREATE FUNCTION identity_users_adopt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO identity_memberships(organization_id,user_id,role)
  SELECT id,NEW.public_id,CASE WHEN NEW.role='admin' THEN 'admin' ELSE 'member' END FROM identity_organizations WHERE is_default
  ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
CREATE TRIGGER identity_users_adopt AFTER UPDATE OF public_id ON users FOR EACH ROW
 WHEN (OLD.public_id IS NULL AND NEW.public_id IS NOT NULL) EXECUTE FUNCTION identity_users_adopt();
