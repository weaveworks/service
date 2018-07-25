import time
import psycopg2
from datetime import date, datetime, timedelta, timezone
from itertools import groupby
import logging

from prometheus_client import Gauge
from google.cloud.bigquery import Client as BigQueryClient

from ..billing import bigquery
from ..billing import db
from ..billing import zuora
from ..billing.utils import datetime_ceil_date, datetime_floor_date


_LOG = logging.getLogger(__name__)


class UsageCheck(object):
    def __init__(self, production):
        self.production = production
        self.usage_gauge = Gauge('billable_usage_recorded', 'Billable usage recorded from different sources', ['source', 'instance', 'days_ago', 'unit'])
        self.usage_check_time_gauge = Gauge('billable_usage_recorded_check_time', 'The time we last checked the billable usage')

    def run(self, stop_event, org_external_ids, bq_creds, users_db_creds, billing_db_creds, zuora_creds):
        bq_client = BigQueryClient(project='weaveworks-bi', credentials=bq_creds)
        zuora_client = zuora.Zuora(**zuora_creds)

        while not stop_event.is_set():
            try:
                self.check_usage(org_external_ids, bq_client, zuora_client, users_db_creds, billing_db_creds)
            except Exception as e:
                _LOG.exception('Error while checking usage')
                self.usage_check_time_gauge.set(0)

            stop_event.wait(60 * 60 * 24) # it's only worth checking once a day

    def check_usage(self, org_external_ids, bq_client, zuora_client, users_db_creds, billing_db_creds):
        now = datetime.utcnow().replace(tzinfo=timezone.utc)
        today = datetime_floor_date(now)
        start = datetime_floor_date(now - timedelta(days=2))
        end = datetime_ceil_date(now  - timedelta(days=1))

        with psycopg2.connect(**users_db_creds) as users_conn:
            orgs = db.get_org_details(users_conn, org_external_ids)

        with psycopg2.connect(**billing_db_creds) as billing_conn:
            aggregates_by_source = {
                'bigquery': bigquery.get_daily_aggregates(bq_client, self.production, orgs, start, end),
                'db': db.get_daily_aggregates(billing_conn, orgs, start, end),
                'zuora': get_zuora_aggregates(zuora_client, orgs, start, end),
            }

        for source, aggregates in aggregates_by_source.items():
            for org, day, unit, value in aggregates:
                self.usage_gauge.labels(
                        source=source,
                        instance=org.external_id,
                        days_ago=(today - day).days,
                        unit=unit
                    ).set(value)

        self.usage_check_time_gauge.set(time.time())

def get_zuora_aggregates(zuora_client, orgs, start, end):
    return [
        (org, date, 'node-seconds', sum(usage['quantity'] for usage in usage_by_day))
        for org in orgs
        for date, usage_by_day in groupby(
            (
                usage
                for usage in zuora_client.get_usage(org.zuora_account_id, start, end)
                if usage['unitOfMeasure'] == 'node-seconds'
            ),
            lambda usage: datetime_floor_date(zuora.parse_datetime(usage['startDateTime'])))
    ]
