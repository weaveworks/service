# Collecting Events with Fluentd and Bigquery
This directory contains the source files needed to make a Docker image of a
[Fluentd](http://www.fluentd.org/) service with the right plugins to send events to an instance of [BigQuery](https://cloud.google.com/bigquery/), and expose prometheus metrics.

This is currently used [in authfe](https://github.com/weaveworks/service/blob/9fe4b77f78090b9b8714b751fd57bfc706f0f054/authfe/routes.go#L153), with [runtime configuration](https://github.com/weaveworks/service-conf/blob/master/k8s/dev/default/fluent-events-config.yaml) to listen on port 24224 for events, publish them to BigQuery, and expose prometheus metrics on port 24231.
