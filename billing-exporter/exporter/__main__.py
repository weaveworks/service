import sys
import argparse
import json
from threading import Event, Thread

from prometheus_client import start_http_server
from google.oauth2 import service_account

from .checks.access import AccessCheck
from .checks.usage import UsageCheck


def parse_args(args):
    parser = argparse.ArgumentParser('billing-exporter')
    parser.add_argument('--bigquery-creds', nargs='?', type=argparse.FileType('r'))
    parser.add_argument('--users-db-creds', required=True, type=argparse.FileType('r'))
    parser.add_argument('--billing-db-creds', required=True, type=argparse.FileType('r'))
    parser.add_argument('--zuora-creds', required=True, type=argparse.FileType('r'))
    parser.add_argument('--production', type=bool, default=False)
    parser.add_argument('--check-instance-id', type=str, nargs='+')
    
    return parser.parse_args(args)

def load_json_file(fh):
    with fh:
        return json.load(fh)

def main(args):
    opts = parse_args(args)

    if opts.bigquery_creds:
        bq_creds = service_account.Credentials.from_service_account_file(
            opts.bigquery_creds)
    else:
        bq_creds = None

    zuora_creds = load_json_file(opts.zuora_creds)

    should_stop = Event()
    # Start up the server to expose the metrics.
    start_http_server(8000)

    for target, args in (
            (AccessCheck(opts.production).run, (should_stop, bq_creds)),
            (UsageCheck(opts.production).run,
             (should_stop, opts.check_instance_id, bq_creds, load_json_file(opts.users_db_creds), load_json_file(opts.billing_db_creds), load_json_file(opts.zuora_creds)))):
        t = Thread(target=target, args=args)
        t.start()

    try:
        should_stop.wait()
    except:
        should_stop.set()

if __name__ == '__main__':
    main(sys.argv[1:])
