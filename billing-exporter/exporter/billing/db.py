from .model import Org


def get_org_details(conn, orgs):
    if len(orgs) == 1:
        # Hack to avoid invalid SQL: (foo)
        orgs = list(orgs) + list(orgs)

    with conn.cursor() as cur:
        cur.execute('''
            SELECT external_id, id, trial_expires_at, zuora_account_number, gcp_account_id
            FROM organizations WHERE external_id IN ({})'''.format(
                ','.join('{!r}'.format(o) for o in orgs)
            ))
        return [Org(row[0], str(row[1]), row[2], row[3], row[4]) for row in cur.fetchall()]


def get_daily_aggregates(conn, orgs, start, end):
    if not orgs:
        return []

    with conn.cursor() as cur:
        q = '''
        WITH instances (instance_id, trial_end) AS (
            VALUES {orgs}
        )
        SELECT aggregates.instance_id, date_trunc('day', bucket_start) as day, amount_type, SUM(amount_value) as total
        FROM aggregates
        JOIN instances ON CAST(aggregates.instance_id AS INTEGER) = instances.instance_id
        WHERE amount_type = 'node-seconds'
        AND bucket_start >= {start!r}
        AND bucket_start < {end!r}
        AND instances.instance_id IS NOT NULL
        AND bucket_start > instances.trial_end::timestamp
        GROUP BY aggregates.instance_id, day, amount_type
        ORDER BY aggregates.instance_id ASC, day DESC, amount_type ASC;
        '''.format(
            start=start.isoformat(),
            end=end.isoformat(),
            orgs=', '.join(
                '({}, {!r})'.format(o.internal_id, o.trial_expires_at.isoformat())
                for o in orgs
            )
        )
        cur.execute(q)
        orgs_by_id = {org.internal_id: org for org in orgs}
        return [
            (orgs_by_id[row[0]], row[1], row[2], int(row[3]))
            for row in cur.fetchall()
        ]
