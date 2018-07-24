from datetime import date, datetime, timedelta
import time
import logging

from google.cloud import bigquery
from prometheus_client import Gauge

_LOG = logging.getLogger(__name__)

_QUERY = '''
#standardSQL
WITH instances_daily_access AS (
  SELECT 
    _PARTITIONDATE as pd,
    ID as internal_instance_id,
    MIN(RefuseDataUpload) as RefuseDataUpload -- If it was ever false in a 24h window, then count the whole day as false
  FROM service{dataset_suffix}.instances
  WHERE _PARTITIONDATE = @day
  GROUP BY pd, internal_instance_id
  ORDER BY internal_instance_id
)
-- Look for instances which were both denied access, and also appear in logged events
SELECT pd, internal_instance_id FROM instances_daily_access WHERE RefuseDataUpload = true
INTERSECT DISTINCT
SELECT 
  _PARTITIONDATE as pd,
  internal_instance_id
FROM billing{dataset_suffix}.events
WHERE _PARTITIONDATE = @day
GROUP BY _PARTITIONDATE, internal_instance_id
ORDER BY internal_instance_id
'''


class AccessCheck(object):
    def __init__(self, production):
        self.production = production
        self.violation_gauge = Gauge('instance_denial_violations', 'Number of instances violating access denial policies')
        self.violation_check_time_gauge = Gauge('instance_denial_violations_check_time', 'The time we last checked the number of instances violating access denial policies')

    def run(self, stop_event, bq_creds):
        bq_client = bigquery.Client(project='weaveworks-bi', credentials=bq_creds)
        while not stop_event.is_set():
            try:
                self.check_access(bq_client)
            except Exception as e:
                _LOG.exception('Error while checking access')
                self.violation_check_time_gauge.set(0)
            
            stop_event.wait(60 * 60 * 24) # it's only worth checking once a day
    
    def check_access(self, bq_client):
        day_to_check = datetime.utcnow().date() - timedelta(days=1)
        mistakes = self.query_access_log(bq_client, day_to_check)

        if mistakes:
            print(f'Detected access denial violations for {len(mistakes)} instances. IDs: {mistakes!r}')
        self.violation_gauge.set(len(mistakes))
        self.violation_check_time_gauge.set(time.time())

    def query_access_log(self, bq_client, date):
        q = _QUERY.format(dataset_suffix='' if self.production else '_dev')
        
        query_cfg = bigquery.QueryJobConfig()
        query_cfg.query_parameters = [
          bigquery.ScalarQueryParameter('day', 'DATE', date)
        ]
        query_cfg.use_query_cache = True

        query_job = bq_client.query(q, query_cfg)

        return tuple(
          internal_instance_id
          for date, internal_instance_id in query_job.result()
        )