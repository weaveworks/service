# Scripts

This directory contains scripts which may be of use when investigating billing discrepancies.

- `gcp_parse_loggly.py` - Parse GCP upload records from a Loggly JSON export
- `dollarise_loggly.py` - Dollarise the actual usage and the usage uploaded to GCP
- `billing_aggregates_vs_gcp_upload.py` - Compare our usage aggregates to the amounts uploaded to GCP
- `remove_zuora_duplicates.py` - Remove duplicated usage objects in Zuora

## Remove Zuora duplicates

Due to [unfortunate circumstances][service-conf#2793], it's possible that we upload the daily usage twice to Zuora.

1. Start by grabbing the Zuora secret from
   `service-conf/k8s/prod/billing/zuora-secret.yaml`, use `base64 --decode` and export it in a `ZUORA_SECRET` environment variable.

```console
$ export ZUORA_SECRET=yexxxxxxxxxxxxxxxxxxxxx
```

2. Grab the list of Zuora instances and put in a
   `billing-exporter/scripts/instances` file.

```console
$ ./infra/database shell prod users_vpc
users_vpc=> SELECT id, external_id, zuora_account_number FROM organizations WHERE zuora_account_number IS NOT NULL;

  id   |     external_id     |       zuora_account_number
-------+---------------------+----------------------------------
 8902  | summer-rock-55      | W5d25151c3cfdaebef9ae1d5f6aff4ee
 12316 | chilly-surf-27      | W4b6011fd73b7483c347bce4557177e0
 13124 | withered-cloud-01   | W4aced4c9b6544aa3b01fa4d805e4e2e
 ...
```

This `instances` file should look like:

```text
 8902  | summer-rock-55      | W5d25151c3cfdaebef9ae1d5f6aff4ee
 12316 | chilly-surf-27      | W4b6011fd73b7483c347bce4557177e0
 13124 | withered-cloud-01   | W4aced4c9b6544aa3b01fa4d805e4e2e
 ...
```

3. Change the `dt_from` and `dt_to` variables to the day of the
   `BillingUploaderDown` incident.

4. Build the billing-exporter image:

```console
$ cd billing-exporter
$ docker build -t billing-scripts .
```

5. Run the `remove_zuora_duplicates.py` script: note this will actually delete
   usage objects only it finds duplicate entries with the exact same
   node-seconds number.

```console
$ docker run -e ZUORA_SECRET -e PYTHONPATH=/ -w /scripts -ti billing-scripts python remove_zuora_duplicates.py
DELETE {'quantity': 1118979.0, 'submissionDateTime': '2018-11-28 07:15:23', 'id': '2c92a09967588cfd01675ae3957d45c3'} for 8902/summer-rock-55/W5d25151c3cfdaebef9ae1d5f6aff4ee
DELETE {'quantity': 518391.0, 'submissionDateTime': '2018-11-28 07:15:23', 'id': '2c92a09967588cfd01675ae3957c45ba'} for 12316/chilly-surf-27/W4b6011fd73b7483c347bce4557177e0
DELETE deleting {'quantity': 689433.0, 'submissionDateTime': '2018-11-28 07:15:23', 'id': '2c92a09967588cfd01675ae3957945a4'} for 13124/withered-cloud-01/W4aced4c9b6544aa3b01fa4d805e4e2e
0 row(s) for 8456/silent-rain-17/Wcce5093f1863f0037b5fbb61ad4456a
0 row(s) for 12690/thawing-dust-53/Wd468a216e5082926fada0088a01d49d
[...]
```

[service-conf#2793]: https://github.com/weaveworks/service-conf/issues/2793
