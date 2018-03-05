CREATE TABLE IF NOT EXISTS attachments (
  attachment_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  event_id uuid NOT NULL REFERENCES events ON DELETE CASCADE,
  format varchar(255) NOT NULL,
  body text NOT NULL
)
