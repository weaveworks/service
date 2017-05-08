CREATE TABLE IF NOT EXISTS notebooks (
  id            UUID PRIMARY KEY NOT NULL,
  org_id        text,
  created_by    text,
  created_at    timestamp with time zone default current_timestamp,
  updated_by    text,
  updated_at    timestamp with time zone default current_timestamp,
  version       UUID NOT NULL,
  title         text,
  entries       json
);
