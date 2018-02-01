ALTER TABLE organizations ADD COLUMN cleanup BOOLEAN;
UPDATE organizations SET cleanup = 'f';
ALTER TABLE organizations ALTER COLUMN cleanup SET NOT NULL;
ALTER TABLE organizations ALTER COLUMN cleanup SET DEFAULT FALSE;
