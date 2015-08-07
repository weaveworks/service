# Scope As A Service - Application Mapper

The Application Mapper is a proxy between Scope probes/uis and Scope apps.

The Application Mapper:

1. Listens for http/ws requests under the URL path `api/app/<orgID>/*` from Scope probes/uis.
2. Authenticates all requests using the User Management API.
3. Trims the `api/app/<orgID>` path prefix and forwards requests to the app of
   organization `<orgID>`, allocating a new app if necessary. For instance,
   `api/app/<orgID>/request` would result in `/request` being forwarded to the
   app of organization `<orgID>`.

## Run

```
$ make
$ docker run weaveworks/app-mapper
```

## Tests

unit tests:

```
$ make test
```

integration tests:

```
$ make integration-test
```
