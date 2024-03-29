# Testing

In order to test the weekly report emails on a local instance, both Flux and Prom services need to be running
to generate the weekly report. To avoid that, the report generator can be pointed to remote services by changing
`promURI`/`fluxURI` constants in `weeklyreports/report.go`. For example, to point them to an instance in dev,
the endpoints need to changed to:

```go
const = (
    promURI = "https://user:[TOKEN]@frontend.dev.weave.works/api/prom"
    fluxURI = "https://user:[TOKEN]@frontend.dev.weave.works/api/flux"
)
```

On top of that, `report.Organization.CreatedAt` in `generateDeploymentsHistogram` probably needs to be pushed a
few days back to make sure the creation date of the local instance is not blocking the deployments histogram from
being shown.
