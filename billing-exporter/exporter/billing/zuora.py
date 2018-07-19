import requests
import csv
import tempfile
from datetime import datetime, timedelta, timezone
from functools import lru_cache


def _fmt_date(date):
    return date.strftime('%d/%m/%Y')


def parse_datetime(s):
    return datetime.strptime(s, '%Y-%m-%d %H:%M:%S').replace(tzinfo=timezone.utc)


class Zuora(object):
    def __init__(self, url, username, password):
        self.base_url = url
        self.s = s = requests.Session()
        s.auth = (username, password)

    def _req(self, method, path):
        r = self.s.request(method=method, url=self.base_url + path)
        r.raise_for_status()
        return r.json()

    def get(self, path):
        return self._req('GET', path)

    def get_usage(self, account_id, start, end):
        url = f"{self.base_url}/v1/usage/accounts/{account_id}?pageSize=40"
        while url:
            r = self.s.get(url)
            r.raise_for_status()

            j = r.json()
            if not j['success']:
                raise Exception(j)

            for row in j['usage']:
                ts = parse_datetime(row['startDateTime'])
                if ts >= end:
                    continue
                if ts < start:
                    return
                yield row

            next_page = j.get('nextPage')
            if not next_page:
                break
            elif '://'  in next_page:
                # Zuora behaves strangely sometimes
                url = next_page
            else:
                url = f"{self.base_url}{next_page}"

    @lru_cache()
    def _get_usage_assignment(self, account_id):
        r = self.get('/v1/subscriptions/accounts/' + account_id)
        subscription, = r['subscriptions']
        ratePlan, = subscription['ratePlans']
        ratePlanCharge, = ratePlan['ratePlanCharges']
        return subscription['subscriptionNumber'], ratePlanCharge['number']

    def upload_usage(self, usage):
        rows = [
            (account_id,
            'node-seconds',
            total,
            _fmt_date(date),
            _fmt_date(date + timedelta(days=1)),
            self._get_usage_assignment(account_id)[0],
            self._get_usage_assignment(account_id)[1],
            'manual import')
            for account_id, date, total in usage
        ]
        print(len(usage), len(rows))
        with tempfile.TemporaryFile('w+') as fh:
            w = csv.writer(fh)

            w.writerow(('ACCOUNT_ID', 'UOM', 'QTY', 'STARTDATE', 'ENDDATE', 'SUBSCRIPTION_ID', 'CHARGE_ID', 'DESCRIPTION'))
            w.writerows(rows)
            fh.flush()
            fh.seek(0)

            r = self.s.post(self.base_url + '/v1/usage',
                files={'file': ('manual-upload.csv', fh, 'text/csv', {})})
            print(r)
            print(r.text)
            r.raise_for_status()
            # TODO check upload import status