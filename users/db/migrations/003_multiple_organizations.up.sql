CREATE SEQUENCE memberships_id_seq;
CREATE TABLE IF NOT EXISTS memberships (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('memberships_id_seq'),
  user_id               text,
  organization_id       text
) inherits(traceable);
CREATE UNIQUE INDEX memberships_user_id_organization_id_idx ON memberships (user_id, organization_id) WHERE deleted_at IS NULL;

INSERT INTO memberships (user_id, organization_id) (SELECT id, organization_id from users);
ALTER TABLE users DROP COLUMN organization_id;
