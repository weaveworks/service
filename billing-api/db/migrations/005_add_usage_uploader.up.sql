CREATE TYPE uploader_type AS enum ('zuora');
ALTER TABLE usage_uploads ADD COLUMN uploader uploader_type NOT NULL DEFAULT 'zuora';
ALTER TABLE usage_uploads ALTER COLUMN uploader DROP DEFAULT;
