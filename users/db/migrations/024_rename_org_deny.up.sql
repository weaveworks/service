ALTER TABLE organizations RENAME COLUMN deny_ui_features TO refuse_data_access;
ALTER TABLE organizations RENAME COLUMN deny_token_auth TO refuse_data_upload;
