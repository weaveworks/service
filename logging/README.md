# Collecting Docker Log Files with Fluentd and Bigquery
This directory contains the source files needed to make a Docker image
that collects Docker container log files using [Fluentd](http://www.fluentd.org/)
and sends them to an instance of [BigQuery](https://cloud.google.com/bigquery/).
This image is designed to be used as part of the [Kubernetes](https://github.com/kubernetes/kubernetes)
cluster bring up process.
