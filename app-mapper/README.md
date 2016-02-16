# app-mapper

The app-mapper is a proxy between Scope probes/UIs and Scope apps. It creates apps lazily on the first request.

## Build

You should build all components via the toplevel Makefile.

## Run

See instructions in the toplevel README about running a local cluster.

## Test

```sh
$ env GO15VENDOREXPERIMENT=1 go test
```

## API

* `api/app/<orgName>/*`

  HTTP Methods: all HTTP methods and websockets.

  Authentication: `_weave_run_session` cookie

  Meant to proxy requests from the Scope UI to Scope apps.

  Trims the `api/app/<orgName>` path prefix and forwards requests to the app of
  organization with name `<orgName>`, allocating a new app if necessary. For
  instance, `api/app/<orgName>/request` would result in `/request` being
  forwarded to the app of organization with name `<orgName>`.

* `/api/report` (report from Scope probe) and `/api/control/ws` (control websocket from probe)

  HTTP Methods: all HTTP methods and websockets.

  Authentication: `Authorization` header (set to a probe-token)

  Forwards the request request *as-is* to the app of organization associated to
  the probe-token in the `Authorization` header, allocating a new app if
  necessary.

* `api/org/<orgName>/probes`

  HTTP Methods: GET

  Authentication: `_weave_run_session` cookie

  Responds with a description of the probes which contacted a Scope app of
  the organization with name `<orgName>`.

  Sample response:

```json
[
  {"id": "someProbeID1", "lastSeen":"2015-08-13T14:06:19.689855986Z"},
  {"id": "someProbeID2", "lastSeen":"2015-08-13T14:06:19.68985711Z"}
]
```
