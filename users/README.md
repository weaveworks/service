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

Mail is sent to mailcatcher, which runs at (http://smtp.weave.local)[http://smtp.weave.local]
Users can be approved at (http://users.weave.local/private/api/users)[http://users.weave.local/private/api/users]

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

## User session secret generation

The user management service signs the session cookies with HMAC. In order to do
that, it needs a key which is provided on the command line with the `--session-key`
argument.

In order to generate a key from the command line you can do:

```bash
cat /dev/random|xxd -c 2000 -ps|head -c64; echo`
```
