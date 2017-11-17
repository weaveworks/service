-- Add 'gcp' to uploader_type by renaming the type and recreating it with the new value,
-- because we otherwise get: ERROR 25001: ALTER TYPE ... ADD cannot run inside a transaction block"
-- with our DB migrations framework and PostgreSQL.
ALTER TYPE uploader_type RENAME TO uploader_type_old;
CREATE TYPE uploader_type AS enum ('zuora', 'gcp');

-- Alter all previous values:
ALTER TABLE usage_uploads ALTER COLUMN uploader TYPE uploader_type USING uploader::text::uploader_type;

-- Drop the old enum:
DROP TYPE uploader_type_old;
