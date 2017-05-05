CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS notebooks (
  id            UUID PRIMARY KEY NOT NULL default uuid_generate_v4(),
  org_id        text,
  created_by    text,
  created_at    timestamp with time zone default current_timestamp,
  updated_by    text,
  updated_at    timestamp with time zone default current_timestamp,
  title         text,
  entries       json,
);
