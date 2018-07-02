ALTER TABLE aggregates ADD COLUMN upload_id INTEGER DEFAULT NULL;

-- An index to make it easy to find usage which needs processing
CREATE INDEX idx_aggregates_to_upload_instance_id 
ON aggregates USING btree (instance_id)
WHERE upload_id IS NULL;