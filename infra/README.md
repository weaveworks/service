# infra

The **infra** deals with everything between our metal (AWS) and our application.
It's concerned with provisioning the scheduling system (k8s), stateful storage (RDS), and DNS (Route53).

```
+-----------------------------+  --.
|             AWS             |    |
+-----------------------------+    |
+-----+ +-----+ +-----+ +-----+    |
| R53 |-| ELB | | EC2 | | RDS |    | infra
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

1. [Prerequisites](#prerequisites)
1. [Standup](#standup)
1. [Teardown](#teardown)
1. [FAQ](#faq)

## Prerequisites

[Install the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/installing.html).
If you want to do this on your own user account, create an IAM user with AdministratorAccess.
Otherwise, ask a team member for credentials for the shared account.
Configure your AWS client with those credentials and confirm it works.
Apart from the AWS keys, you should set a region, which should match the region of the existing/desired Kubernetes cluster.

```
$ aws configure
$ aws s3 ls /
```

[Download the Terraform tool](https://terraform.io/downloads.html).
You don't need the patched version anymore, as we're not trying to interact with Docker or Docker Swarm directly.

Download the kubectl (1.1.1) tool
 ([Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl),
  [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl)).
Put it in your PATH.
Each cluster will have a kubeconfig file checked in.
To interact with a cluster, use kubectl --kubeconfig.

```
$ kubectl --kubeconfig=foo/kubeconfig get pods
```

Note that there are more sophisticated ways to manage multiple clusters and kubeconfigs.
See [this Kubernetes documentation](http://kubernetes.io/v1.1/docs/user-guide/kubeconfig-file.html) for more info.

You will also need **jq**: `apt-get install jq` or `brew install jq`.

## Standup

Each cluster is represented by a subdirectory in infra with the same name as the cluster.
In each subdirectory, there is a file called var, which contains all the necessary config.
In this example, we will be using a cluster called **foo**.
Please change foo to dev, prod, etc. as appropriate.
If this is your first time standing up a cluster, don't just copy/paste.
Run these commands one at a time.

```
mkdir foo
cp var.template foo/var

# Edit foo/var with your cluster's config.
# You can fill in everything except the ELB info.
# That will come later.

./k8s up foo
./tfgen foo
./rds up foo
./database bootstrap foo

# Deploy the application on Kubernetes.
# Get the address of the frontend ELB, via kubectl get svc.
# Get the zone ID of the ELB, via aws elb describe-load-balancers.
# Put these values in the foo/var file.

./tfgen foo  # Copies the ELB information to a tfvars file.
./r53 up foo # Provisions Route53 to point to the ELB.

git add foo/*
git commit -m "Stand up foo cluster"
```

## Teardown

```
./r53 down foo
./rds down foo
./k8s down foo

git rm -rf foo/
git commit -m "Tear down foo cluster"
```

## FAQ

### How can I test my Kubernetes cluster is working?

See the helloworld directory.

### How can I debug Kubernetes?

`kubectl get events -w` is a good place to start.
