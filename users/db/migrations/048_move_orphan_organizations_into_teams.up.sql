-- Move orphan organizations together with their user members into the new teams.
-- See https://github.com/weaveworks/service/issues/2117 for more details.

-- Move all orphan organizations into their dedicated teams
UPDATE organizations SET team_id = teams.id FROM teams
WHERE organizations.team_id IS NULL AND CONCAT(organizations.external_id, '-team') = teams.external_id;

-- Move all user memberships into the new teams
-- (implicitly linking the users to same organizations)
--
-- Note that some entries in `memberships` might already have an
-- entry in `team_memberships` since organizations are allowed to
-- be transferred into other teams. In that case, the entry in
-- `memberships` was not removed.
INSERT INTO team_memberships(team_id, user_id, created_at, deleted_at)
SELECT organizations.team_id, memberships.user_id, memberships.created_at, memberships.deleted_at
FROM memberships
  INNER JOIN organizations ON organizations.id = memberships.organization_id
  INNER JOIN teams ON organizations.team_id = teams.id
WHERE
  teams.external_id LIKE '%-team'
;

-- Finally delete all the legacy direct user-organization memberships
-- (we keep the table since it's still being referenced in the code)
DELETE FROM memberships;
