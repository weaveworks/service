ALTER TABLE organizations ADD COLUMN label text;
UPDATE organizations
   SET label = name
 WHERE label IS NULL;
