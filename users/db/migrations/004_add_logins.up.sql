CREATE SEQUENCE logins_id_seq;
CREATE TABLE IF NOT EXISTS logins (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('logins_id_seq'),
  user_id               text,
  provider              text,
  provider_id           text,
  session               jsonb
) inherits(traceable);
CREATE UNIQUE INDEX logins_provider_provider_id_idx ON logins (provider, provider_id) WHERE deleted_at IS NULL;
