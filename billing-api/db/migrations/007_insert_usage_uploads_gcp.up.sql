-- Usage upload starts now, from latest aggregate.
INSERT INTO usage_uploads (max_aggregate_id, uploader) SELECT MAX(id), 'gcp' FROM aggregates;
