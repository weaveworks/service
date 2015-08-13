# Scope As A Service - Application Mapper

The Application Mapper is a proxy between Scope probes/uis and Scope apps.

## Endpoints

* `api/app/<orgName>/*`

  HTTP Methods: all http methods and websockets

  Authentication: `_weave_run_session` cookie

  Meant to proxy requests from the Scope ui to Scope apps.

  Trims the `api/app/<orgName>` path prefix and forwards requests to the app of
  organization with name `<orgName>`, allocating a new app if necessary. For
  instance, `api/app/<orgName>/request` would result in `/request` being
  forwarded to the app of organization with name `<orgName>`.

* `/api/report`

  HTTP Methods: all http methods and websockets

  Authentication: `Authorization` header (set to a probe-token)

  Meant to proxy requests from Scope probes to Scope apps.

  Forwards the request request *as-is* to the app of organization associated to
  the probe-token in the `Authorization` header, allocating a new app if
  necessary.

* `api/org/<orgName>/probes`

  HTTP Methods: GET

  Authentication: `_weave_run_session` cookie

  Responds with a description of the probes which contacted a Scopee app of
  the organization with name `<orgName>`.

  Sample response:

```json
[
  {"id": "someProbeID1", "lastSeen":"2015-08-13T14:06:19.689855986Z"},
  {"id": "someProbeID2", "lastSeen":"2015-08-13T14:06:19.68985711Z"}
]
```

## Run

```
$ make
$ docker run weaveworks/app-mapper
```

## Tests

Unit tests:

```
$ make test
```

Integration tests:

```
$ make integration-test
```
