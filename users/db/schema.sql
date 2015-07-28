CREATE DATABASE weave_development WITH ENCODING = 'UTF-8';

\c weave_development;

CREATE TABLE IF NOT EXISTS traceable (
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  deleted_at timestamp with time zone
);

CREATE SEQUENCE users_id_seq;
CREATE TABLE IF NOT EXISTS users (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('users_id_seq'),
  email                 text,
  organization_id       text,
  token                 text,
  token_created_at      timestamp with time zone,
  approved_at           timestamp with time zone,
  first_login_at        timestamp with time zone,
  last_login_at         timestamp with time zone
) inherits(traceable);
CREATE UNIQUE INDEX users_lower_email_idx ON users (lower(email)) WHERE deleted_at IS NULL;

CREATE SEQUENCE organizations_id_seq;
CREATE TABLE IF NOT EXISTS organizations (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('organizations_id_seq'),
  name                  text,
  first_probe_update_at timestamp with time zone,
  last_probe_update_at  timestamp with time zone
) inherits(traceable);

\c postgres;

CREATE DATABASE weave_test WITH TEMPLATE weave_development;
