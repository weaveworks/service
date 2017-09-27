-- This adds new columns to replace deny flags.
-- We still keep the deny flags around to be able to roll back a failed deployment.
-- They should be removed after the upgrade is considered successful.

ALTER TABLE organizations ADD COLUMN refuse_data_access boolean NOT NULL DEFAULT FALSE;
ALTER TABLE organizations ADD COLUMN refuse_data_upload boolean NOT NULL DEFAULT FALSE;

UPDATE organizations SET refuse_data_access = deny_ui_features;
UPDATE organizations SET refuse_data_upload = deny_token_auth;

COMMENT ON COLUMN organizations.deny_ui_features IS 'Deprecated: use refuse_data_access';
COMMENT ON COLUMN organizations.deny_token_auth IS 'Deprecated: use refuse_data_upload';
