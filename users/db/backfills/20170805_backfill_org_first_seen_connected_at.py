from google.cloud import bigquery
import psycopg2

# Users database
DB_HOST = "prod-users-vpc-database.cqqzyyx2xnct.us-east-1.rds.amazonaws.com"
DB_NAME = "users"
DB_USER = "postgres"
DB_PASS = "" # https://github.com/weaveworks/service-conf/blob/master/infra/prod/tfstate

# BigQuery query. Requires Google Cloud application default credentials setup locally.
FIRST_CONNECTED_QUERY = """
SELECT org_id, MIN(dt) as first
    FROM service.events
    WHERE event = "/api/report" OR event = "/api/prom/push" OR event LIKE "/api/flux/v_/daemon"
    GROUP BY org_id
"""

def run_backfill():
    client = bigquery.Client(project='weaveworks-bi')
    query = client.run_sync_query(FIRST_CONNECTED_QUERY)
    query.timeout_ms = 120 * 1000

    print("Running BigQuery query...")
    query.run()
    assert query.complete
    print("BigQuery query complete.")

    print("Connecting to postgres database...")
    conn_string = "host={} dbname={} user={} password={}".format(DB_HOST, DB_NAME, DB_USER, DB_PASS)
    with psycopg2.connect(conn_string) as conn:
        with conn.cursor() as cur:
            print("Running backfill...")
            for org_id, first in query.fetch_data():
                if first:
                    print("Setting orgID '{}' first_seen_connected_at to '{}'".format(org_id, first))
                    sql = "UPDATE organizations SET first_seen_connected_at = '%s' WHERE id = '%s'"
                    cur.execute(sql, (first, org_id))
            print("Committing...")
            conn.commit()
    print("Backfill complete.")

if __name__ == "__main__":
    run_backfill()
