-- See https://github.com/weaveworks/service/issues/2398 for more details


-- team.member.remove
INSERT INTO permissions (id, name, description) VALUES ('team.member.remove', 'Remove team members', 'Users with this permission are allowed to remove existing members from the team.') ON CONFLICT DO NOTHING;
-- Only admins can remove team members.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.member.remove', 'admin') ON CONFLICT DO NOTHING;


-- team.members.view
INSERT INTO permissions (id, name, description) VALUES ('team.members.view', 'View team members', 'Users with this permission are allowed to view all members of the team.') ON CONFLICT DO NOTHING;
-- Everyone can view team members.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.members.view', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.members.view', 'editor') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('team.members.view', 'viewer') ON CONFLICT DO NOTHING;


-- instance.transfer
INSERT INTO permissions (id, name, description) VALUES ('instance.transfer', 'Transfer instance', 'Users with this permission are allowed to transfer this instance between teams.') ON CONFLICT DO NOTHING;
-- Only admins can transfer instances to other teams.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.transfer', 'admin') ON CONFLICT DO NOTHING;


-- instance.token.view
INSERT INTO permissions (id, name, description) VALUES ('instance.token.view', 'View instance token', 'Users with this permission are allowed to view the instance token.') ON CONFLICT DO NOTHING;
-- Only admins can view the instance token (we probably only need it for setting up the instance, users who'd know the token could assume a lot of control over the instance).
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('instance.token.view', 'admin') ON CONFLICT DO NOTHING;


-- notification.settings.update
INSERT INTO permissions (id, name, description) VALUES ('notification.settings.update', 'Update notification settings', 'Users with this permission are allowed to update the notification settings.') ON CONFLICT DO NOTHING;
-- Only admins can update the notification settings.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notification.settings.update', 'admin') ON CONFLICT DO NOTHING;


-- notebook.create
INSERT INTO permissions (id, name, description) VALUES ('notebook.create', 'Create a new notebook', 'Users with this permission are allowed to create new notebooks.') ON CONFLICT DO NOTHING;
-- Both admins and editors can create notebooks.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.create', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.create', 'editor') ON CONFLICT DO NOTHING;


-- notebook.update
INSERT INTO permissions (id, name, description) VALUES ('notebook.update', 'Update a notebook', 'Users with this permission are allowed to update notebooks.') ON CONFLICT DO NOTHING;
-- Both admins and editors can update notebooks.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.update', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.update', 'editor') ON CONFLICT DO NOTHING;


-- notebook.delete
INSERT INTO permissions (id, name, description) VALUES ('notebook.delete', 'Delete a notebook', 'Users with this permission are allowed to delete notebooks.') ON CONFLICT DO NOTHING;
-- Both admins and editors can delete notebooks.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.delete', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('notebook.delete', 'editor') ON CONFLICT DO NOTHING;


-- scope.host.exec
INSERT INTO permissions (id, name, description) VALUES ('scope.host.exec', 'Open a shell in a host', 'Users with this permission are allowed to open a shell in a host.') ON CONFLICT DO NOTHING;
-- Only admins can open a shell in a host.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.host.exec', 'admin') ON CONFLICT DO NOTHING;


-- scope.container.exec
INSERT INTO permissions (id, name, description) VALUES ('scope.container.exec', 'Open a shell on a container', 'Users with this permission are allowed to open a shell on a container.') ON CONFLICT DO NOTHING;
-- Only admins can open a shell on a container.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.exec', 'admin') ON CONFLICT DO NOTHING;


-- scope.container.attach.in
INSERT INTO permissions (id, name, description) VALUES ('scope.container.attach.in', 'Attach to a running container (write access)', 'Users with this permission are allowed to attach to a running container with write access.') ON CONFLICT DO NOTHING;
-- Only admins can attach to a running container with write access.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.attach.in', 'admin') ON CONFLICT DO NOTHING;


-- scope.container.attach.out
INSERT INTO permissions (id, name, description) VALUES ('scope.container.attach.out', 'Attach to a running container (read access)', 'Users with this permission are allowed to attach to a running container with read access.') ON CONFLICT DO NOTHING;
-- Both admins and editors can attach to a running container with read access.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.attach.out', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.attach.out', 'editor') ON CONFLICT DO NOTHING;


-- scope.replicas.update
INSERT INTO permissions (id, name, description) VALUES ('scope.replicas.update', 'Change the desired replica count', 'Users with this permission are allowed to update desired replica counts.') ON CONFLICT DO NOTHING;
-- Both admins and editors can change the desired replica count.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.replicas.update', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.replicas.update', 'editor') ON CONFLICT DO NOTHING;


-- scope.pod.delete
INSERT INTO permissions (id, name, description) VALUES ('scope.pod.delete', 'Delete a pod', 'Users with this permission are allowed to delete pods.') ON CONFLICT DO NOTHING;
-- Both admins and editors can delete pods.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.pod.delete', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.pod.delete', 'editor') ON CONFLICT DO NOTHING;


-- flux.image.deploy
INSERT INTO permissions (id, name, description) VALUES ('flux.image.deploy', 'Deploy a new image', 'Users with this permission are allowed to deploy new images.') ON CONFLICT DO NOTHING;
-- Both admins and editors can deploy new images.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('flux.image.deploy', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('flux.image.deploy', 'editor') ON CONFLICT DO NOTHING;


-- flux.policy.update
INSERT INTO permissions (id, name, description) VALUES ('flux.policy.update', 'Update workload deploy policies', 'Users with this permission are allowed to update workload deploy policies.') ON CONFLICT DO NOTHING;
-- Both admins and editors can update workload deploy policies.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('flux.policy.update', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('flux.policy.update', 'editor') ON CONFLICT DO NOTHING;
