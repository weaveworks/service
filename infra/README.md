# infra

The **infra** deals with everything between our metal (AWS) and our application.
It's concerned with provisioning the scheduling system (k8s), stateful storage (RDS, DynamoDB), message bus (SQS) and DNS (Route53).
Infra also creates various IAM users to control access to these resources.

```
+--------------------------------------------------+  --.
|             AWS                                  |    |
+--------------------------------------------------+    |
+-----+ +-----+ +-----+ +-----+ +-----+ +----------+    |
| R53 |-| ELB | | EC2 | | RDS | | SQS | | DynamoDB |    | infra
+-----+ |     | +-----+ |     | +-----+ +----------+    |
        |     |    |    |     |    |       |    |       |
        |     | +-----+ |     |    o       o    o       |
        |     |-| k8s | |     |   /_\     /_\  /_\      |
        +-----+ +-----+ +-----+    |       |    |     --'
                   |       |       |       |    |
                +---------------------------------+
                |               App               |
                +---------------------------------+
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

Download Terraform [zip file](https://terraform.io/downloads.html). Be sure to use a recent version (>= 0.6.16).
Homebrew has a version of Terraform, but it wasn't the latest at the time of writing.

Download the kubectl (1.2.4) tool
 ([Linux](https://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl),
  [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/darwin/amd64/kubectl)).
Put it in your PATH.
Each cluster will have a kubeconfig file checked in.
To interact with a cluster, use `kubectl --kubeconfig`.

```
$ kubectl --kubeconfig=infra/<env>/kubeconfig get pods
```

Note that there are more sophisticated ways to manage multiple clusters and kubeconfigs, but we currently prefer
the way with checked config and explicit path.
See [this Kubernetes documentation](http://kubernetes.io/v1.1/docs/user-guide/kubeconfig-file.html) for more info.

You will also need **jq**: `apt-get install jq` or `brew install jq`.

## Standup

Each cluster is represented by a subdirectory in infra with the same name as the cluster.
In each of cluster subdirectories, there is a file called `var`, which contains the necessary config.
In this example, we will be using a cluster called `foo`.
Please change `foo` to `dev`, `prod`, etc. as appropriate.
If this is your first time standing up a cluster, don't just copy/paste.
Run these commands one at a time.

> **Do not use dashes or underscores for the name. It is used for various things, some of which
> do not handle dashes or underscores.**

Create environment `foo` and copy template files:

```
cd infra
mkdir foo
cp var.template foo/var
```

You now need to edit `foo/var`:

  - fill in all `AWS_*` parameters and `RDS_USERS_DB_PASSWORD`
  - (optionally) change `K8S_*` parameters to reflect your desired cluster

The rest of parameters will come later.

Generate an SSH key:
```
./ssh-keygen foo
```

Now run:

```
./tfgen foo
./k8s-anywhere up foo
./rds up foo
./dynamodb up foo
./sqs up foo
```

> If you are seeing any Terraform errors, be sure to repeat the command that produced that error.

Next extract and use the URL-encoded credentials for the users that were created,
providing them to the appropriate components (collection, query, control):

```
./iam foo report_writer  # user who can write to DynamoDB, for collection service
./iam foo report_reader  # user who can read from DynamoDB, for query service
./iam foo sqs_readwriter # user who can read from and write to SQS, for control service
```

You will need to obtain `kubeconfig` file with `./get-kubeconfig foo` command. To run this the cluster should be fully
operational, which takes a few minutes after `./k8s-anywhere up foo` has finished.

Run `./get-kubeconfig foo > foo/kubeconfig`.

Next, check if all is well and the number of nodes (`K8S_NODE_COUNT`) you have set in `foo/var` is what you got:
```
kubectl --kubeconfig=foo/kubeconfig get nodes
```

Deploy SkyDNS addon:
```
kubectl --kubeconfig=foo/kubeconfig create -f "https://raw.github.com/kubernetes/kubernetes-anywhere/master/phase2/docker-images/toolbox/resources/addons-v1.2.yaml"
```

Initialise the database schema:
```
./database bootstrap foo
```

Now you should deploy the application on Kubernetes.

You will need to prepare `k8s/foo` in toplevel directory, i.e. you would copy `k8s/prod` and
update the configuration to cater for the new cluster (this includes RDS, SQS, Dynamor, S3
and other credentials and endpoints).

Once the application is deployed, you need get the address of the ELB created for `frontend` service:
```
kubectl --kubeconfig=foo/kubeconfig describe svc frontend
```

Next, get the zone ID of the ELB, via `aws elb describe-load-balancers`, and put these values in the `foo/var` file,
and run the following commands:

```
./tfgen foo  # Copies the ELB information to a tfvars file.
./r53 up foo # Provisions Route53 to point to the ELB.

