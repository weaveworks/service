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

![order.png](https://i.imgur.com/LzDkR8k.png)

1. [Prerequisites](#prerequisites)
1. [Standup](#standup)
1. [Teardown](#teardown)
1. [FAQ](#faq)

## Prerequisites

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

You interact with Kubernetes clusters via the kubectl tool.
You can download the latest stable version for your platform.

```
$ bash -c '
    VERSION=$(curl -Ss -XGET https://storage.googleapis.com/kubernetes-release/release/stable.txt)
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m) ; ARCH=${ARCH/x86_64/amd64}
    URL="https://storage.googleapis.com/kubernetes-release/release/${VERSION}/bin/${OS}/${ARCH}/kubectl"
    wget -q --show-progress -O kubectl $URL
    chmod +x kubectl
'
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

## Standup

All instructions assume you're working with the **foo** cluster.
**Make sure your AWS client is configured with the correct IAM** before continuing.
If this is your first time standing up a cluster, don't just copy/paste.
Run these commands one at a time.

```
./k8s foo up
./rds foo up
./r53 foo up

git add foo.*
git commit -m "Standup foo cluster"
```

## Teardown

```
./r53 foo down
./rds foo down
./k8s foo down

git rm foo.*
git commit -m "Teardown foo cluster"
```

## FAQ

### How can I test my Kubernetes cluster is working?

See the k8s-helloworld directory.

### How do I clean up these S3 buckets?

```
$ bash -c '
  for b in $(aws s3 ls / | grep weaveworks-scope-kubernetes- | awk \'{print $3}\')
  do
    echo $b
    aws s3 rm --recursive s3://$b/
    aws s3 rb s3://$b
  done
'
```
