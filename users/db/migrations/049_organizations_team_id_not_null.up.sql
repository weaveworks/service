-- Every organization must have a team now.
ALTER TABLE organizations ALTER COLUMN team_id SET NOT NULL;
