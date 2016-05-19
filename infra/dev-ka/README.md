# Basic workflows

## Create a new cluster

There is probably a cluster already when you are reading this, but anyhow here is how you'd create one if there wasn't one:
```
make -C infra/dev-ka/terraform init
./infra/dev-ka/get_kubeconfig > ./infra/dev-ka/kubeconfig
./infra/dev-ka/connect
```

In another terminal create the default addon (right now this is only SkyDNS):
```
kubectl --kubeconfig infra/dev-ka/kubeconfig create -f "https://raw.github.com/weaveworks/kubernetes-anywhere/master/docker-images/toolbox/resources/addons-v1.2.yaml"
```

You can use `kubectl --kubeconfig infra/dev-ka/kubeconfig create -f dev-ka` to deploy all of the services, or do anything else you are intending to do with the new cluster.

## Destroy the cluster

```
make -C infra/dev-ka/terraform destroy
```

# Other workflows

> This is work-in-progress, the workflows are to be refined

## 1. Create a new cluster, without modifying the exiting one

```
git checkout -b my-new-cluster
```

Edit `infra/dev-ka/terraform/main.tf` and change the `cluster` parameter to something else, only alpahnumeric characters are allowed, e.g. `sam01`.

Update modules:
```
make -C infra/dev-ka/terraform modules
```

Create the cluster:
```
make -C infra/dev-ka/terraform init
./infra/dev-ka/get_kubeconfig > ./infra/dev-ka/kubeconfig
```

Edit `connect`
./connect

## 2. Modify the cluster you have created and apply changes

Make change to either the Kubernetes Anywhere module (e.g.  [`user-data.yaml`](https://github.com/weaveworks/kubernetes-anywhere/blob/master/examples/aws-ec2-terraform/secure-v1.2-user-data.yaml)),
or any other Terraform code/module. You can use modules from different forks and branches, see `infra/dev-ka/terraform/main.tf` for syntax.

Run:

```
make -C infra/dev-ka/terraform modules
make -C infra/dev-ka/terraform reapply
```
