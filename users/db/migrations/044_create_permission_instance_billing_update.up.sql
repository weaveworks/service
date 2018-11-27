-- See https://github.com/weaveworks/service/issues/2398 for more details
INSERT INTO permissions (id, name, description) VALUES ('instance.billing.update', 'Manage billing', 'Users with this permission are allowed to manage billing and update credit card details.') ON CONFLICT DO NOTHING;

-- Only admins can manage billing details.
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('instance.billing.update', 'admin') ON CONFLICT DO NOTHING;
