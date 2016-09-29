#!/usr/bin/env ruby
# vim:set ft=ruby:
#
# Requires Ruby 2.2+
#
# Generate `monitoring/grafana/cpu_mem_by_service.json` with:
#   $ cd monitoring/grafana  # only to make following command shorter
#   $ ./generate_cpu_mem_by_service.rb | jq -S . > cpu_mem_by_service.json

require 'json'

services = %w(
  billing/billing-api
  default/authfe
  default/frontend
  default/launch-generator
  default/ui-server
  default/users
  default/users-db-exporter
  deploy/api
  deploy/deploy-db-exporter
  deploy/worker
  extra/demo
  extra/launch-generator
  extra/pr-assigner
  fluxy/fluxy
  kube-system/heapster
  kube-system/kube-dns
  kube-system/kubernetes-dashboard
  kube-system/monitoring-grafana
  kube-system/monitoring-influxdb
  kube-system/scope
  monitoring/alertmanager
  monitoring/grafana
  monitoring/kube-api-exporter
  monitoring/kubediff
  monitoring/prometheus
  monitoring/terradiff
  prism/consul
  prism/dev-grafana
  prism/dev-retrieval
  prism/distributor
  prism/ingester
  prism/memcached
  prism/prod-grafana
  prism/prod-retrieval
  scope/collection
  scope/consul
  scope/control
  scope/memcached
  scope/nats
  scope/pipe
  scope/query
)

def template(rows)
  # Set the ids on each panel
  rows.each_with_index { |row, rowIndex|
    row[:panels].each_with_index { |panel, panelIndex|
      panel[:id] = ((rowIndex+1)*10) + panelIndex
    }
  }


  # Return the big template
  {
    "annotations":  {
      "list":  []
    },
    "id":  nil,
    "title":  "CPU & Memory Usage by Service",
    "tags":  [],
    "style":  "dark",
    "timezone":  "utc",
    "editable": true,
    "gnetId": nil,
    "hideControls": false,
    "links": [],
    "refresh": "10s",
    "rows": rows,
    "schemaVersion": 12,
    "sharedCrosshair": false,
    "templating": {
      "list": []
    },
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "timepicker": {
      "now": true,
      "refresh_intervals": [
        "5s",
        "10s",
        "30s",
        "1m",
        "5m",
        "15m",
        "30m",
        "1h",
        "2h",
        "1d"
      ],
      "time_options": [
        "5m",
        "15m",
        "1h",
        "6h",
        "12h",
        "24h",
        "2d",
        "7d",
        "30d"
      ]
    },
    "version": 9
  }
end

def row(namespace, name)
  {
    "collapse": false,
    "editable": true,
    "height": "250px",
    "panels": [
      {
        "aliasColors": {},
        "bars": false,
        "datasource": "Scope-as-a-Service Prometheus",
        "editable": true,
        "error": false,
        "fill": 1,
        "grid": {
          "threshold1": nil,
          "threshold1Color": "rgba(216, 200, 27, 0.27)",
          "threshold2": nil,
          "threshold2Color": "rgba(234, 112, 112, 0.22)"
        },
        "id": 20,
        "isNew": true,
        "legend": {
          "avg": false,
          "current": false,
          "max": false,
          "min": false,
          "show": true,
          "total": false,
          "values": false
        },
        "lines": true,
        "linewidth": 2,
        "links": [],
        "nilPointMode": "connected",
        "nullPointMode": "connected",
        "percentage": false,
        "pointradius": 5,
        "points": false,
        "renderer": "flot",
        "seriesOverrides": [],
        "span": 6,
        "stack": false,
        "steppedLine": false,
        "targets": [
          {
            "expr": "sum(irate(container_cpu_usage_seconds_total{job=\"cadvisor\",io_kubernetes_pod_namespace=\"#{namespace}\",io_kubernetes_pod_name=~\"#{name}-.*\"}[1m])) by (io_kubernetes_pod_namespace,io_kubernetes_pod_name)",
            "intervalFactor": 2,
            "legendFormat": "{{io_kubernetes_pod_namespace}}/{{io_kubernetes_pod_name}}",
            "refId": "A",
            "step": 10
          }
        ],
        "timeFrom": nil,
        "timeShift": nil,
        "title": "#{namespace}/#{name} CPU Usage",
        "tooltip": {
          "msResolution": true,
          "shared": true,
          "sort": 0,
          "value_type": "cumulative"
        },
        "type": "graph",
        "xaxis": {
          "show": true
        },
        "yaxes": [
          {
            "format": "percentunit",
            "label": nil,
            "logBase": 1,
            "max": 1,
            "min": 0,
            "show": true
          },
          {
            "format": "short",
            "label": nil,
            "logBase": 1,
            "max": nil,
            "min": nil,
            "show": true
          }
        ]
      },
      {
        "aliasColors": {},
        "bars": false,
        "datasource": "Scope-as-a-Service Prometheus",
        "editable": true,
        "error": false,
        "fill": 1,
        "grid": {
          "threshold1": nil,
          "threshold1Color": "rgba(216, 200, 27, 0.27)",
          "threshold2": nil,
          "threshold2Color": "rgba(234, 112, 112, 0.22)"
        },
        "id": 21,
        "isNew": true,
        "legend": {
          "avg": false,
          "current": false,
          "max": false,
          "min": false,
          "show": true,
          "total": false,
          "values": false
        },
        "lines": true,
        "linewidth": 2,
        "links": [],
        "nilPointMode": "connected",
        "nullPointMode": "connected",
        "percentage": false,
        "pointradius": 5,
        "points": false,
        "renderer": "flot",
        "seriesOverrides": [],
        "span": 6,
        "stack": false,
        "steppedLine": false,
        "targets": [
          {
            "expr": "sum(container_memory_usage_bytes{job=\"cadvisor\",io_kubernetes_pod_namespace=\"#{namespace}\",io_kubernetes_pod_name=~\"#{name}-.*\"}) by (io_kubernetes_pod_namespace,io_kubernetes_pod_name)",
            "intervalFactor": 2,
            "legendFormat": "{{io_kubernetes_pod_namespace}}/{{io_kubernetes_pod_name}}",
            "refId": "A",
            "step": 10
          }
        ],
        "timeFrom": nil,
        "timeShift": nil,
        "title": "#{namespace}/#{name} Memory Usage",
        "tooltip": {
          "msResolution": true,
          "shared": true,
          "sort": 0,
          "value_type": "cumulative"
        },
        "type": "graph",
        "xaxis": {
          "show": true
        },
        "yaxes": [
          {
            "format": "bytes",
            "label": nil,
            "logBase": 1,
            "max": nil,
            "min": 0,
            "show": true
          },
          {
            "format": "short",
            "label": nil,
            "logBase": 1,
            "max": nil,
            "min": nil,
            "show": true
          }
        ]
      }
    ],
    "title": "#{namespace}/#{name}"
  }
end

puts JSON.pretty_generate(template(services.sort.uniq.map { |service|
  namespace, service = service.split("/")
  row(namespace.strip, service.strip)
}))
