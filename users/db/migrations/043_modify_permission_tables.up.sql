-- Rename roles_permissions table (see https://github.com/weaveworks/service/pull/2409#discussion_r236929059)
ALTER TABLE role_permission_memberships RENAME TO roles_permissions;
ALTER INDEX role_permission_memberships_permission_id_role_id_idx RENAME TO roles_permissions_role_id_permission_id_idx;

-- Drop user_team_role_memberships table with all its dependencies ...
DROP TABLE user_team_role_memberships;
DROP SEQUENCE user_team_role_memberships_id_seq;

-- ... in favor of an extra column in team_memberships table (see https://github.com/weaveworks/service/pull/2409#discussion_r236929914)
ALTER TABLE team_memberships ADD COLUMN role_id text REFERENCES roles(id);

-- Also create a new unique index for team_memberships that includes the role (to enforce single one) in favor of old unique index
CREATE UNIQUE INDEX team_memberships_user_id_team_id_role_id_idx ON team_memberships (user_id, team_id, role_id) WHERE deleted_at IS NULL;
DROP INDEX team_memberships_user_id_team_id_idx;