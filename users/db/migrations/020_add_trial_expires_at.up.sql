-- Store trial expiry information in the database.
--
-- Because we're going to give everyone an extra 30 days of trial after we
-- enable billing (see https://github.com/weaveworks/billing/issues/170), we
-- can ignore the `trial:days` feature flag and just say that everyone has 30
-- days from the migration.
--
-- We can update these values later when we go live.
ALTER TABLE organizations ADD COLUMN trial_expires_at timestamp with time zone NOT NULL DEFAULT now() + interval '30 days';
ALTER TABLE organizations ALTER COLUMN trial_expires_at DROP DEFAULT;
