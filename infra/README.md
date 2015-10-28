# infra

```
+-------+ --.
|  EC2  |   |
+-------+   | infra
+-------+   |
|  k8s  |   |
+-------+ --'
+-------+
|  App  |
+-------+
```

The **infra** deals with everything between our metal (EC2, GCE, ...) and our scheduling platform (k8s).

1. [Bootstrap a new cluster](#bootstrap-a-new-cluster)
1. [Maintain an existing cluster](#maintain-an-existing-cluster)
1. [Tear down an old cluster](#tear-down-an-old-cluster)

# Bootstrap a new cluster

For now, we deploy onto EC2.

## Set up AWS

- Install the AWS tool
- Configure your credentials
- Run these test commands

## Set up kubectl

- Install the kubectl tool
- Run these test commands

## Run the bootstrapping script

- See http://kubernetes.io/v1.0/docs/getting-started-guides/aws.html
- Inspect our ec2/config-default.sh
- Override defaults as necessary
- Run it in this way
- Common errors?

## Verify the cluster

- Create a helloworld rc
- Create a helloworld svc
- curl the ELB
- Migrate to the next version
- Tear it all down

## Set up any CNAME

- k8s.yourname.weave.works, or whatever, in Route53
- Deploy a (mock?) frontend

# Maintain an existing cluster

## Add EC2 instances

TODO

## Swap out an EC2 instance

TODO

# Tear down an old cluster

TODO
