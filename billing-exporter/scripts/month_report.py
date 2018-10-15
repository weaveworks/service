from datetime import date, datetime, timezone
import logging
from collections import defaultdict

from dateutil import relativedelta
import click
from google.oauth2 import service_account
from google.cloud.bigquery import Client as BigQueryClient
import psycopg2

from exporter.billing import bigquery
from exporter.billing import db
from exporter.billing import zuora
from exporter.checks.usage import get_zuora_aggregates
from exporter.billing.utils import datetime_ceil_date, datetime_floor_date, daterange, UriParamType, inject_password_from_file, all_equal


_LOG = logging.getLogger(__name__)


@click.command()
@click.option('--billing-database-uri', type=UriParamType())
@click.option('--billing-database-password-file', type=click.File(), default=None)
@click.option('--users-database-uri', type=UriParamType())
@click.option('--users-database-password-file', type=click.File(), default=None)
@click.option('--zuora-uri', type=UriParamType())
@click.option('--zuora-password-file', type=click.File(), default=None)
@click.option('--bigquery-creds-file', type=click.Path())
@click.option('--production', is_flag=True, flag_value=True, default=False)
@click.option('--date-in-month', type=click.DateTime(formats=('%Y-%m-%d', )))
def main(
        billing_database_uri, billing_database_password_file, 
        users_database_uri, users_database_password_file,
        zuora_uri, zuora_password_file, bigquery_creds_file,
        production, date_in_month):
    logging.basicConfig()

    today = datetime_floor_date(datetime.utcnow()).replace(tzinfo=timezone.utc)

    start = date_in_month.replace(day=1,tzinfo=timezone.utc)
    print('Billing discrepancy report for month of', start)

    end = start + relativedelta.relativedelta(months=1)

    if end > today:
        print('Stopping report at', today)
        end = today

    billing_db_uri = inject_password_from_file(
        'billing', billing_database_uri, billing_database_password_file).geturl()
    users_db_uri = inject_password_from_file(
        'users', users_database_uri, users_database_password_file).geturl()
    zuora_uri = inject_password_from_file(
        'zuora', zuora_uri, zuora_password_file).geturl()
    bq_creds = service_account.Credentials.from_service_account_file(
        bigquery_creds_file)

    bq_client = BigQueryClient(project='weaveworks-bi', credentials=bq_creds)
    zuora_client = zuora.Zuora(zuora_uri)
    
    print('Loading metadata for all instances')

    with psycopg2.connect(dsn=users_db_uri) as users_conn:
        orgs = [
            o
            for o in db.get_org_details(users_conn)
            if o.zuora_account_number
        ]

    print('Gathering usage data')

    with psycopg2.connect(dsn=billing_db_uri) as billing_conn:
        aggregates_by_source = {
            'bigquery': bigquery.get_daily_aggregates(bq_client, production, orgs, start, end),
            'db': db.get_daily_aggregates(billing_conn, orgs, start, end),
            'zuora': get_zuora_aggregates(zuora_client, orgs, start, end),
        }
    
    print('Checking for discrepancies')
    sources = ('bigquery', 'db', 'zuora')

    aggs_index = defaultdict(lambda: defaultdict(dict))
    for source, aggregates in aggregates_by_source.items():
        for org, day, unit, value in aggregates:
            if unit == 'node-seconds':
                aggs_index[org][day][source] = value

    discrepancies = defaultdict(dict)
    for org in orgs:
        totals = defaultdict(int)
        for d in daterange(start, end):
            day_data = aggs_index[org][d]
            if not day_data:
                continue

            for source, value in day_data.items():
                totals[source] += value
            if not all_equal(day_data.get(s, 0) for s in sources):
                discrepancies[org][d] = day_data

        if totals and (discrepancies[org] or not all_equal(totals.get(s, 0) for s in sources)):
            discrepancies[org][None] = totals

    def fmtDelta(amounts, k1, k2, includeZeros=False):
        i = amounts.get(k2, 0) - amounts.get(k1, 0)
        if i > 0:
            return f'+{i}'
        elif i == 0 and not includeZeros:
            return ''
        return str(i)

    print('Discrepancy report:')
    print('NB: Trial start date and instance deletion are not yet fully accounted for')
    print()
    print('instance    \tdate      \tbigquery  \t   +/-    \t    db    \t   +/-    \t   zuora  ')
    for org, org_disc in discrepancies.items():
        if not org_disc:
            continue

        rest = dict(org_disc)
        try:
            totals = rest.pop(None)
        except KeyError:
            total_item = []
        else:
            total_item = [(None, totals)]
        items = list(sorted(rest.items())) + total_item

        rjust = lambda i: str(i).rjust(9)

        for d, amounts in items:
            print(
                '\t'.join((
                    org.external_id,
                    ('total' if d is None else str(d.date())).ljust(10),
                    rjust(amounts.get('bigquery', 0)),
                    rjust(fmtDelta(amounts, 'bigquery', 'db', includeZeros=d is None)),
                    rjust(amounts.get('db', 0)),
                    rjust(fmtDelta(amounts, 'db', 'zuora', includeZeros=d is None)),
                    rjust(amounts.get('zuora', 0))
                ))
            )
        
        print()


if __name__ == '__main__':
    main()
