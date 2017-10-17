# Weave Cloud

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

```
  Internet
     |
     v
+----------+ /*           +--------------------+
|          |------------->| ui-server (client) |
|          |              +--------------------+
|          |
|          | /api/users      +-------+  +----+
|          |---------------->| users |--| DB |
|          |                 +-------+  +----+
|  authfe  |
|          | /api/report     +------------+
|          |---------------->| Collection |-+
|          |                 +------------+ |
|          |                   +------------+
|          | /api/topology   +------------+
|          |---------------->| Query      |-+
|          |                 +------------+ |
|          |                   +------------+
|          | /api/control    +------------+
|          |---------------->| Control    |-+
|          |                 +------------+ |
|          |                   +------------+
|          | /api/pipe       +------------+
|          |---------------->| Pipe       |-+
|          |                 +------------+ |
|          |                   +------------+
|          |
|          | /api/billing    +-------------+  +------------+
|          |---------------->| billing-api |--| Billing DB |
|          |                 +-------------+  +------------+
+----------+
```

When visiting with a web browser, users see the front page via the ui-server,
and are directed to sign up or log in. Authentication and email is managed by
the users service. Once authenticated, users are brought to their dashboard.
From there, they can learn how to configure a probe, and may view their Scope
instance.

## Build

The Makefile will automatically produce container images for each component, saved to your local Docker daemon.
And the k8s/local resource definitions use the latest images from the local Docker daemon.
So, you just need to run make.

```
$ make
```

## Test & Deploy

For information about how to test these images locally, see [service-conf.git](http://github.com/weaveworks/service-conf.git).

## Push Manually

Normally the CI system will push images, so you won't have to push
them manually. If you have some reason for pushing images manually,
follow these instructions.

The build gives each image a name based on the state of the git
working directory from which it was built, according to the script
`./tools/image-tag`. If the working directory has uncommitted changes, the
image name will include `-WIP` (work in progress), to make it stand
out.  For example:

```
$ ./tools/image-tag
deployment-instructions-eca773b-WIP
```

The script `push-images` pushes the images *named for the current
state of the git working directory* to the remote repository. So, if
you just ran `make` and it succeeded, the images pushed will be those
that were built by make. If you did anything with git or the source
files in between, it will probably bail out.

```
$ make
$ ./push-images
```

If you just want to tag and push specific container(s), you may
specify them as arguments.

```
$ ./push-images users ui-server
```

Note that make will not usually build every image, so if you are
running `push-images` from your own build directory, it's likely you
will need to name specific components. The script will bail before
pushing anything if it can't find all the images you mention (or imply
by not mentioning any).

## Billing

For information about billing, see [BILLING.md](https://github.com/weaveworks/service/blob/master/BILLING.md)
