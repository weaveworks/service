import sys
import json
import signal
import logging
from threading import Event, Thread
from urllib.parse import urlparse

from prometheus_client import start_http_server, make_wsgi_app
from google.oauth2 import service_account
import click
from werkzeug.serving import run_simple
from werkzeug.middleware.dispatcher import DispatcherMiddleware
from flask import Flask

from .checks.access import AccessCheck
from .checks.usage import UsageCheck
from .billing.utils import UriParamType, inject_password_from_file
from .app import app

_LOG = logging.getLogger(__name__)


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
@click.option('--billing-database-uri', type=UriParamType(), required=True)
@click.option(
    '--billing-database-password-file', type=click.File(), default=None)
@click.option('--users-database-uri', type=UriParamType(), required=True)
@click.option(
    '--users-database-password-file', type=click.File(), default=None)
@click.option('--zuora-uri', type=UriParamType(), required=True)
@click.option('--zuora-password-file', type=click.File(), default=None)
@click.option('--bigquery-creds-file', type=click.Path(), required=True)
@click.option('--production', is_flag=True, flag_value=True, default=False)
@click.option('--check-instance-id', 'check_instance_ids', multiple=True)
def main(billing_database_uri, billing_database_password_file,
         users_database_uri, users_database_password_file, zuora_uri,
         zuora_password_file, bigquery_creds_file, production,
         check_instance_ids):
    config = locals()
    logging.basicConfig()

    should_stop = Event()

    def signal_handler():
        should_stop.set()

    signal.signal(signal.SIGINT, signal_handler)

    billing_db_uri = inject_password_from_file(
        'billing', billing_database_uri,
        billing_database_password_file).geturl()
    users_db_uri = inject_password_from_file(
        'users', users_database_uri, users_database_password_file).geturl()
    zuora_uri = inject_password_from_file('zuora', zuora_uri,
                                          zuora_password_file).geturl()

    bq_creds = service_account.Credentials.from_service_account_file(
        bigquery_creds_file)

    run_checker(AccessCheck(production), should_stop, bq_creds)
    run_checker(
        UsageCheck(production), should_stop, check_instance_ids, bq_creds,
        users_db_uri, billing_db_uri, zuora_uri)

    app.config.update(
        billing_db_uri=billing_db_uri,
        users_db_uri=users_db_uri,
        zuora_uri=zuora_uri,
        bq_creds=bq_creds,
        production=production
    )
    app.debug = True

    try:
        run_simple(
            '0.0.0.0',
            8000,
            DispatcherMiddleware(app, {'/metrics': make_wsgi_app()}),
            use_reloader=False,
            use_debugger=app.debug,
            use_evalex=app.debug)
    finally:
        try:
            should_stop.wait()
        except:
            should_stop.set()
        else:
            should_stop.set()


if __name__ == '__main__':
    main(sys.argv[1:])
