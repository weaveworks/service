ALTER TABLE usage_uploads ADD COLUMN uploader text NOT NULL DEFAULT 'zuora';
ALTER TABLE usage_uploads ALTER COLUMN uploader DROP DEFAULT;
