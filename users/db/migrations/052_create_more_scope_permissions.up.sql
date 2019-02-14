
-- scope.pod.logs.view
INSERT INTO permissions (id, name, description) VALUES ('scope.pod.logs.view', 'View pod logs', 'Users with this permission are allowed to open logs terminal for Kubernetes pods.') ON CONFLICT DO NOTHING;
-- Everyone can view K8s pod logs.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.pod.logs.view', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.pod.logs.view', 'editor') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.pod.logs.view', 'viewer') ON CONFLICT DO NOTHING;

-- scope.container.pause
INSERT INTO permissions (id, name, description) VALUES ('scope.container.pause', 'Pause Docker containers', 'Users with this permission are allowed to pause and unpause Docker containers.') ON CONFLICT DO NOTHING;
-- Only admins and editors can pause Docker containers.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.pause', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.pause', 'editor') ON CONFLICT DO NOTHING;

-- scope.container.restart
INSERT INTO permissions (id, name, description) VALUES ('scope.container.restart', 'Restart Docker containers', 'Users with this permission are allowed to restart Docker containers.') ON CONFLICT DO NOTHING;
-- Only admins and editors can restart Docker containers.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.restart', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.restart', 'editor') ON CONFLICT DO NOTHING;

-- scope.container.stop
INSERT INTO permissions (id, name, description) VALUES ('scope.container.stop', 'Stop Docker containers', 'Users with this permission are allowed to stop Docker containers.') ON CONFLICT DO NOTHING;
-- Only admins and editors can stop Docker containers.
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.stop', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO roles_permissions(permission_id, role_id) VALUES ('scope.container.stop', 'editor') ON CONFLICT DO NOTHING;
