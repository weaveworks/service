-- Adds a primary key `id` while replacing the existing pkey with a unique constraint
CREATE UNIQUE INDEX aggregates_instance_bucket_type_idx ON aggregates (instance_id, bucket_start, amount_type);
ALTER TABLE aggregates DROP CONSTRAINT aggregates_pkey, ADD COLUMN id serial primary key;
