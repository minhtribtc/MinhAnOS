-- Storage Manager (com.minhanos.storage): shares, quotas and volumes are System Admin only for now.
INSERT INTO identity_permissions(key,description) VALUES('storage.manage','storage.manage') ON CONFLICT DO NOTHING;
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT id,'storage.manage' FROM identity_roles WHERE key='system-admin' ON CONFLICT DO NOTHING;
INSERT INTO identity_app_grants(app_id,permission_key,scope,state) SELECT 'com.minhanos.storage','storage.manage','{"kind":"system","id":null}','enabled' WHERE NOT EXISTS(SELECT 1 FROM identity_app_grants WHERE app_id='com.minhanos.storage' AND permission_key='storage.manage');
