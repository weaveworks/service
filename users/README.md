# Scope As A Service - User Management API

## Run

```
$ docker-compose up
```

## Development with boot2docker and docker-compose

Assumes master of weave.

```
$ boot2docker up
$ eval "$(boot2docker shellinit)"
$ weave launch-router
$ weave launch-proxy --tls \
    --tlscacert /var/lib/boot2docker/tls/ca.pem \
    --tlscert /var/lib/boot2docker/tls/server.pem \
    --tlskey /var/lib/boot2docker/tls/serverkey.pem
$ eval "$(weave env)"
$ docker-compose up
```

Mail is sent to mailcatcher, which runs on port 1080.
Users can be approved at `http://$(bootdocker ip):3000/private/api/users`

## Tests

unit tests:

```
$ make test
```

integration tests:

```
$ weave launch
$ eval "$(weave env)"
$ make integration-test
```
