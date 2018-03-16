# dashboard-api

Serve dashboards to the UI.

- [Scope document](https://docs.google.com/document/d/1I1TKUGlnAJvb7ASGrmgYYME6PyRU-cTZYH_ABUYzDqE/edit)
- [Initial design doc](https://docs.google.com/document/d/1CQ2JW2_E1Tj6-CfAGcbf6sXzePvJnQCuywcz5L3l7iE/edit?usp=sharing)

## Testing a workload dashboard against the UI locally

When developing a new dashboard, it's more than useful to be able to
visualize it. We want this to be much simpler in the future (by allowing
dashboard uploads), but in the mean time, the easiest way to do it is to:

- Spawn a local UI connecting to frontend.dev.w.w:

```
service-ui/client$ SERVICE_HOST=frontend.dev.weave.works yarn start
```

- Generate a local dashboard JSON:

```shell
service$ go run ./dashboard-api/cmd/wc-dashboard/main.go -js -namespace cortex -workload ingester go-runtime > /path/to/service-ui/client/src/pages/prom/workloads/dashboards.js
```

- Hack `workload-homepage.jsx` to load the newly generated dashboard:

```diff
diff --git a/client/src/pages/prom/workloads/workload-homepage.jsx b/client/src/pages/prom/workloads/workload-homepage.jsx
index 3411347..3c2c539 100644
--- a/client/src/pages/prom/workloads/workload-homepage.jsx
+++ b/client/src/pages/prom/workloads/workload-homepage.jsx
@@ -18,6 +18,7 @@ import TimeTravelWrapper from '../time-travel-wrapper';
 import TabSection from './tab-section';
 import TabSelector from './tab-selector';
 import WorkloadDashboard from './workload-dashboard';
+import { DashboardsJSON } from './dashboards';
 
 const WorkloadHomepageWrapper = styled.div`
   margin: 15px 20px;
@@ -199,7 +200,7 @@ function mapStateToProps(state, { params }) {
     startTime,
     endTime,
     rangeMs,
-    dashboards,
+    dashboards: JSON.parse(DashboardsJSON) || dashboards,
     dashboardIds,
   };
 }
```

## Updating golden test files

We serve JSON files with promQL queries and we can't really test they are valid
(ie. will produce data), the way used to ensure we don't regress in what we
send to the FE is done with golden files. We record a known-to-work state in
`testdata/*.golden` and test that they don't change unexpectedly.

When something changes, like a query in a dashboard, it is necessary to
regenerate the golden files. This is done with:

```shell
go test ./dashboard-api/... -args -update
```

The diff can be inspected as a way to check for unexpected changes.
