-- See https://github.com/weaveworks/service/issues/2120 for more details

CREATE SEQUENCE permissions_id_seq;
CREATE TABLE IF NOT EXISTS permissions (
    id                  text PRIMARY KEY NOT NULL DEFAULT nextval('permissions_id_seq'),
    name                text NOT NULL,
    description         text NOT NULL,

    created_at          timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at          timestamp with time zone
);

CREATE SEQUENCE roles_id_seq;
CREATE TABLE IF NOT EXISTS roles (
    id                  text PRIMARY KEY NOT NULL DEFAULT nextval('roles_id_seq'),
    name                text NOT NULL,

    created_at          timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at          timestamp with time zone
);

INSERT INTO roles (id, name) VALUES ('admin', 'Admin') ON CONFLICT DO NOTHING;
INSERT INTO roles (id, name) VALUES ('editor', 'Editor') ON CONFLICT DO NOTHING;
INSERT INTO roles (id, name) VALUES ('viewer', 'Viewer') ON CONFLICT DO NOTHING;

CREATE SEQUENCE role_permission_memberships_id_seq;
CREATE TABLE IF NOT EXISTS role_permission_memberships (
    id                  text PRIMARY KEY NOT NULL DEFAULT nextval('role_permission_memberships_id_seq'),
    permission_id       text NOT NULL REFERENCES permissions(id),
    role_id             text NOT NULL REFERENCES roles(id),

    created_at          timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at          timestamp with time zone
);

CREATE UNIQUE INDEX role_permission_memberships_permission_id_role_id_idx ON role_permission_memberships (permission_id, role_id) WHERE deleted_at IS NULL;

CREATE SEQUENCE user_team_role_memberships_id_seq;
CREATE TABLE IF NOT EXISTS user_team_role_memberships (
    id                  text PRIMARY KEY NOT NULL DEFAULT nextval('user_team_role_memberships_id_seq'),
    user_id             text NOT NULL REFERENCES users(id),
    team_id             text NOT NULL REFERENCES teams(id),
    role_id             text NOT NULL REFERENCES roles(id),

    created_at          timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at          timestamp with time zone
);

CREATE UNIQUE INDEX user_team_role_memberships_user_id_team_id_role_id_idx ON user_team_role_memberships (user_id, team_id, role_id) WHERE deleted_at IS NULL;
