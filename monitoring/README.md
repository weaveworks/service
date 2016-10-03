# monitoring

This is a single container that runs

- Prometheus, with Kubernetes support (0.16.0+)
- Grafana, with Prometheus backend support (2.5.0+)
- gfdatasource, a service to inject Grafana with the Prometheus data source (as Grafana has no way to statically declare data sources)

Each component (users, app-mapper, etc.) exposes a scrapable endpoint with metrics.
Prometheus queries the Kubernetes master to discover instances of each component, and scrapes them on a regular interval.
Grafana is configured with dashboards that call the Prometheus server.

```
     +-------+  +------------+  +-----------+
     | users |  | app-mapper |  | component |
     +-------+  +------------+  +-----------+
       ^          ^               ^
       | .--------'               |
       | | .----------------------'
       | | |
+ - - -|-|-|- - - - - - - - - - - - - - - - - - - - - - - +
|      | | |                                              |
|    +------------+                                       |
|    | Prometheus |                                       |
|    +------------+                                       |
|      ^                                                  |
|      |                                                  |
|    +---------+                      +--------------+    |
|    | Grafana |<---------------------| gfdatasource |    |
|    +---------+                      +--------------+    |
|                                                         |
+ - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
```

Access the dashboards on **monitoring:3000**.

## Build

You should build all components via the toplevel Makefile.

## Run

See instructions in the toplevel README about running a local cluster.

## Test

No tests.
