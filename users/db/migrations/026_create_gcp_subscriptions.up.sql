CREATE SEQUENCE gcp_subscriptions_id_seq;
CREATE TABLE IF NOT EXISTS gcp_subscriptions (
  id                 text PRIMARY KEY NOT NULL DEFAULT nextval('gcp_subscriptions_id_seq'),
  account_id         text not null,
  active             boolean default false,
  created_at         timestamp with time zone not null default now(),
  consumer_id        text,
  subscription_name  text,
  subscription_level text
);
CREATE UNIQUE INDEX gcp_subscriptions_account_id_idx ON gcp_subscriptions (account_id);
CREATE UNIQUE INDEX gcp_subscriptions_subscription_name_idx ON gcp_subscriptions (subscription_name);

ALTER TABLE organizations ADD COLUMN gcp_subscription_id text;
