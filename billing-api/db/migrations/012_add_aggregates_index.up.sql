CREATE INDEX idx_instance_id_bucket_start_amount_type ON aggregates USING btree (instance_id, bucket_start, amount_type);
