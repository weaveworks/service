instances = []
with open('./instances') as f:
    for line in f:
        id, external_id, zuora_id = [x.strip() for x in line.split('|')]
        instances.append([id, external_id, zuora_id])

from exporter.billing.zuora import Zuora, parse_datetime
from pprint import pprint
import os

secret = os.getenv('ZUORA_SECRET', '')
zclient = Zuora('https://zuora-api-user@weave.works:%s@api.zuora.com/rest' % secret)
dt_from = parse_datetime('2018-11-28 00:00:00')
dt_to = parse_datetime('2018-11-29 00:00:00')

# Useful to debug what the response objects look like
# pprint(list(zclient.get_usage('W23aa447d3c0d65c63232415a7941a89', dt_from, dt_to)))

for id, external_id, zuora_id in instances:
    rows = list(zclient.get_usage(zuora_id, dt_from, dt_to))
    if len(rows) != 2:
        print('%d row(s) for %s/%s/%s' % (len(rows), id, external_id, zuora_id))
        continue
    if rows[0]['quantity'] != rows[1]['quantity']:
        print('different quantities for %s/%s/%s' % (id, external_id, zuora_id))
        continue
    zclient.delete_usage(rows[0])
    summary = {'id': rows[0]['id'], 'quantity': rows[0]['quantity'], 'submissionDateTime': rows[0]['submissionDateTime']}
    print('DELETE %s for %s/%s/%s' % (summary, id, external_id, zuora_id))

