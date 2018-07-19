from datetime import timedelta

from google.cloud.bigquery import QueryJobConfig, ArrayQueryParameter, ScalarQueryParameter

from .utils import datetime_ceil_date, datetime_floor_date

QUERY = '''
#standardSQL
SELECT
  internal_instance_id as instance_id,
  TIMESTAMP_TRUNC(received_at, DAY, 'UTC') AS day,
  amount_type,
  SUM(amount_value) as amount_value
FROM
  billing{dataset_suffix}.events
WHERE
  received_at IS NOT NULL
  AND received_at >= @start_time
  AND received_at < @end_time
  AND _PARTITIONDATE >= @partition_start
  AND _PARTITIONDATE < @partition_end
  AND internal_instance_id IN UNNEST(@instance_ids)
GROUP BY
  instance_id,
  day,
  amount_type
ORDER BY
  instance_id ASC, day DESC, amount_type ASC
'''


def get_daily_aggregates(client, production, orgs, start, end):
    q = QUERY.format(dataset_suffix='' if production else '_dev')

    query_cfg = QueryJobConfig()
    query_cfg.query_parameters = [
        ScalarQueryParameter('start_time', 'TIMESTAMP', start),
        ScalarQueryParameter('end_time', 'TIMESTAMP', end),
        ScalarQueryParameter('partition_start', 'DATE', datetime_floor_date(start).date()),
        # Extend the partition into the next day so we catch any late usage
        ScalarQueryParameter('partition_end', 'DATE', datetime_ceil_date(end).date() + timedelta(days=1)),
        ArrayQueryParameter('instance_ids', 'STRING', orgs),
    ]
    query_cfg.use_query_cache = True

    query_job = client.query(q, job_config=query_cfg)
    # A dry run query completes immediately.
    orgs_by_id = {org.internal_id: org for org in orgs}
    for row in query_job.result():
        yield orgs_by_id[row[0]], row[1], row[2], row[3]