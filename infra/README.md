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

Here, you'll find scripts to provision each piece of the cluster.
Each script produces output file(s) that are used as input to other scripts.
All output files should be checked in.

![order.png](http://i.imgur.com/l52oxHz.png)

1. [Prerequisites](#prerequisites)
1. [Standup](#standup)
1. [Teardown](#teardown)
1. [FAQ](#faq)

## Prerequisites

[Install the AWS tool](https://docs.aws.amazon.com/cli/latest/userguide/installing.html).
If you want to do this on your own user account, create an IAM user with AdministratorAccess.
Otherwise, ask a team member for credentials for the shared account.
Configure your AWS client with those credentials and confirm it works.

```
$ aws configure
$ aws s3 ls /
```

[Download the Terraform tool](https://terraform.io/downloads.html).
You don't need the patched version anymore, as we're not trying to interact with Docker or Docker Swarm directly.

Download the kubectl (1.1.1) tool:
 [Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl),
 [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl).
Put it in your PATH.
Each cluster will have a kubeconfig file checked in.
To interact with a cluster, use kubectl --kubeconfig.

```
$ kubectl --kubeconfig=foo/kubeconfig get pods
```

> 💁
> There are more sophisticated ways to manage multiple clusters and kubeconfigs.
> See [this Kubernetes documentation](http://kubernetes.io/v1.0/docs/user-guide/kubeconfig-file.html) for more info.

## Standup

All instructions assume you're working with the **foo** cluster; change it as appropriate.
We also assume your AWS client is configured with the correct IAM by default.
If this is your first time standing up a cluster, don't just copy/paste.
Run these commands one at a time.

```
mkdir foo
cp someother/var foo/var # and edit

./k8s up foo
./tfgen foo
./rds up foo
./schemaload foo
# Stand up application components
./r53 up foo

git add foo/*
git commit -m "Stand up foo cluster"
```

## Teardown

```
./r53 down foo
# Tear down application components
./rds down foo
./k8s down foo

git rm -rf foo/
git commit -m "Tear down foo cluster"
```

## FAQ

### How can I test my Kubernetes cluster is working?

See the helloworld directory.
