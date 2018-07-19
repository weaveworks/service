from .model import Org


def get_org_details(conn, orgs):
    with conn.cursor() as cur:
        cur.execute(
            'SELECT external_id, id, trial_expires_at, zuora_account_number FROM organizations WHERE external_id IN ({})'.format(
                ','.join(orgs)
            ))
        return [Org(row[0], str(row[1]), row[2], row[3]) for row in cur.fetchall()]


def get_daily_aggregates(conn, orgs, start, end):
    with conn.cursor() as cur:
        q = '''
        SELECT instance_id, DATE(bucket_start) as day, amount_type, SUM(amount_value) as total
        FROM aggregates
        AND bucket_start >= {start!r}
        AND bucket_start < {end!r}
        AND instance_id IN ({orgs})
        GROUP BY instance_id, day, amount_type
        ORDER BY instance_id ASC, day DESC, amount_type ASC;
        '''.format(start=start.isoformat(), end=end.isoformat(), orgs=', '.join(repr(o.interal_id) for o in orgs))
        cur.execute(q)
        orgs_by_id = {org.internal_id: org for org in orgs}
        return [
            (orgs_by_id[row[0]], row[1], row[2], int(row[3]))
            for row in cur.fetchall()
        ]
