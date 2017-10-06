CREATE TABLE IF NOT EXISTS post_trial_invoices(
  usage_import_id      text not null primary key,
  external_id          text not null,
  zuora_account_number text not null,
  created_at           timestamp with time zone not null default now()
);
