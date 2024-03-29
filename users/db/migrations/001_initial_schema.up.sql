CREATE TABLE IF NOT EXISTS traceable (
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  deleted_at timestamp with time zone
);

DO
  $$
  BEGIN
    CREATE SEQUENCE users_id_seq;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS users (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('users_id_seq'),
  email                 text,
  organization_id       text,
  token                 text,
  token_created_at      timestamp with time zone,
  approved_at           timestamp with time zone,
  first_login_at        timestamp with time zone
) inherits(traceable);
DO
  $$
  BEGIN
    CREATE UNIQUE INDEX users_lower_email_idx ON users (lower(email)) WHERE deleted_at IS NULL;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

DO
  $$
  BEGIN
    CREATE SEQUENCE organizations_id_seq;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS organizations (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('organizations_id_seq'),
  name                  text,
  probe_token           text,
  first_probe_update_at timestamp with time zone
) inherits(traceable);

DO
  $$
  BEGIN
    CREATE UNIQUE INDEX organizations_lower_name_idx ON organizations (lower(name)) WHERE deleted_at IS NULL;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

DO
  $$
  BEGIN
    CREATE UNIQUE INDEX organizations_probe_token_idx ON organizations (probe_token) WHERE deleted_at IS NULL;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;
