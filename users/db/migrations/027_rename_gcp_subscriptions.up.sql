-- Rename table
alter table gcp_subscriptions rename to gcp_accounts;
alter sequence gcp_subscriptions_id_seq rename to gcp_accounts_id_seq;
alter table organizations rename gcp_subscription_id to gcp_account_id;
-- Rename columns
alter table gcp_accounts rename active to activated;

COMMENT ON COLUMN gcp_accounts.account_id IS 'Google externalAccountID';
COMMENT ON COLUMN gcp_accounts.activated IS 'Whether this account has been activated during signup';
COMMENT ON COLUMN gcp_accounts.consumer_id IS 'Identifies this account for usage upload';
