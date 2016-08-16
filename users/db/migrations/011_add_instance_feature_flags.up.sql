ALTER TABLE organizations ADD COLUMN feature_flags text[] NOT NULL DEFAULT '{}';
