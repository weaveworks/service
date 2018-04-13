-- Billing account, the wannabe main billing entity.
-- It is minimalistic at the moment, but should eventually grow to be linked to trials, billing providers, and other billing-related concepts (?).
-- See also: https://docs.google.com/document/d/1NH5U__x9QR-_jKaQpOLnYlqCU8uyFGJx_AqrgxD793o/edit#heading=h.27smegwpb5wj
CREATE TABLE IF NOT EXISTS billing_accounts (
  id                SERIAL PRIMARY KEY,
  created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  deleted_at        TIMESTAMP WITH TIME ZONE,
  billed_externally BOOLEAN DEFAULT FALSE -- A flag to identify who is a customer, even though they
                                          -- may not have a Zuora account, nor a GCP account.
                                          -- Currently, this is useful to allow customers, who have used
                                          -- their trial and are billed externally, to create new instances
                                          -- without having the access to these instances automatically removed,
                                          -- and therefore having to contact us to have their access granted.
                                          -- Eventually, this should be refactored into a no-op, so-called
                                          -- "external"billing provider (?).
);
CREATE UNIQUE INDEX billing_accounts_id_idx ON billing_accounts (id);


-- One-to-many relationship between billing accounts and teams:
-- - A team can have zero or one billing account.
-- - A billing account can be linked to zero, one or n teams.
CREATE TABLE IF NOT EXISTS billing_accounts_teams (
  id                 SERIAL PRIMARY KEY,
  created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  deleted_at         TIMESTAMP WITH TIME ZONE,
  billing_account_id SERIAL NOT NULL REFERENCES billing_accounts (id),
  team_id            TEXT NOT NULL UNIQUE -- REFERENCES users.teams.id
);
CREATE UNIQUE INDEX billing_accounts_teams_billing_account_id_idx ON billing_accounts_teams (billing_account_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX billing_accounts_teams_team_id_idx ON billing_accounts_teams (team_id) WHERE deleted_at IS NULL;
