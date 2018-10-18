from datetime import date, datetime, timezone
import logging
from collections import defaultdict, namedtuple

from dateutil import relativedelta
from google.oauth2 import service_account
from google.cloud.bigquery import Client as BigQueryClient
import psycopg2

from exporter.billing import bigquery
from exporter.billing import db
from exporter.billing import zuora
from exporter.checks.usage import get_zuora_aggregates
from exporter.billing.utils import datetime_ceil_date, datetime_floor_date, daterange, UriParamType, inject_password_from_file, all_equal

_LOG = logging.getLogger(__name__)


def discrepancy_report(billing_db_uri, users_db_uri, zuora_uri, bq_creds,
                       production, datetime_in_month):
    today = datetime_floor_date(datetime.utcnow()).replace(tzinfo=timezone.utc)

    start = datetime_in_month.replace(day=1, tzinfo=timezone.utc)
    _LOG.debug('Billing discrepancy report for month of', start)

    end = start + relativedelta.relativedelta(months=1)

    if end > today:
        _LOG.debug('Stopping report at', today)
        end = today

    bq_client = BigQueryClient(project='weaveworks-bi', credentials=bq_creds)
    zuora_client = zuora.Zuora(zuora_uri)

    _LOG.debug('Loading metadata for all instances')
    orgs = load_orgs(users_db_uri)

    _LOG.debug('Gathering usage data')
    usage_by_platform = gather_usage(billing_db_uri, bq_client, zuora_client,
                                     orgs, start, end, production)

    _LOG.debug('Checking for discrepancies')
    return find_discrepancies(orgs, usage_by_platform, start, end)


def load_orgs(users_db_uri):
    with psycopg2.connect(dsn=users_db_uri) as users_conn:
        return [
            o for o in db.get_org_details(users_conn) if o.zuora_account_number
        ]


def gather_usage(billing_db_uri, bq_client, zuora_client, orgs, start, end,
                 production):
    with psycopg2.connect(dsn=billing_db_uri) as billing_conn:
        return {
            'bigquery':
            bigquery.get_daily_aggregates(bq_client, production, orgs, start,
                                          end),
            'db':
            db.get_daily_aggregates(billing_conn, orgs, start, end),
            'zuora':
            get_zuora_aggregates(zuora_client, orgs, start, end),
        }


DiscrepancyReport = namedtuple('DiscrepancyReport', ('org', 'days', 'total'))


def find_discrepancies(orgs, usage_by_platform, start, end):
    sources = ('bigquery', 'db', 'zuora')

    aggs_index = defaultdict(lambda: defaultdict(dict))
    for source, aggregates in usage_by_platform.items():
        for org, day, unit, value in aggregates:
            if unit == 'node-seconds':
                aggs_index[org][day][source] = value

    for org in orgs:
        discrepancies = []
        totals = defaultdict(int)
        for d in daterange(start, end):
            day_data = aggs_index[org][d]
            if not day_data:
                continue

            for source, value in day_data.items():
                totals[source] += value
            if not all_equal(day_data.get(s, 0) for s in sources):
                discrepancies.append((d, day_data))

        if discrepancies:
            yield DiscrepancyReport(org, tuple(discrepancies), dict(totals))
