DROP INDEX IF EXISTS organizations_lower_name_idx;
UPDATE organizations SET name = label;
