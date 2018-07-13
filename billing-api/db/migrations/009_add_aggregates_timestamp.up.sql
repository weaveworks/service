-- Adds a `created_at` timestamp to track when usage aggregates are being inserted in the DB.
-- This is useful to track when usage data made it to BigQuery, and eventually our billing DB,
-- and being able to identify any delay along the way.
ALTER TABLE aggregates ADD COLUMN created_at timestamp with time zone NOT NULL DEFAULT now()
