import time
import psycopg2
from datetime import date, datetime, timedelta, timezone
from itertools import groupby

from prometheus_client import Gauge
from google.cloud.bigquery import Client as BigQueryClient

from ..billing import bigquery
from ..billing import db
from ..billing import zuora
from ..billing.utils import datetime_ceil_date, datetime_floor_date


class UsageCheck(object):
    def __init__(self, production):
        self.production = production
        self.usage_gauge = Gauge('billable_usage_recorded', 'Billable usage recorded from different sources', ['source', 'instance', 'days_ago', 'unit'])
        self.usage_check_time_gauge = Gauge('billable_usage_recorded_check_time', 'The time we last checked the billable usage')

    def run(self, stop_event, org_external_ids, bq_creds, users_db_creds, billing_db_creds, zuora_creds):
        bq_client = BigQueryClient(project='weaveworks-bi', credentials=bq_creds)
        zuora_client = zuora.Zuora(**zuora_creds)

        while not stop_event.is_set():
            now = datetime.utcnow().replace(tzinfo=timezone.utc)
            today = datetime_floor_date(now)
            start = datetime_floor_date(now - timedelta(days=2))
            end = datetime_ceil_date(now  - timedelta(days=1))

            with psycopg2.connect(**users_db_creds) as users_conn:
                internal_ids = db.get_internal_ids(users_conn, org_external_ids)
            external_ids = {i: e for e, i in internal_ids.items()}

            with psycopg2.connect(**billing_db_creds) as billing_conn:
                aggregates_by_source = {
                    'bigquery': bigquery.get_daily_aggregates(bq_client, self.production, internal_ids.values(), start, end),
                    'db': db.get_daily_aggregates(billing_conn, internal_ids.values(), start, end),
                    'zuora': get_zuora_aggregates(zuora_client, None, internal_ids.values(), start, end),
                }

            for source, aggregates in aggregates_by_source.items():
                for internal_id, day, unit, value in aggregates:
                    self.usage_gauge.labels(
                        source=source,
                        instance=external_ids[internal_id],
                        days_ago=(today - day).days,
                        unit=unit
                        ).set(value)

            self.usage_check_time_gauge.set(time.time())

            stop_event.wait(60 * 60 * 24) # it's only worth checking once a day

    
def get_zuora_aggregates(zuora_client, users_db_conn, internal_ids, start, end):
    # zuora_accounts = db.get_zuora_accounts(users_db_conn, internal_ids)
    zuora_accounts = {'2': 'Webb5831f5b6f998e183fcec5792a778'}

    return [
        (internal_id, date, 'node-seconds', sum(usage['quantity'] for usage in usage_by_day))
        for internal_id, account_id in sorted(zuora_accounts.items())
        for date, usage_by_day in groupby(
            (
                usage
                for usage in zuora_client.get_usage(account_id, start, end)
                if usage['unitOfMeasure'] == 'node-seconds'
            ),
            lambda usage:  datetime_floor_date(zuora.parse_datetime(usage['startDateTime'])))
    ]
