# infra

The **infra** deals with everything between our metal (AWS) and our application.
It's concerned with provisioning the scheduling system (k8s), stateful storage (RDS), load balancers (ELB), and DNS (Route53).

```
+-----------------------------+  --.
|             AWS             |    |
+-----------------------------+    |
+-----+ +-----+ +-----+ +-----+    |
| r53 |-| ELB | | EC2 | | RDS |    | infra
+-----+ |     | +-----+ |     |    |
        |     |    |    |     |    |
        |     | +-----+ |     |    |
        |     |-| k8s | |     |    |
        +-----+ +-----+ +-----+  --'
                   |       |
                +-------------+
                |     App     |
                +-------------+
```

1. [Bootstrap](#bootstrap)
1.1. [Set up AWS](#set-up-aws)
1.1. [Set up kubectl](#set-up-kubectl)
1.1. [Get kubernetes.bash](#get-kubernetes.bash)
1.1. [Run provision.bash](#run-provision.bash)
1.1. [Verify the cluster](#verify-the-cluster)
1.1. [Commit the kubeconfig](#commit-the-kubeconfig)
1.1. [Provision databases](#provision-databases)
1.1. [Deploy the application](#deploy-the-application)
1.1. [Provision DNS](#provision-dns)
1. [Teardown](#teardown)
1.1. [Tear down DNS](#tear-down-dns)
1.1. [Delete frontend service](#delete-frontend-service)
1.1. [Delete replication controllers](#delete-replication-controllers)
1.1. [Verify pods are gone](#verify-pods-are-gone)
1.1. [Tear down database](#tear-down-database)
1.1. [Tear down Kubernetes](#tear-down-kubernetes)
1.1. [Delete file storage](#delete-file-storage)

# Bootstrap

For now, we deploy our clusters onto AWS.

## Set up AWS

[Install the AWS tool](https://docs.aws.amazon.com/cli/latest/userguide/installing.html).
If you want to do this on your own user account, create an IAM user with AdministratorAccess.
Otherwise, ask a team member for credentials for the shared account.
Configure your AWS client with those credentials.

```
$ aws configure
```

Confirm it works.

```
$ aws s3 ls /
```

## Set up kubectl

You interact with Kubernetes clusters via the kubectl tool.
Download it with the get-kubectl.bash script.

```
$ ./get-kubectl.bash
$ mv kubectl $HOME/bin # or whatever
```

A cluster is defined by a configuration 3-tuple: a cluster, including the Kubernetes master IP; a user, including credentials; and a context, binding them together with a specific name.
Each cluster foo should have a corresponding foo.kubeconfig checked in to revision control.
To interact with a cluster, use kubectl --kubeconfig.

```
$ kubectl --kubeconfig=foo.kubeconfig get pods
```

> 💁
> There are more sophisticated ways to manage multiple clusters and kubeconfigs.
> See [this Kubernetes documentation](http://kubernetes.io/v1.0/docs/user-guide/kubeconfig-file.html) for more info.

## Script overview

Here's how the scripts work.

```
      +------------------+
      | config-base.bash |--.                           +--kubernetes--------++
      +------------------+  |                           |  +--cluster------+  |   +-----+
   +--------------------+   v      +-----------------+  |  |  +--aws----+.-|--|-->|     |
-->| provision.bash foo |---+--+-->| get-k8s-io.bash |--|--|--|-->*.sh  |--|--|-->| AWS |
   +--------------------+      ^   +-----------------+  |  |  +---------+'-|--|-->|     |
          +-----------------+  |                        |  +---------------+  |   +-----+
          | config-foo.bash |--'                        +---------------------+
          +-----------------+

```

## Get kubernetes.bash

The core bootstrapping script, get-k8s-io.bash, is provided and maintained by the Kubernetes project.
We make a couple of modifications, to make it more failsafe.

```
$ ./get-bootstrapping-script.bash
```

If this causes local modifications, please make a PR for them.

## Run provision.bash

To create a new cluster named e.g. foo, the provisioning script expects to find a **config-foo.bash** file with settings.
Create that file for your cluster, using an existing file as a template.
Then, run the script.

```
$ ./provision.bash foo
```

This will take several minutes.

> 💁
> The script moves your existing ~/.kube/config to ~/.kube/config.backup.TIMESTAMP.

## Verify the cluster

To verify the cluster, we'll deploy an application, rolling-upgrade it to a new version, and then tear it all down.
We assume you have a working Go compiler and Docker installed.

```
$ go version
$ docker ps
```

We will work in the helloworld directory.

```
$ cd helloworld
```

Create the first version of your application.
Compile helloworld.go for Linux.

```
$ env GOOS=linux GOARCH=amd64 go build -o helloworld .
```

Build the Docker container.
Kubernetes will eventually need to download this container, so we'll put it on Docker Hub.
That means we should tag it as **yourname**/helloworld, where **yourname** is your Docker Hub username.
We'll also use an explicit version tag, 1.0.0.

```
$ docker build -t yourname/helloworld:1.0.0 .
```

Push the Docker container to Docker Hub.
This requires you to have an account on Docker Hub, and login with the docker CLI.
See [this documentation](https://docs.docker.com/reference/commandline/login/) for details.

```
$ docker login
$ docker push yourname/helloworld:1.0.0
```

Now we will tell Kubernetes to download and run this container.
First, edit the helloworld-rc.yaml to use the container with the correct tag.

```
$ sed -i'.bak' 's/yourname/peterbourgon/g' helloworld-rc.yaml ; rm -f *.bak
```

Then, tell Kubernetes to create a new replication controller from the file.

```
$ kubectl create -f helloworld-rc.yaml
```

Check that it was created.

```
$ kubectl get rc
CONTROLLER         CONTAINER(S)   IMAGE(S)                                  SELECTOR                                    REPLICAS
helloworld-1.0.0   helloworld     docker.io/peterbourgon/helloworld:1.0.0   app=helloworld,track=stable,version=1.0.0   1
```

Check that a pod is running.

```
$ kubectl get pods
NAME                     READY     STATUS    RESTARTS   AGE
helloworld-1.0.0-uxnyk   1/1       Running   0          1m
```

Use kubectl to forward a port from your local machine to the pod directly.
Then, curl that port to see your pod is working.
Note you need to copy/paste the specific pod name from the above step.

```
$ kubectl port-forward -p helloworld-1.0.0-uxnyk 10000:80
$ curl -Ss -XGET localhost:10000
Hello world
```

Scale the number of replicas up to 3.

```
$ kubectl scale --replicas=3 rc helloworld-1.0.0
scaled
```

Verify.

```
$ kubectl get rc
CONTROLLER         CONTAINER(S)   IMAGE(S)   SELECTOR   REPLICAS
helloworld-1.0.0   helloworld     ...        ...        3
```

To expose this application to the world, we need to create a Kubernetes service.
A service bridges a set of pods (matching some label query) and a load balancer endpoint.
Kubernetes automatically uses the load balancer of the underlying platform; in our case, an ELB.
Our helloworld service will match all app=helloworld pods, ignoring all other label dimensions like version.

```
$ kubectl create -f helloworld-svc.yaml
```

Inspect the service until you see the ELB endpoint that was created.
It may take several minutes to appear in the output.

```
$ kubectl describe svc helloworld
Name:                   helloworld
Namespace:              default
Labels:                 app=helloworld
Selector:               app=helloworld
Type:                   LoadBalancer
IP:                     10.0.254.122
LoadBalancer Ingress:   ab1896c8f7eff11e58b1502f93cffe5e-1066700612.us-west-2.elb.amazonaws.com
Port:                   <unnamed>       80/TCP
NodePort:               <unnamed>       30088/TCP
Endpoints:              10.244.3.15:80
Session Affinity:       None
No events.
```

In another terminal, set up a loop to continuously GET the ELB.
We'll use that to verify the version upgrade works as expected.

```
$ bash -c 'while true; do curl -Ss -XGET ab1896c8f7eff11e58b1502f93cffe5e-1066700612.us-west-2.elb.amazonaws.com; sleep 0.5; done'
Hello world
Hello world
Hello world
```

Now, we'll deploy a new version of our application.
Change helloworld.go to print "Foo bar" instead of "Hello world".
Recompile, rebuild the Docker container as version 2.0.0, and push it to Docker Hub.

```
$ sed -i'.bak' 's/Hello world/Foo bar/g' helloworld.go ; rm -f *.bak
$ env GOOS=linux GOARCH=amd64 go build -o helloworld .
$ docker build -t yourname/helloworld:2.0.0 .
$ docker push yourname/helloworld:2.0.0
```

Modify the replication controller to control the 2.0.0 container.

```
$ sed -i'.bak' 's/1.0.0/2.0.0/g' helloworld-rc.yaml ; rm -f *.bak
```

Now, let's do a rolling update, from 1.0.0 to 2.0.0.
We'll wait 3s between starting a new pod and killing an old one.
In production, you want to wait longer, 1m or more.

```
$ kubectl rolling-update helloworld-1.0.0 -f helloworld-rc.yaml --update-period=3s
```

In your other terminal, you should see "Hello world" and "Foo bar" interleaved, and then only "Foo bar".
All done.
Now, let's tear everything down.

```
$ kubectl delete svc helloworld
$ kubectl delete rc helloworld-2.0.0
$ git checkout -- helloworld-rc.yaml
```

No pods left.

```
$ kubectl get pods
NAME      READY     STATUS    RESTARTS   AGE
```

## Commit the kubeconfig

The Kubernetes script wrote user, cluster, and context settings to your ~/.kube/config.
The provisioning script has copied this file to **foo.kubeconfig**.
Now that we've verified Kubernetes is working, you should commit this file, to allow your teammates to use it.
Others may access your cluster via e.g.

```
$ kubectl --kubeconfig=foo.kubeconfig get pods
```

> 💁
> There are probably security considerations here, which I am electing to ignore.

## Provision databases

TODO.

- Terraform?
- Data migration?

## Deploy the application

See parent directory.

## Provision DNS

TODO

```
+-----------------------+            +-----+        +------------------+        +---------------+
| foo.cloud.weave.works |--Route53-->| ELB |--k8s-->| frontend service |--k8s-->| frontend pods |
+-----------------------+            +-----+        +------------------+        +---------------+
```

- Create frontend service (don't necessarily need pods yet)
- Get ELB from k8s
- Use Route53 to point CNAME to ELB

# Teardown

This is a manual process.
Configure your AWS client to the appropriate region, with the correct credentials.

## Tear down DNS

```
$ TODO
```

## Delete frontend service

```
$ TODO
```

## Delete replication controllers

```
$ TODO
```

## Verify pods are gone

```
$ TODO
```

## Tear down database

```
$ TODO
```

## Tear down Kubernetes

Replace foo with your cluster name in the below command.

```
$ ./provision.bash foo down
```

## Delete file storage

Replace foo with your cluster name in the below command.

```
$ bash -c '
  for b in $(aws s3 ls / | grep weaveworks-scope-kubernetes-foo | awk "{print $3}")
  do
    echo $b
    aws s3 rm --recursive s3://$b/
    aws s3 rb s3://$b
  done
'
```
