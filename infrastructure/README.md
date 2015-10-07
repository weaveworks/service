!!!!!DO NOT RUN THESE COMMANDS UNLESS YOU KNOW WHAT YOU'RE DOING!!!!!

# Prerequisites

- You will need terraform installed (https://www.terraform.io/downloads.html)

# Bring up infrastructure (VMs, DNS etc):

This step create the required VMs & Route53 zones on AWS, and installs Docker on those VMs.
You only want to do this when bringing up a new cluster.

```
# cd terraform; terraform apply -state={prod/dev}.tfstate -var-file={prod/dev}.tfvars
```

Note this step will potentially destroy all the VMs you have running and take the service
offline.  Do not run it, unless you know what you are doing.

# Configure Google DNS

This step tells Google DNS (where we host the weave.works zone) to recursively use the
Route53 zone you just created.  You should never need to do this, as you should never
delete the Route53 zone.

- Goto https://console.developers.google.com/
- Select project 'weaveworks (hallowed-hold-777)' in the top left
- Select 'Cloud DNS' in the left hand menu
- Select the 'weaveworks' zone in the list
- Select the 'cloud.weave.works.' NS record from the list, click the edit pen on the right
- Change the nameserver entries to match those from the AWS Route53 zone created by the above
  terraform script.

# Build and push the custom consul images

(If it has been updated at all)

# Bring up infrastructure services

This step bring up the various infrastructure services (Weave, Scope, Consul and Swarm).

Note, it may take some time for the DNS records to propagate, so before running these commands, you need to ensure:

```
# dig +short docker.cloud.weave.works
```

returns the expected IP addresses.

These steps will bounce the related service so should never be run on the production system.
They are pretty much only useful for blowing away and bringing up a new environment.

```
# cd services; ./service.sh (-prod) weave
# cd services; ./service.sh (-prod) consul
# cd services; ./service.sh (-prod) swarm
```
