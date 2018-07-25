#!/usr/bin/env python3
'''
Our RDS instances are NOT publicly available, therefore, this script should be run from within our cluster.
Usage:
    Within cluster:
    $ ./billing/billing_aggregates_vs_gcp_upload.py prod loggly/loggly_events_2018-07-03 17_43_54.249652.csv
    Outside cluster:
    $ kubectl cp <pod>:/billing_aggregates_vs_gcp_uploads.json incident-2018-06-18/billing_aggregates_vs_gcp_uploads.json

NB: This script is kind of broken because I transplanted it without replacing the config mechanism
'''
import datetime
from collections import OrderedDict
from itertools import groupby
from operator import itemgetter
from sys import argv
import json
import logging

import psycopg2
import dateutil.parser

from config import CONFIG

LOGGER = logging.getLogger('billing_aggregates_vs_gcp_upload')
IDX_ID = 0
IDX_BUCKET = 1
IDX_AMOUNT = 2
DATETIME_FORMAT = '%Y-%m-%d %H:%M:%S%z'


def main(env, filepath='../loggly_events_2018-07-03 17_43_54.249652.csv'):
    logging.basicConfig()
    with open(filepath, 'r') as f:
        gcp_uploads = [line.strip().split('\t') for line in f.readlines()]
        LOGGER.info('read %s GCP uploads', len(gcp_uploads))
        buckets = sorted(set(map(itemgetter(IDX_BUCKET), gcp_uploads)))
        t_start = dateutil.parser.parse(buckets[0])
        # WARNING: usage typically carries over the next hour, hence the latest bucket
        # is very likely to have discrepancies which will be resolved in an hour:
        t_end = dateutil.parser.parse(buckets[-1])
        LOGGER.info('start: %s ; end: %s', t_start, t_end)

    config = CONFIG[env]
    with psycopg2.connect(**config['users-db']) as users_conn:
        gcp_instances = read_gcp_instances(users_conn)
        consumer_ids = set(gcp_instances.values())
        LOGGER.info('read %s GCP instances', len(gcp_instances))
    with psycopg2.connect(**config['billing-db']) as billing_conn:
        aggregates = read_aggregates(billing_conn, gcp_instances.keys(), t_start.strftime(DATETIME_FORMAT), t_end.strftime(DATETIME_FORMAT))
        LOGGER.info('read %s aggregates', len(aggregates))

    report = {}

    LOGGER.info('group and add billing DB\'s usages to the report')
    for key, aggregates_iter in groupby(aggregates, key=lambda aggregate: (aggregate[IDX_ID], aggregate[IDX_BUCKET])):
        instance_id, bucket = key
        consumer_id = gcp_instances.get(instance_id)
        if not consumer_id:
            LOGGER.warning('skipped %s (no matching consumer ID)', instance_id)
            continue
        if consumer_id not in report:
            report[consumer_id] = {}
        bucket = str(bucket)
        if bucket not in report[consumer_id]:
            report[consumer_id][bucket] = {}
        report[consumer_id][bucket]['usage'] = sum(int(aggregate[IDX_AMOUNT]) for aggregate in aggregates_iter)

    LOGGER.info('group and add GCP uploads to the report')
    for key, gcp_uploads_iter in groupby(gcp_uploads, key=lambda gcp_upload: (gcp_upload[IDX_ID], gcp_upload[IDX_BUCKET])):
        consumer_id, bucket = key
        if consumer_id not in consumer_ids:
            LOGGER.warning('skipped %s (no matching consumer ID in billing DB)', instance_id)
            continue
        if consumer_id not in report:
            report[consumer_id] = {}
        bucket = str(dateutil.parser.parse(bucket))
        if bucket not in report[consumer_id]:
            report[consumer_id][bucket] = {}
        report[consumer_id][bucket]['uploaded'] = sum(int(gcp_upload[IDX_AMOUNT]) for gcp_upload in gcp_uploads_iter)

    LOGGER.info('sort report by bucket, for readability')
    for consumer_id, usage in report.items():
        report[consumer_id] = OrderedDict(sorted(usage.items()))

    LOGGER.info('export report as JSON file')
    with open('billing_aggregates_vs_gcp_uploads.json', 'w') as f:
        json.dump(report, f)

    LOGGER.info('done')


def read_aggregates(conn, instance_ids, start, end):
    with conn.cursor() as cur:
        cur.execute(
            '''
            SELECT instance_id, bucket_start, amount_value
            FROM aggregates
            WHERE bucket_start >= '%s'
            AND bucket_start <= '%s'
            AND amount_type = 'node-seconds'
            AND instance_id IN (%s)
            ORDER BY instance_id ASC, bucket_start DESC;
            ''' % (start, end, "'%s'" % "','".join(map(str, instance_ids)))
        )
        return cur.fetchall()


def read_gcp_instances(conn):
    with conn.cursor() as cur:
        cur.execute(
            '''
            SELECT o.id, g.consumer_id
            FROM organizations AS o, gcp_accounts AS g
            WHERE o.gcp_account_id IS NOT NULL
            AND o.gcp_account_id != ''
            AND o.gcp_account_id = g.id;
            '''
        )
        return {iid: cid for iid, cid in cur.fetchall() if iid and cid}


if __name__ == '__main__':
    main(argv[1], argv[2])
