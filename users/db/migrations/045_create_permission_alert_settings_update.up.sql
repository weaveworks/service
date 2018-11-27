-- See https://github.com/weaveworks/service/issues/2398 for more details
INSERT INTO permissions (id, name, description) VALUES ('alert.settings.update', 'Edit alerting rules', 'Users with this permission are allowed to edit the Prometheus alerting rules.') ON CONFLICT DO NOTHING;

-- Both admins and editors can update alerting rules.
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('alert.settings.update', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('alert.settings.update', 'editor') ON CONFLICT DO NOTHING;
