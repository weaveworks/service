ALTER TABLE organizations ADD COLUMN external_id text;

DO
  $$
  BEGIN
    CREATE UNIQUE INDEX organizations_lower_external_id_idx ON organizations (lower(external_id)) WHERE deleted_at IS NULL;
    EXCEPTION WHEN duplicate_table THEN
      -- do nothing, it's already there
END
$$ LANGUAGE plpgsql;

UPDATE organizations SET external_id = name;
ALTER TABLE organizations ALTER COLUMN external_id SET NOT NULL;
