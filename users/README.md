# users

The user management API service.

## Build

You should build all components via the toplevel Makefile.

## Run

See instructions in the toplevel README about running a local cluster.

## Test

```sh
$ env GO15VENDOREXPERIMENT=1 go test
```

## User session secret generation

The user management service signs the session cookies with HMAC. In order to do
that, it needs a key which is provided on the command line with the `--session-key`
argument.

In order to generate a key from the command line you can do:

```sh
$ cat /dev/random|xxd -c 2000 -ps|head -c64; echo`
```
