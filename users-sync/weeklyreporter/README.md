## Testing

To make this work locally, a few steps need to be done:

1. Check out`README.md` in `users/weeklyreports` to see what needs to be done for report generating to work
2. Change `true` to `false` in `common/users/client.go` -> `NewInsercureConn` in `NewClient` func to temporarily disable load balancing (see https://github.com/weaveworks/common/pull/128)
3. Change `postgres://postgres@users-db.weave.local/users?sslmode=disable` to `memory://postgres@users-db.weave.local/users?sslmode=disable` in `users-sync/cmd/users-sync.go`
4. Change `k8s/local/default/users-svc.yaml` in `service-conf` repo so that the users service accepts RPC requests and reboot the service:
```
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
5. Add `weekly-reportable` feature flag to your local instance(s)

If this still didn't make it work as you expected, try the following:

* Comment out the `filter.SeenPromConnected(true)` condition in `GetOrganizationsReadyForWeeklyReport`
* Change the `cronSchedule` setting in `job.go` to make job runs more frequent for testing

That should be enough to keep it going!
