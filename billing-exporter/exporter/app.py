from datetime import date, datetime

from flask import Flask, request

from .reports.discrepancies import discrepancy_report

app = Flask(__name__)


@app.route('/')
def hello_world():
    today = date.today()
    return f'''
<html>
<head>
<title>Billing Exporter</title>
</head>
<body>
<h1>Billing Exporter</h1>
<h2>Discrepancy Report</h2>
<p>
    Compare usage recorded in bigquery, billing db and zuora, highlighting any discrepancies.<br />
    Queries all instances. Does not fully accoutn for trial start/end dates and instance deletion.
</p>
<form action="/discrepancies" method="post">
    <label for="date">Date in month</label>
    <input name="date" id="date" value="{today.isoformat()}">
    <button>Run</button>
</form>
<body>
'''


@app.route('/discrepancies', methods=('POST', ))
def discrepancies():
    date = datetime.strptime(request.form['date'], '%Y-%m-%d')
    discrepancies = discrepancy_report(
        **{
            k: v
            for k, v in app.config.items() if k in {
                'billing_db_uri', 'users_db_uri', 'zuora_uri', 'bq_creds',
                'production'
            }
        },
        datetime_in_month=date,
    )

    def _delta(amounts, k1, k2, includeZeros=False):
        i = amounts.get(k2, 0) - amounts.get(k1, 0)
        if i > 0:
            return f'+{i}'
        elif i == 0 and not includeZeros:
            return ''
        return str(i)

    rows = []
    for report in discrepancies:
        rows += [
            f'''<tr>{
                ''.join(
                    f'<td>{field}</td>'
                    for field in (
                        report.org.external_id,
                        'total' if day is None else str(day.date()),
                        amounts.get('bigquery', 0),
                        _delta(amounts, 'bigquery', 'db', includeZeros=day is None),
                        amounts.get('db', 0),
                        _delta(amounts, 'db', 'zuora', includeZeros=day is None),
                        amounts.get('zuora', 0),
                    )
                )
            }</tr>'''
            for day, amounts in report.days + ((None, report.total), )
        ] + ['<tr></tr>']

    table_body = '\n'.join(rows)
    return f'''
<html>
<head>
<title>Billing Exporter: Discrepancy Report</title>
</head>
<body>
<h1>Discrepancy Report</h1>
<strong>NB: Trial start date and instance deletion are not yet fully accounted for</strong>
<table>
    <thead>
        <tr><td>instance</td><td>date</td><td>bigquery</td><td>+/-</td><td>db</td><td>+/-</td><td>zuora</td></tr>
    </thead>
    <tbody>{table_body}</tbody>
</table>
<body>'''
