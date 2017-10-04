CREATE TABLE IF NOT EXISTS aggregates (
  instance_id         text,
  bucket_start        timestamp with time zone,
  amount_type         text,
  amount_value        bigint,
  PRIMARY KEY(instance_id, bucket_start, amount_type)
);
