# infra

The **infra** deals with everything between our metal (EC2, GCE, ...) and our scheduling platform (k8s).

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

1. [Basic config](#basic-config)
1. [Bootstrap a new cluster](#bootstrap-a-new-cluster)
1. [Maintain an existing cluster](#maintain-an-existing-cluster)
1. [Tear down an old cluster](#tear-down-an-old-cluster)

# Basic config

You interact with Kubernetes clusters via the kubectl tool.
Download it with the get-kubectl.bash script.

```
$ ./get-kubectl.bash
$ mv kubectl $HOME/bin  # or whatever
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


# Bootstrap a new cluster

For now, we deploy our clusters onto EC2.

## Set up AWS

- [Install the AWS tool](https://docs.aws.amazon.com/cli/latest/userguide/installing.html)
- `aws configure`
- `aws s3 ls /`

## Set up kubectl

You should have the kubectl tool already.
If not, see the [basic config](#basic-config) section.

## Get the latest bootstrapping script

The core bootstrapping script is provided and maintained by the Kubernetes project.
We make a couple of modifications, to make it more failsafe.
TODO.

## Run the provisioning script

To create a new cluster, run the provisioning script with a single argument of the name of the cluster, e.g. foo.
The script expects to find a **config-foo.bash** file with settings for your cluster.
Use an existing config file as a template.
Then, run the script.

> 💁
> The script moves your existing ~/.kube/config to ~/.kube/config.backup.TIMESTAMP.

```
$ ./provision.bash foo
```

This will take several minutes.

> 💁
> The script changes your default AWS region to the one specified in your config-foo.bash file.
> The Kubernetes bootstrapping script expects it to work that way when uploading assets to S3.
> Feel free to change it back when finished.


## Share the kubeconfig

The provisioning script wrote user, cluster, and context settings to your ~/.kube/config.
To allow others to connect to your cluster, you should copy your kubeconfig file to **foo.kubeconfig**, and check it in.

```
$ cp ~/.kube/config foo.kubeconfig
```

> 💁
> There are probably security considerations here, which I am electing to ignore.

Now, other developers may access your cluster via e.g.

```
$ kubectl --kubeconfig=foo.kubeconfig get pods
```

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

