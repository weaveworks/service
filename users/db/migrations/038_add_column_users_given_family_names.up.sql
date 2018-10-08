ALTER TABLE users ADD COLUMN given_name text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN family_name text NOT NULL DEFAULT '';

-- This will split the full name into first/given and last/family names, and populate the fields accordingly
-- It's not fool-proof or pretty, but it'll be good enough
-- A lot of users have only entered single word names, so we take that name to be a first name
UPDATE users
SET given_name = trim(both ' ' from split_names.given_name),
    family_name = trim(both ' ' from split_names.family_name)
FROM (
    WITH w_family_name AS (
        select
            id,
            name,
            reverse(substr(reverse(name),1,strpos(reverse(name),' '))) as family_name
        FROM users
        WHERE name != ''
    )
    SELECT
        id,
        substr(name, 1, length(name) - length(family_name)) as given_name,
        family_name
    from w_family_name
) AS split_names
WHERE users.id = split_names.id;
