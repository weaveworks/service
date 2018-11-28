-- Rename roles_permissions table (see https://github.com/weaveworks/service/pull/2409#discussion_r236929059)
ALTER TABLE role_permission_memberships RENAME TO roles_permissions;
ALTER INDEX role_permission_memberships_permission_id_role_id_idx RENAME TO roles_permissions_role_id_permission_id_idx;

-- Drop user_team_role_memberships table with all its dependencies ...
DROP TABLE IF EXISTS user_team_role_memberships;
DROP SEQUENCE IF EXISTS user_team_role_memberships_id_seq;

-- ... in favor of an extra column in team_memberships table (see https://github.com/weaveworks/service/pull/2409#discussion_r236929914)
-- TODO(fbarl): The default 'admin' should be removed once we enable permissions in the UI (see https://github.com/weaveworks/service/pull/2413/files#r237180045)
ALTER TABLE team_memberships ADD COLUMN role_id text NOT NULL REFERENCES roles(id) DEFAULT 'admin';

-- Remove ID sequence on roles (see https://github.com/weaveworks/service/pull/2413#discussion_r237190087)
COMMENT ON COLUMN roles.id IS 'This should always be set to a unique descriptor name, e.g. ''admin'' or ''editor''.';
ALTER TABLE roles ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS roles_id_seq;
