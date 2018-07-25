import sys
import argparse
import json
import signal
import logging
from threading import Event, Thread

from prometheus_client import start_http_server
from google.oauth2 import service_account

from .checks.access import AccessCheck
from .checks.usage import UsageCheck


def parse_args(args):
    parser = argparse.ArgumentParser('billing-exporter')
    parser.add_argument(
        '--bigquery-creds-file', required=True, type=str)
    parser.add_argument(
        '--users-db-creds-file', required=True, type=argparse.FileType('r'))
    parser.add_argument(
        '--billing-db-creds-file', required=True, type=argparse.FileType('r'))
    parser.add_argument(
        '--zuora-creds-file', required=True, type=argparse.FileType('r'))
    parser.add_argument(
        '--production', type=bool, default=False)
    parser.add_argument(
        '--check-instance-id', type=str, nargs='+')
    return parser.parse_args(args)


def load_json_file(fh):
    with fh:
        return json.load(fh)


def run_checker(checker, *args, **kwargs):
    def _target():
        try:
            checker.run(*args, **kwargs)
        except Exception as e:
            print(e)
            raise

    t = Thread(target=_target)
    t.start()


def main(args):
    logging.basicConfig()

    opts = parse_args(args)

    bq_creds = service_account.Credentials.from_service_account_file(
        opts.bigquery_creds_file)

    should_stop = Event()
    def signal_handler():
        should_stop.set()

    signal.signal(signal.SIGINT, signal_handler)

    run_checker(
        AccessCheck(opts.production),
        should_stop,
        bq_creds)
    run_checker(
        UsageCheck(opts.production),
        should_stop,
        opts.check_instance_id,
        bq_creds,
        load_json_file(opts.users_db_creds_file),
        load_json_file(opts.billing_db_creds_file),
        load_json_file(opts.zuora_creds_file))

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
