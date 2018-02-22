ALTER TABLE events
  ADD fallback varchar(255) NOT NULL,
  ADD html text,
  ADD metadata json,
  ALTER COLUMN messages DROP NOT NULL;
