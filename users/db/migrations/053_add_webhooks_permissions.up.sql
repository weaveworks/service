-- instance.webhook.create
INSERT INTO permissions(id, name, description) VALUES ('instance.webhook.create', 'Create instance webhook', 'Users with this permission are allowed to create webhooks for instances.') ON CONFLICT DO NOTHING;
-- admins and editors can create webhooks
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.webhook.create', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.webhook.create', 'editor') ON CONFLICT DO NOTHING;



-- instance.webhook.delete
INSERT INTO permissions(id, name, description) VALUES ('instance.webhook.delete', 'Delete instance webhook', 'Users with this permission are allowed to delete webhooks for instances.') ON CONFLICT DO NOTHING;
-- admins and editors can delete webhooks
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.webhook.delete', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.webhook.delete', 'editor') ON CONFLICT DO NOTHING;
