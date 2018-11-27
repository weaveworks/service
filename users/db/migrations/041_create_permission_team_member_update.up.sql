-- See https://github.com/weaveworks/service/issues/2398 for more details
INSERT INTO permissions (id, name, description) VALUES ('team.member.update', 'Grant/remove permissions', 'Users with this permission are allowed to change the permissions of other existing team members.') ON CONFLICT DO NOTHING;

-- Only admins can manage team permissions.
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('team.member.update', 'admin') ON CONFLICT DO NOTHING;
