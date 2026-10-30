-- Admin session control and forced password change after an administrative reset.
ALTER TABLE users ADD COLUMN password_change_required boolean NOT NULL DEFAULT false;
INSERT INTO identity_permissions(key,description) VALUES('identity.users.sessions.read','identity.users.sessions.read'),('identity.users.sessions.revoke','identity.users.sessions.revoke') ON CONFLICT DO NOTHING;
-- System Admin and the bounded User Admin role; target ceilings still apply per request.
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT r.id,p FROM identity_roles r CROSS JOIN unnest(ARRAY['identity.users.sessions.read','identity.users.sessions.revoke']) p WHERE r.key IN ('system-admin','user-admin') ON CONFLICT DO NOTHING;
INSERT INTO identity_app_grants(app_id,permission_key,scope,state) SELECT 'com.minhanos.account-center',p,'{"kind":"system","id":null}','enabled' FROM unnest(ARRAY['identity.users.sessions.read','identity.users.sessions.revoke']) p WHERE NOT EXISTS(SELECT 1 FROM identity_app_grants g WHERE g.app_id='com.minhanos.account-center' AND g.permission_key=p);
