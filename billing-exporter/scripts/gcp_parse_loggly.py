#!/usr/bin/env python3
'''
Parse and simplify GCP usage uploads objects of this shape:

    {
      "consumerId": "project_number:946613700308",
      "endTime": "2018-05-30T11:00:00Z",
      "metricValueSets": [
        {
          "metricName": "google.weave.works/standard_nodes",
          "metricValues": [
            {
              "int64Value": "2940"
            }
          ]
        }
      ],
      "operationId": "61f4d6a7-28e6-5919-8386-d8d4b4a7151d",
      "operationName": "HourlyUsageUpload",
      "startTime": "2018-05-30T10:00:00Z"
    }

out of Loggly JSON exports.

To get the Loggly JSON export:
- Go to https://weave.loggly.com
- Query:
  json.kubernetes.container_name:uploader AND "Uploading GCP usage"`
  from -15d
  to now
  (e.g.: https://weave.loggly.com/search#terms=json.kubernetes.container_name:uploader%20AND%20%22Uploading%20GCP%20usage%22&from=2018-06-18T17:43:29.592Z&until=2018-07-03T17:43:29.592Z&source_group=)
- Export as JSON.
'''

from os.path import join, split, splitext
from sys import argv
import json


TOKEN = 'Uploading GCP usage: '


def parse_loggly(events):
    for event in events:
        log = event['event']['json']['log']
        idx = log.index(TOKEN) + len(TOKEN)
        array = json.loads(log[idx:].strip())
        for obj in array:
            vs = [item['metricValues'] for item in obj['metricValueSets']]
            assert len(vs) == 1
            assert len(vs[0]) == 1
            v = int(vs[0][0]['int64Value'])
            yield obj['consumerId'], obj['startTime'], v


def main(filepath):
    with open(filepath, 'r') as f:
        data = list(parse_loggly(json.load(f)['events']))
    directory, file = split(filepath)
    csv_filepath = join(directory, splitext(file)[0] + '.csv')
    with open(csv_filepath, 'w') as f:
        for cid, t, v in data:
            f.write('%s\t%s\t%s\n' % (cid, t, v))


if __name__ == '__main__':
    main(argv[1])
