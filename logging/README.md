# Collecting Docker Log Files with Fluentd and Bigquery
This directory contains the source files needed to make a Docker image
that sets up a [Fluentd](http://www.fluentd.org/) daemon which listens on port 24224 for events
and sends them to an instance of [BigQuery](https://cloud.google.com/bigquery/).

This is currently used [in authfe](https://github.com/weaveworks/service/blob/9fe4b77f78090b9b8714b751fd57bfc706f0f054/authfe/routes.go#L153)
