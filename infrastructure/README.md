
Bring up infrastructure (VMs, DNS etc):

```
# cd terraform; terraform apply -var-file=staging.tfvars)
```

Bring up services:

```
# cd services; ./service.sh weave
# cd services; ./service.sh consul
# cd services; ./service.sh swarm
```
