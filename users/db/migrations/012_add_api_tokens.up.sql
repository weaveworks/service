CREATE SEQUENCE api_tokens_id_seq;
CREATE TABLE IF NOT EXISTS api_tokens (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('api_tokens_id_seq'),
  user_id               text,
  token                 text,
  description           text
) inherits(traceable);
CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX api_tokens_token_idx ON api_tokens (lower(token)) WHERE deleted_at IS NULL;
