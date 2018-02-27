ALTER TABLE events
  ADD text text,
  ADD metadata json,
  ALTER COLUMN messages DROP NOT NULL;
