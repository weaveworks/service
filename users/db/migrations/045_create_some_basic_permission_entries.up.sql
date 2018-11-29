-- See https://github.com/weaveworks/service/issues/2398 for more details


-- team.member.update
INSERT INTO permissions (id, name, description) VALUES ('team.member.update', 'Grant/remove permissions', 'Users with this permission are allowed to change the permissions of other existing team members.') ON CONFLICT DO NOTHING;
-- Only admins can manage team permissions.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.member.update', 'admin') ON CONFLICT DO NOTHING;


-- team.member.invite
INSERT INTO permissions (id, name, description) VALUES ('team.member.invite', 'Invite new team members', 'Users with this permission are allowed to invite new team members to the team.') ON CONFLICT DO NOTHING;
-- Only admins can invite new members to the team.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.member.invite', 'admin') ON CONFLICT DO NOTHING;


-- instance.delete
INSERT INTO permissions (id, name, description) VALUES ('instance.delete', 'Delete a team instance', 'Users with this permission are allowed to delete instances shared by the team.') ON CONFLICT DO NOTHING;
-- Only admins can delete team instances.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.delete', 'admin') ON CONFLICT DO NOTHING;


-- instance.billing.update
INSERT INTO permissions (id, name, description) VALUES ('instance.billing.update', 'Manage billing', 'Users with this permission are allowed to manage billing and update credit card details.') ON CONFLICT DO NOTHING;
-- Only admins can manage billing details.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.billing.update', 'admin') ON CONFLICT DO NOTHING;


-- alert.settings.update
INSERT INTO permissions (id, name, description) VALUES ('alert.settings.update', 'Edit alerting rules', 'Users with this permission are allowed to edit the Prometheus alerting rules.') ON CONFLICT DO NOTHING;
-- Both admins and editors can update alerting rules.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('alert.settings.update', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('alert.settings.update', 'editor') ON CONFLICT DO NOTHING;
