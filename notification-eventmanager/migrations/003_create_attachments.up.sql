CREATE TABLE IF NOT EXISTS attachments (
  PRIMARY KEY (attachment_id),
  attachment_id uuid NOT NULL,
  event_id uuid NOT NULL REFERENCES events ON DELETE CASCADE,
  format varchar(255) NOT NULL,
  body text NOT NULL
)
