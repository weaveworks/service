# monitoring

This is a single container that runs

- Prometheus, with DNS A-record support (0.16.0+)
- Grafana, with Prometheus backend support (currently requires building manually from a branch)
- gfdatasource, a service to inject Grafana with the Prometheus data source (as Grafana has no way to statically declare data sources)
- loadgen, a service to generate some load to the components (so we don't have just flat lines)

Each component (users, app-mapper, etc.) exposes a scrapable endpoint with metrics.
Prometheus uses DNS A-records to discover instances of each component, and scrapes them on a regular interval.
Grafana is configured with dashboards that call the Prometheus server.

```
           .---------------------------------------.
           |               .---------------------. |
           |               |              .----. | |
           v               v              v    | | |
     +-------+  +------------+  +-----------+  | | |
     | users |  | app-mapper |  | component |  | | |
     +-------+  +------------+  +-----------+  | | |
       ^          ^               ^            | | |
       | .--------'               |            | | |
       | | .----------------------'            | | |
       | | |                                   | | |
+ - - -|-|-|- - - - - - - - - - - - - - - - - -|-|-|- - - +
|      | | |                                   | | |      |
|    +------------+                        +---------+    |
|    | Prometheus |                        | loadgen |    |
|    +------------+                        +---------+    |
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
