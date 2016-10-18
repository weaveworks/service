CREATE TABLE IF NOT EXISTS traceable (
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  deleted_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS configs (
  id text NOT NULL,
  type text NOT NULL,
  conf jsonb NOT NULL,
  PRIMARY KEY (id, type)
) inherits(traceable);
