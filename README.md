# Scope As A Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Run the infrastructure

You will need a Linux host or Virtual Machine to run the infrastructure locally. In case of
using a VM you will be able to connect to the infrastructure from your host
(e.g. Mac laptop).

You will need to have `kubectl` >= 1.1 installed on both environments:

* [For Mac](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl).
* [For Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl)

Also, please make sure to run Docker < 1.9 since version 1.9 has performance
issues which will make your machine unusable.

Now, on your Linux host or VM, build the service and deploy it locally.

```
cd $GOPATH/src/github.com/weaveworks/service
make
./deploy.sh -local
```

Then, we need to get your browser onto the local Kubernetes cluster network to
be able to talk to all its services. We have a handy `connect.sh` script for
that.

If you are running everything directly on a Linux host, set `<hostname>` to `127.0.0.1`.

```
./connect.sh <hostname>
```

It will tell you how to configure your host/browser to talk over the Kubernetes network.
When configuring your system proxies, ensure that proxies are *not* bypassed for `*.local`.

## Test the workflow

On your Mac laptop (or directly on your Linux host),

1. http://scope.weave.works — sign up
1. http://mailcatcher.default.svc.cluster.local — you should see a welcome email
1. http://users.default.svc.cluster.local — approve yourself
1. http://mailcatcher.default.svc.cluster.local — click on the link in the approval email

On your VM (or directly on your Linux host),

* Use the token in the approval email (e.g. `lhFr_M4SwtOmjLrrxHc2` )to start a probe:

```
scope launch --service-token=lhFr_M4SwtOmjLrrxHc2 "$(kubectl get svc frontend -o template --template '{{.spec.clusterIP}}')":80
```

Back on Mac laptop (or directly on your Linux host),

* Navigate to http://scope.weave.works and behold the beauty

Note that you'll need to preload a recent build of the Scope image.

## Deploy a new version of a service

```
┌ Local VM or VPS ─ ─ ─ ─ ─ ─ ┐     ┌ Remote (local, dev, prod) ─ ┐

│                ┌──────────┐ │     │ ┌─────────────────────────┐ │
                 │  Docker  │         │         Docker          │
│ ┌──────┐       │  ┌─────┐ │ │     │ │ ┌─────┐     ┌─────────┐ │ │
  │source│───────┼─▶│Image│─┼─────────┼▶│Image│────▶│Container│ │
│ └──────┘   ▲   │  └─────┘ │ │  ▲  │ │ └─────┘  ▲  └─────────┘ │ │
             │   └──────────┘    │    └──────────┼──────────────┘
│            │                │  │  │            │                │
             │                   │               │
│            │                │  │  │            │                │
             │                   │               │
└ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ┘  │  └ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ┘
             │                   │               │
             │                   │               │
             │                   │               │
           make               push.sh        deploy.sh
```

1. Make and merge changes following a normal PR workflow.
1. Produce up-to-date Docker image(s) on your local VM: `make`
1. Login to Quay with `docker login quay.io`. This only needs to be done once.
   If you don't have access to Quay ask a fellow scopet to grant it. If you
   already have access to Quay and are unsure about what credentials to type,
   go to https://quay.io/tutorial/. (You will need to set up a Quay password.)
1. Push the image(s) to the relevant hosts: `./push.sh -dev servicename`
1. Connect to the environment: `./connect.sh -dev`. You don't need to export
   anything; the deploy script takes care of that.
1. Deploy to the environment: `./deploy.sh -dev`
1. Commit and push the new .tfstate to master!

Replace `-dev` with `-local` or `-prod` as appropriate.
