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
  - [Set up AWS](#set-up-aws)
  - [Set up kubectl](#set-up-kubectl)
  - [Update get-k8s-io.bash](#update-get-k8s-iobash)
  - [Run provision.bash](#run-provisionbash)
  - [Verify the cluster](#verify-the-cluster)
  - [Commit the kubeconfig](#commit-the-kubeconfig)
  - [Provision databases](#provision-databases)
  - [Deploy the application](#deploy-the-application)
  - [Provision DNS](#provision-dns)
1. [Teardown](#teardown)
  - [Tear down DNS](#tear-down-dns)
  - [Delete frontend service](#delete-frontend-service)
  - [Delete replication controllers](#delete-replication-controllers)
  - [Verify pods are gone](#verify-pods-are-gone)
  - [Tear down database](#tear-down-database)
  - [Tear down Kubernetes](#tear-down-kubernetes)
  - [Delete file storage](#delete-file-storage)

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
  | config-base.bash |--.                           +-cluster/aws--+
  +------------------+  |                           |   +------+   |   +-----+
   +----------------+   v      +-----------------+  |   |      |---|-->|     |
-->| provision.bash |---+--+-->| get-k8s-io.bash |--|-->| *.sh |---|-->| AWS |
   +----------------+      ^   +-----------------+  |   |      |---|-->|     |
      +-----------------+  |                        |   +------+   |   +-----+
      | config-foo.bash |--'                        +--------------+
      +-----------------+
```

## Update get-k8s-io.bash

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
$ ./provision.bash foo up
```

This will take several minutes.
See [K8S-AWS.md][] for a description of what the script does on AWS.

> 💁
> The script moves your existing ~/.kube/config to ~/.kube/config.backup.TIMESTAMP.

## Verify the cluster

To verify the cluster, we'll deploy an application, rolling-upgrade it to a new version, and then tear it all down.
See [VERIFY.md][] for instructions.

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
