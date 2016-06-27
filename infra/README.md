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

Download Terraform [zip file](https://terraform.io/downloads.html) or check if your package manager has a recent version.
Homebrew does, so if you are on OS X, run `brew install terraform`. Be sure to use a recent version.

Download the kubectl (1.1.1) tool
 ([Linux](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/linux/amd64/kubectl),
  [Darwin](https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/darwin/amd64/kubectl)).
Put it in your PATH.
Each cluster will have a kubeconfig file checked in.
To interact with a cluster, use kubectl --kubeconfig.

```
$ kubectl --kubeconfig=infra/<env>/kubeconfig get pods
```

Note that there are more sophisticated ways to manage multiple clusters and kubeconfigs, but we currently prefer
the way with checked config and explicit path.
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
cd infra
mkdir foo
cp var.template foo/var

# Edit foo/var with your cluster's config.
# You can fill in everything except the ELB info.
# That will come later.

./k8s up foo
./tfgen foo
./rds up foo
./database bootstrap foo
./dynamodb up foo
./sqs up foo

# Next extract and use the URL encoded credentials for the users that were created,
# providing them to the appropriate components (collection, query, control):

./iam foo report_writer  # user who can write to DynamoDB, for collection service
./iam foo report_reader  # user who can read from DynamoDB, for query service
./iam foo sqs_readwriter # user who can read from and write to SQS, for control service

# Deploy the application on Kubernetes.
# Get the address of the frontend ELB, via kubectl describe svc frontend.
# Get the zone ID of the ELB, via aws elb describe-load-balancers.
# Put these values in the foo/var file.

./tfgen foo  # Copies the ELB information to a tfvars file.
./r53 up foo # Provisions Route53 to point to the ELB.

git add foo/*
git commit -m "Stand up foo cluster"

# If recreating dev/prod cluster, please make sure that the {dev,prod}.weave.works NS records
# in CloudFlare are in sync with the corresponding Route53 zones
```


## Teardown

```
./r53 down foo
./rds down foo
./k8s down foo
./dynamodb down foo
./sqs down foo

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

### How can I add nodes to a cluster?

1. Log in to the appropriate AWS console
1. Go to the EC2 autoscaling group for the Kubernetes nodes
1. Increase the min/max/desired equally to **the same number**
1. Wait a few minutes, and then confirm it's worked via `kubectl get nodes`

### How can I remove nodes from a cluster?

Note that this hasn't yet been attempted.
Follow [this guide](https://sttts.github.io/kubernetes/api/kubectl/2016/01/13/kubernetes-node-evacuation.html).
You'll also need to take them out of the autoscaling group afterwards.
