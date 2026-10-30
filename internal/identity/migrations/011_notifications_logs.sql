-- Notification Center (com.minhanos.notifications) and Log Center (com.minhanos.logs), minimal v1.
INSERT INTO identity_permissions(key,description) VALUES('notifications.own.read','notifications.own.read'),('notifications.own.manage','notifications.own.manage'),('logs.audit.read','logs.audit.read') ON CONFLICT DO NOTHING;
-- Every account reads and marks its own inbox (standard-user is the base of every account).
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT r.id,p FROM identity_roles r CROSS JOIN unnest(ARRAY['notifications.own.read','notifications.own.manage']) p WHERE r.key IN ('standard-user','user-admin','system-admin') ON CONFLICT DO NOTHING;
-- The audit log of every module is System Admin only for now.
INSERT INTO identity_role_permissions(role_id,permission_key) SELECT id,'logs.audit.read' FROM identity_roles WHERE key='system-admin' ON CONFLICT DO NOTHING;
INSERT INTO identity_app_grants(app_id,permission_key,scope,state) SELECT 'com.minhanos.notifications',p,'{"kind":"system","id":null}','enabled' FROM unnest(ARRAY['notifications.own.read','notifications.own.manage']) p WHERE NOT EXISTS(SELECT 1 FROM identity_app_grants g WHERE g.app_id='com.minhanos.notifications' AND g.permission_key=p);
INSERT INTO identity_app_grants(app_id,permission_key,scope,state) SELECT 'com.minhanos.logs','logs.audit.read','{"kind":"system","id":null}','enabled' WHERE NOT EXISTS(SELECT 1 FROM identity_app_grants WHERE app_id='com.minhanos.logs' AND permission_key='logs.audit.read');
