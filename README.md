# Scope as a Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Prerequisites

You need kubectl v1.1.1 in your PATH.
Download the
 [Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl) or
 [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl) release.

```
$ kubectl version
Client Version: version.Info{Major:"1", Minor:"1", GitVersion:"v1.1.1", ... }
error: couldn't read version from server: Get http://localhost:8080/api: dial tcp 127.0.0.1:8080: connection refused
```

## Local development

### Stand up

We bootstrap a one-node Kubernetes "cluster" on top of Docker.
This works on both Linux and Darwin.
Note this **must be Docker 1.8** -- 1.9 has [performance issues](https://github.com/docker/docker/issues/17720) that make it unusable.

```
$ infra/local-k8s up
```

### Build

The Makefile will automatically produce container images for each component, saved to your local Docker daemon.
And the k8s/local resource definitions use the latest images from the local Docker daemon.
So, you just need to run make.

```
$ make
```

You can also pull existing containers from Quay and re-tag them as latest.

```
$ bash -c '
  for c in $(grep "quay.io/weaveworks/" k8s/prod/*-rc.yaml | awk \'{print $3}\')
  do
    docker pull $c
    docker tag -f $c $(echo $c | cut -d\':\' -f1):latest
  done
'
```

### Deploy

Create the components from an empty state.

```
$ kubectl --kubeconfig=infra/local/kubeconfig create -f k8s/local
```

Or, update a specific component.

```
$ kubectl --kubeconfig=infra/local/kubeconfig replace -f k8s/local/users-rc.yaml
```

### Connect

```
$ ./connect local
```

### Test

You must connect to the cluster for these steps to work.

1. Go to http://scope.weave.works and sign up with a bogus email address.
2. Go to http://mailcatcher.default.svc.cluster.local and look for the welcome message.
3. Go to http://users.default.svc.cluster.local and approve yourself.
4. Go to http://mailcatcher.default.svc.cluster.local again, find the approval message, and click the login link.

Now you can start a probe with your service token, and send reports to your local Scope-as-a-Service.

```
$ IP=$(kubectl --kubeconfig=infra/local/kubeconfig get svc frontend -o template --template '{{.spec.clusterIP}}')
$ scope launch --service-token=abc $IP:80
```

### Tear down

```
$ infra/local-k8s down
```

## Remote clusters

You probably won't need to stand up or tear down a cluster on AWS.
Instead, you'll probably interact with an existing cluster, like dev or prod.
But if you really want to know, you can read [the README in the infra subdirectory](/infra).

### Build

To make containers available to remote clusters, you need to
 build them,
 tag them with their Docker image IDs, and
 push them to our private remote repository (Quay).

```
$ make
$ ./tag-and-push
```

If you just want to tag and push specific container(s), you may specify them as arguments.

```
$ ./tag-and-push users ui-server
```

### Deploy

Someone has probably already created the components.
You're probably interested in deploying a new version of a component.
Kubernetes supports this nicely using a rolling upgrade.

```
$ # TODO kubectl
```

For more deployment options, see the Kubernetes documentation.

### Connect

```
$ ./connect mycluster
```

### Test

You must connect to the cluster for these steps to work.

1. Go to http://scope.weave.works and sign up with a real email address.
2. Check your email for the welcome message.
3. Go to http://users.default.svc.cluster.local and approve yourself.
4. Check your email for the approval message, and click the login link.

Now you can start a probe with your service token, and send reports to the Scope-as-a-Service.
If you're using e.g. the dev cluster, you'll need to specify the target by IP.

```
$ scope launch --service-token=abc docker.dev.weave.works:80  # for dev
$ scope launch --service-token=abc                            # for prod
```
