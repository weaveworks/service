# Scope As A Service - Application Mapper

The Application Mapper is a proxy between Scope probes/uis and Scope apps.

The Application Mapper:

1. Listens for http/ws requests from Scope probes/uis.
2. Authenticates all requests using the User Management API.
3. Based on the request credentials, forwards requests *as-is* to the target app,
   allocating a new app if necessary.

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
