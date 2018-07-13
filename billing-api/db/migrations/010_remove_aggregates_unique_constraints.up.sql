-- Relax unicity constraint on aggregates' (instance_id, bucket_start, amount_type),
-- as we are no longer upserting, but simplying inserting in DB.
DROP INDEX aggregates_instance_bucket_type_idx
