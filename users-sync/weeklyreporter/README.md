# Testing

The easiest way to test weekly reporting on you local machine is to make `local-test` instance _reportable_
so that the emails would be triggered for it, but use a remote instance (e.g. `Weave Cloud (dev)`) for
generating the actual report since it is likely to contain more interesting data than your `local-test`
instance.

To make this work, a few steps need to be done:

1. Check out `README.md` in `users/weeklyreports` to see what needs to be done for report generating to work
2. Change `true` to `false` in `common/users/client.go` -> `NewInsecureConn` in `NewClient` func to
  temporarily disable load balancing (see [#128](https://github.com/weaveworks/common/pull/128))
3. Change `postgres://postgres@users-db.weave.local/users?sslmode=disable` to
  `memory://postgres@users-db.weave.local/users?sslmode=disable` in `users-sync/cmd/users-sync.go`
4. Change `k8s/local/default/users-svc.yaml` in `service-conf` repo so that the users service accepts RPC
  requests and reboot the service:
    ```yaml
    spec:
      ports:
        - port: 80
    +      targetPort: 80
    +      name: http
    +      protocol: TCP
    +    - port: 4772
    +      targetPort: 4772
    +      name: grpc-noscrape
    +      protocol: TCP
    ```
5. Add `weekly-reportable` feature flag to your `local-test` instance
6. Boot up your `k8s/local/default` services in Minikube via `./connect` as described in
  [service-conf README](https://github.com/weaveworks/service-conf#deploy)

If this still didn't make it work as you expected, try the following:

* Comment out the `filter.SeenPromConnected(true)` condition in `GetOrganizationsReadyForWeeklyReport`
* Change the `cronSchedule` setting in `job.go` to make job runs more frequent for testing

Now you should be able to test sending test reports through the
[admin interface](http://authfe.default.svc.cluster.local/admin/users/weeklyreports) and
see the emails in [mailcatcher](http://mailcatcher.default.svc.cluster.local/)!
