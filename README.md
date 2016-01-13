# Scope as a Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

```
  Internet
     |
     v
+----------+
|          | /*           +--------------------+
| frontend |------------->| ui-server (client) |
|          |              +--------------------+
|          |
|          | /api/users/login  +-------+  +----+
|          |------------------>| users |--| DB |
|          |                   +-------+  +----+
|          | /api/app
|          | /api/org
|          | /api/report  +------------+  +----+
|          |------------->| app-mapper |--| DB |
+----------+              +------------+  +----+
                                |
                              (k8s)
                                |
                                v
                          +------------+
                          | Scope Apps |-+
                          +------------+ |
                            +------------+
```

When visiting with a web browser, users see the front page via the ui-server,
and are directed to sign up or log in. Authentication and email is managed by
the users service. Once authenticated, users are brought to their dashboard.
From there, they can learn how to configure a probe, and may view their Scope
instance.

Each user gets their own Scope app instance, managed by app-mapper and
supervised by Kubernetes. When visiting scope.weave.works with a browser, the
app-mapper uses browser cookies to proxy the user to the correct Scope app UI.
When the user's Scope probe POSTs reports to scope.weave.works, the app-mapper
uses the service token to proxy to the correct Scope app /api/report endpoint.

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

We bootstrap a one-node Kubernetes "cluster" on top of Docker.  This works on
both Linux and Darwin.  Note this **must be Docker 1.8** -- 1.9 has
[performance issues](https://github.com/docker/docker/issues/17720) that make it
unusable. When using `docker-machine`, create your VM with
`docker-machine create --driver=virtualbox --virtualbox-boot2docker-url=https://github.com/boot2docker/boot2docker/releases/download/v1.8.3/boot2docker.iso <VMname>`.

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
$ # TODO there might be a nicer way of doing this; investigate kubectl
$ kubectl --kubeconfig=infra/local/kubeconfig delete rc users
$ kubectl --kubeconfig=infra/local/kubeconfig create -f k8s/local/users.yaml
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

Someone has probably already created the components, and you probably just want to deploy a new version.
Kubernetes supports this nicely using something called a rolling update.
We've scripted it for you; just follow the prompts.

```
$ ./rolling-update
```

Kubernetes has a lot of ways to move things around in the cluster.
Don't be afraid to read the kubectl documentation.

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
$ scope launch --service-token=abc frontend.dev.weave.works:80  # for dev
$ scope launch --service-token=abc                              # for prod
```
