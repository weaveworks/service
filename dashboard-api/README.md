= dashboard-api

Serve dashboards to the UI.

- [Scope document](https://docs.google.com/document/d/1I1TKUGlnAJvb7ASGrmgYYME6PyRU-cTZYH_ABUYzDqE/edit)
- [Initial design doc](https://docs.google.com/document/d/1CQ2JW2_E1Tj6-CfAGcbf6sXzePvJnQCuywcz5L3l7iE/edit?usp=sharing)

== Updating golden test files

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
