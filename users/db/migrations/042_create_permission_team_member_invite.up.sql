-- See https://github.com/weaveworks/service/issues/2398 for more details
INSERT INTO permissions (id, name, description) VALUES ('team.member.invite', 'Invite new team members', 'Users with this permission are allowed to invite new team members to the team.') ON CONFLICT DO NOTHING;

-- Only admins can invite new members to the team.
INSERT INTO role_permission_memberships(permission_id, role_id) VALUES ('team.member.invite', 'admin') ON CONFLICT DO NOTHING;
