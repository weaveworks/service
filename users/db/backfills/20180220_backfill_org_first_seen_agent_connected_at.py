# Backfill to be run inside cluster due to database VPC
# 0. Modify script settings to match dev or prod
# 1. kubectl --kubeconfig ansible/prod/kubeconfig run "users-backfill" --image=google/cloud-sdk:latest --stdin --tty --restart=Never /bin/sh
# 2. pip install google-cloud-bigquery psycopg2
# 3. gcloud beta auth application-default login
# 4. python ./backfill.py
# 5. kubectl --kubeconfig ansible/prod/kubeconfig delete pod "users-backfill"

from google.cloud import bigquery
import psycopg2
from datetime import datetime

# Users database
DB_HOST = ""
DB_NAME = "users_vpc"
DB_USER = "postgres"
DB_PASS = "" # https://github.com/weaveworks/service-conf/blob/master/infra/prod/tfstate

BQ_DATASET=""

# BigQuery query. Requires Google Cloud application default credentials setup locally.
FIRST_CONNECTED_QUERY = """
SELECT org_id, SUBSTR(event, 6, 4) as path, MIN(day) as first
    FROM {dataset}.events_aggregated
    WHERE event = "/api/report" OR event = "/api/prom/push" OR event LIKE "/api/flux/v_/daemon"
    GROUP BY org_id, path
"""
SET_FIRST_CONNECTED_SQL = "UPDATE organizations SET first_seen_{agent}_connected_at = %s WHERE id = %s"

def event_agent(event):
    if 'flux' == event:
        return 'flux'
    elif 'prom' == event:
        return 'prom'
    elif 'repo' == event:
        return 'scope'

def run_backfill():
    client = bigquery.Client(project='weaveworks-bi')
    query_job = client.query(FIRST_CONNECTED_QUERY.format(dataset=BQ_DATASET))

    print("Running BigQuery query...")
    results = list(query_job.result(timeout=120))
    print("BigQuery query complete.")

    print("Connecting to postgres database...")
    dsn = "host={} dbname={} user={} password={}".format(DB_HOST, DB_NAME, DB_USER, DB_PASS)
    conn = psycopg2.connect(dsn)
    with conn:
        with conn.cursor() as cur:
            print("Running backfill...")
            for org_id, path, first_day in results:
                agent = event_agent(path)
                if agent and org_id and path and first_day:
                    first_timestamp = datetime.combine(first_day, datetime.min.time())
                    print("Setting orgID '{}' first_seen_{}_connected_at to '{}'".format(org_id, agent, first_timestamp))
                    cur.execute(SET_FIRST_CONNECTED_SQL.format(agent=agent), (first_timestamp, org_id))
    conn.close()
    print("Backfill complete.")

if __name__ == "__main__":
    run_backfill()