git add foo/*
git commit -m "Stand up foo cluster"
```

> **If recreating dev/prod cluster, please make sure that the `{dev,prod}.weave.works` NS records
> in CloudFlare are in sync with the corresponding Route53 zones. You must use exactly the same nameservers
> shown on Route53 console.**

## Teardown

For clean teardown, you will need to make sure cluster has no services of type `LoadBalancer`, as those have
an ELB and other AWS resources that are associated with the VPC.

You can either just delete all pods and services, but it's easier to delete just the services which have ELBs.

You can run this to get the list of `kubectl` commands you will need to run:
```
kubectl --kubeconfig devka/kubeconfig get svc --all-namespaces -o json | jq -r '
  .items
  | map(select(.spec.type == "LoadBalancer"))
  | .[].metadata | @sh "delete --namespace=\(.namespace) svc \(.name)"
'
```

Most likely all you will need to do is:
```
kubectl --kubeconfig foo/kubeconfig delete --namespace='default' svc frontend
```

Next, teardown the main cluster resources:

```

./k8s-anywhere down foo
```

If Terraform complains about deleting the VPC, you probably hit
[hashicorp/terraform#6994](https://github.com/hashicorp/terraform/issues/6994),
so you should go and delete the VPC manualy using either the AWS CLI or the UI.

Terraform currently doesn't delete the SSH key pair, so you will need to delete it manually. The name of
the key pair is `kubernetes-foo`. You can do this using the AWS CLI or the UI.

If all is well, you can proceed with teardown:

```
./r53 down foo
./rds down foo
./dynamodb down foo
```

> You will probably see an error message about deleting the S3 bucket, it's a known issue, which you can ignore right now and apply the work-around documented in [#578](https://github.com/weaveworks/service/issues/578#issuecomment-230285797).

```
./sqs down foo
```

Once done, commit your changes.

```
git rm -rf foo/
git commit -m "Tear down foo cluster"
```

## FAQ

### How can I test my Kubernetes cluster is working?

See the helloworld directory.

### How can I debug Kubernetes?

`kubectl get events --all-namespaces --watch` is a good place to start.

### How can I connect to the AWS console?

We use two separate AWS accounts for the dev and prod environments with
[Consolidated Billing](http://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/consolidated-billing.html)
(managed centrally from the dev account).

For dev, log in to https://weaveworks.signin.aws.amazon.com/console (the root account is tom.wilkie@weave.works).

For prod, log in to https://weaveworks-prod.signin.aws.amazon.com/console (the root account is pgm@weave.works).

If you don't have access credentials, ask a fellow developer to
[provide you with an IAM user (with password)](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html#id_users_create_console)
for the environment you want to access.

### How is data backed up and restored?

We are using the
[standard automatic backup system from AWS RDS](http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_WorkingWithAutomatedBackups.html),
which creates daily snapshots. Here's
[how to restore a DB from a snapshot](http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_RestoreFromSnapshot.html).

### How is DNS configured?

As you can see in the var configuration files, each environment creates a DNS
zone and
[an A record aliased to an ELB](http://docs.aws.amazon.com/ElasticLoadBalancing/latest/DeveloperGuide/using-domain-names-with-elb.html#dns-associate-custom-elb). This
ELB
[will normally point to the frontend Kubernetes service](http://kubernetes.io/v1.1/docs/user-guide/services.html#type-loadbalancer)
and in turn is how the Scope service is accessed from the outside world.

The A-records for dev and prod are `frontend.dev.weave.works.` and `frontend.prod.weave.works.`.

On top of that there's a CNAME record for `scope.weave.works` pointing to
`frontend.prod.weave.works` which is how the Scope service is publicly accessed
by end users.

Currently, the `weave.works` domain is manually managed in CloudFloare project. This includes:

* The `scope.weave.works` CNAME record
* The `dev.weave.works` and `prod.weave.works` NS records

### What service is used to deliver email?

We are currently using [SendGrid](https://sendgrid.com/) for sending emails to the users (e.g. welcome and password link emails).

When running locally, emails are delivered to a mailcatcher pod.

### How can I add/remove nodes to/from a cluster?

1. Change the value of `K8S_NODE_COUNT` in `foo/var`
1. Run `./tfgen foo && ./k8s-anywhere up foo`
1. Wait a few minutes, and then confirm it's worked via `kubectl get nodes`
1. If you are removing a node, please run `kubectl delete node <name>`
