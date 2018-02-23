# Hacking on dashboard-api

Various things that can help the next developer that touches this service.

## Developing with a local Prometheus

dashboard-api talks to the cortex querier to retrieve the list of metrics for
a given service. That API is really the prometheus API so one can use a
regular Prometheus process to fake talking to Cortex.

This directory contains the manifests to start a Prometheus pod and service
in the cortex namespace:

- `cortex.yaml` contains a prometheus, kube-state-metrics and prom-node-exporter. This is taken from launch generator with three modifications: put the objects in the cortex namespace, remove the remote_write to Weave Cloud and re-enabling local storage.
- `cortex-service.yaml` container the service mocking the querier service.

To start this mock prometheus:

```shell
kubectl create ns cortex
kubectl apply -f dashboard-api/hacking/cortex.yaml
kubectl apply -f dashboard-api/hacking/cortex-service.yaml
```

Then, start a local dashboard-api using the manifests from service-conf:

```shell
kubectl apply -f k8s/local/cortex/dashboard-api-svc.yaml
kubectl apply -f k8s/local/cortex/dashboard-api-dep.yaml
```

In the browser, access dashboard-api entry points with the test user authenticated:

```shell
http://$(minikube ip):30081/api/app/local-test/api/dashboard/services/default/authfe/metrics
```
