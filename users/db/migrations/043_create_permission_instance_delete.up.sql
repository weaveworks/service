-- See https://github.com/weaveworks/service/issues/2398 for more details
INSERT INTO permissions (id, name, description) VALUES ('instance.delete', 'Delete a team instance', 'Users with this permission are allowed to delete instances shared by the team.') ON CONFLICT DO NOTHING;

-- Only admins can delete team instances.
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('instance.delete', 'admin') ON CONFLICT DO NOTHING;
