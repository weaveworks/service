CREATE TABLE IF NOT EXISTS notebooks (
  id            UUID PRIMARY KEY NOT NULL,
  org_id        text,
  title         text,
  entries       json,
  author_id     text,
  updated_at    timestamp with time zone
);
