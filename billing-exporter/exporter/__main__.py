import sys
import json
import signal
import logging
from threading import Event, Thread
from urllib.parse import urlparse

from prometheus_client import start_http_server
from google.oauth2 import service_account
import click

from .checks.access import AccessCheck
from .checks.usage import UsageCheck


_LOG = logging.getLogger(__name__)


class UriParamType(click.ParamType):
    name = 'uri'

    def convert(self, value, param, ctx):
        try:
            return urlparse(value)
        except ValueError as e:
            self.fail(f'{value!r} is not a valid URL: {e}', param, ctx)


def inject_password_from_file(name, uri, password_file):
    if not password_file:
        return uri

    if uri.password:
        _LOG.warn('Password for %s specified in both URI and password file', name)
    with password_file as fh:
        password = fh.read()

    netloc = ''
    if uri.username:
        netloc += uri.username
    netloc += f':{password}@{uri.hostname}'
    if uri.port:
        netloc += f':{uri.port}'
    return uri._replace(netloc=netloc)


def run_checker(checker, *args, **kwargs):
    def _target():
        try:
            checker.run(*args, **kwargs)
        except Exception as e:
            _LOG.exception("Unhandled exception in checker %s", checker)
            raise

    t = Thread(target=_target)
    t.start()


@click.command()
@click.option('--billing-database-uri', type=UriParamType())
@click.option('--billing-database-password-file', type=click.File(), default=None)
@click.option('--users-database-uri', type=UriParamType())
@click.option('--users-database-password-file', type=click.File(), default=None)
@click.option('--zuora-uri', type=UriParamType())
@click.option('--zuora-password-file', type=click.File(), default=None)
@click.option('--bigquery-creds-file', type=click.Path())
@click.option('--production', is_flag=True, flag_value=True, default=False)
@click.option('--check-instance-id', 'check_instance_ids', multiple=True)
def main(
        billing_database_uri, billing_database_password_file, 
        users_database_uri, users_database_password_file,
        zuora_uri, zuora_password_file, bigquery_creds_file,
        production, check_instance_ids):
    logging.basicConfig()

    should_stop = Event()
    def signal_handler():
        should_stop.set()

    signal.signal(signal.SIGINT, signal_handler)

    billing_db_uri = inject_password_from_file(
        'billing', billing_database_uri, billing_database_password_file).geturl()
    users_db_uri = inject_password_from_file(
        'users', users_database_uri, users_database_password_file).geturl()
    zuora_uri = inject_password_from_file(
        'zuora', zuora_uri, zuora_password_file).geturl()

    bq_creds = service_account.Credentials.from_service_account_file(
        bigquery_creds_file)

    run_checker(
        AccessCheck(production),
        should_stop,
        bq_creds)
    run_checker(
        UsageCheck(production),
        should_stop,
        check_instance_ids,
        bq_creds,
        users_db_uri,
        billing_db_uri,
        zuora_uri)

    # Start up the server to expose the metrics.
    start_http_server(8000)

    try:
        should_stop.wait()
    except:
        should_stop.set()
    else:
        should_stop.set()


if __name__ == '__main__':
    main(sys.argv[1:])
