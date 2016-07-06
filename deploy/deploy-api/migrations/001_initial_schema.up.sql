CREATE TABLE IF NOT EXISTS traceable (
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  deleted_at timestamp with time zone
);

DO
  $$
  BEGIN
    CREATE SEQUENCE deploy_id_seq;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS deploys (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('deploy_id_seq'),
  organization_id       text NOT NULL,
  image                 text NOT NULL,
  version               text NOT NULL,
  priority              int,
  state                 text NOT NULL,
  log_key               text,
  created_at            timestamp with time zone default now()
) inherits(traceable);

DO
  $$
  BEGIN
    CREATE SEQUENCE conf_id_seq;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS conf (
  id                    text PRIMARY KEY NOT NULL DEFAULT nextval('conf_id_seq'),
  organization_id       text NOT NULL,
  conf                  text NOT NULL,
  created_at            timestamp with time zone default now()
) inherits(traceable);
