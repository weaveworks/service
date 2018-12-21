-- Ground work for https://github.com/weaveworks/service/issues/2117.
--
--
-- Creates one team for each of the orphan organizations (i.e. organizations that are so far not part of a team):
--   * Teams will be given the same name based on organization name, as is currently the case in the app
--   * External IDs will be a direct extension of organization ones to be able to map them 1-1 when continuing the migration
--   * Since organizations.external_id might contain duplicates (constraint only on `deleted_at is null`) we additionally
--     include the `organizations.id`.
--   * We also handle deleted organizations by creating deleted teams for them
--   * The trial expiry date will be carried over as well
--
-- NOTE: This INSERT action will fail on any external ID conflicts so there should be none before the migration is ran.
--       We absolutely don't want to move orphan organizations into existing teams belonging to different users!

INSERT INTO teams(name, external_id, trial_expires_at, deleted_at)
SELECT CONCAT(LEFT(name, 95), ' Team') AS name, CONCAT(external_id, '-', id) AS external_id, trial_expires_at, deleted_at
FROM organizations WHERE team_id IS NULL;
