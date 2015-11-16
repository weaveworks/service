# Scope as a Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Local development

Prerequisites.

- A Linux host -- later, you'll be able to interact with the cluster from your Mac, if you want to
- kubectl v1.1.1+ -- [Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl), [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl)
- Docker 1.8 -- 1.9 has [performance issues](https://github.com/docker/docker/issues/17720) that make it unusable

The deploy local script will deploy Kubernetes as a one-node cluster via Docker containers.

```
# Linux host
cd $GOPATH/src/github.com/weaveworks/service
make
./deploy.sh -local
```

Now, we will get your laptop onto the Kubernetes cluster you've just built.
If your laptop is also the Linux host, you can use 127.0.0.1 for hostname.

```
# Your laptop
./connect.sh <hostname>
```

Follow the instructions provided after you connect.
Then, you should be able to access the individual services.

- http://scope.weave.works -- front page, signup with a bogus email address
- http://mailcatcher.default.svc.cluster.local —- you should see a welcome email
- http://users.default.svc.cluster.local —- you can approve yourself here
- http://mailcatcher.default.svc.cluster.local —- you should see another email, and can click the login link

Now you can start a probe, and send reports to your Scope in the cloud.

```
scope launch --service-token=a0b1c2d3e4f5g6h7 "$(kubectl get svc frontend -o template --template '{{.spec.clusterIP}}')":80
```

You should see your topology appear in the web UI.

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
