ALTER TABLE aggregates ADD COLUMN upload_id INTEGER DEFAULT NULL;

-- Flag all existing aggregate records as processed
WITH upd AS (
    INSERT INTO usage_uploads (max_aggregate_id, uploader)
        (SELECT MAX(id), 'zuora'
        FROM aggregates)
    RETURNING id
)
UPDATE aggregates SET upload_id = (SELECT id from upd) WHERE upload_id is NULL;

-- An index to make it easy to find usage which needs processing
CREATE INDEX idx_aggregates_to_upload_instance_id 
ON aggregates USING btree (instance_id)
WHERE upload_id IS NULL;
