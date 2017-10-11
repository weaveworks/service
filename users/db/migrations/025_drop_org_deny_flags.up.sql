-- Remove deprecated flags, they have been replaced by refuse_data_{access,upload}

ALTER TABLE organizations DROP COLUMN deny_ui_features;
ALTER TABLE organizations DROP COLUMN deny_token_auth;
