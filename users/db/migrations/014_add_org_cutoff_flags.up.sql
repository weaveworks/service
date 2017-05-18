ALTER TABLE organizations ADD COLUMN deny_ui_features boolean NOT NULL DEFAULT FALSE;
ALTER TABLE organizations ADD COLUMN deny_token_auth boolean NOT NULL DEFAULT FALSE;
