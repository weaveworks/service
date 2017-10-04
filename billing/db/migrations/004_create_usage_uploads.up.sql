CREATE TABLE IF NOT EXISTS usage_uploads (
  id               serial primary key,
  max_aggregate_id integer,
  started_at       timestamp with time zone default now()
);

COMMENT ON COLUMN usage_uploads.max_aggregate_id IS 'the highest aggregates.id seen during this upload run';

-- We insert the latest aggregates `id` which is *not* correct. We will
-- not upload the usage since last midnight. But it's not a big deal since
-- no customer has billing enabled so far.
-- Doing it properly is not trivial since the newly added `aggregates.id`
-- does not seem to be ordered by anything.
INSERT INTO usage_uploads (max_aggregate_id) SELECT MAX(id) FROM aggregates;
