-- Phase 5: Files authorization goes through the identity policy (user entitlement AND app grant AND
-- app access rules); resource rules (own namespace, host operator for the system root) stay in Files.
INSERT INTO identity_permissions(key,description) VALUES('files.read','files.read'),('files.write','files.write'),('files.system.read','files.system.read'),('files.system.write','files.system.write') ON CONFLICT DO NOTHING;
-- Every active account keeps today's behaviour: its own files. System root needs System Admin + host operator.
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT r.id,p FROM identity_roles r CROSS JOIN unnest(ARRAY['files.read','files.write']) p WHERE r.key IN ('standard-user','user-admin','system-admin') ON CONFLICT DO NOTHING;
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT r.id,p FROM identity_roles r CROSS JOIN unnest(ARRAY['files.system.read','files.system.write']) p WHERE r.key='system-admin' ON CONFLICT DO NOTHING;
INSERT INTO identity_app_grants(app_id,permission_key,scope,state) SELECT 'com.minhanos.files',p,'{"kind":"system","id":null}','enabled' FROM unnest(ARRAY['files.read','files.write','files.system.read','files.system.write']) p WHERE NOT EXISTS(SELECT 1 FROM identity_app_grants g WHERE g.app_id='com.minhanos.files' AND g.permission_key=p);
-- One deny rule per app and subject; access rules are looked up per app on every Files request.
CREATE UNIQUE INDEX identity_app_access_subject ON identity_app_access(app_id,effect,coalesce(user_id,group_id,role_id));
