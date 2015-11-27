# DEPRECATED

These instructions refer to a previous version of the infrastructure.
For the up-to-date version, see [the imfra directory](/infra).

---

!!!!!DO NOT RUN THESE COMMANDS UNLESS YOU KNOW WHAT YOU'RE DOING!!!!!

# The Infrastructure

This directory contains configuration and scripts for bringing up 'the infrastructure'.
'The infrastructure' is a collection of VMs running various services which allow us to
run 'the service' in production.  'The infrastructure' also encompasses DNS records so
users can find the service and hosts in 'the infrastructure' can find each other.

The various 'infrastructure services' are:
- Docker, so we can run containers on each host
- Weave, so these containers can find & talk to each other, across hosts.
- Consul, so that the Swarm master can find the docker engines on each host
  (Consul not to be used for anything else, consider it part of swarm).
- Swarm, so we have a single Docker endpoint to run jobs across the cluster.
- Scope, to visualise this whole mess.

# Lifecycle

Currently we only support creating an instance of 'the infrastructure'.

Upgrades, config changes are TODO.

# Environments

There are 2 configured environments:
- 'dev': a set of VMs you can play with.  Use connect.sh -dev to get access.
- 'prod': do not play here.  Use connect.sh -prod to get access.

# Bringing the infrastructure up

Note this is pretty much a one time operation, and has been done.  Do not run these
commands again, you will end up deleting everything.

## Prerequisites

- You will need terraform installed (https://www.terraform.io/downloads.html)

## Bring up infrastructure (VMs, DNS etc):

This step create the required VMs & Route53 zones on AWS, and installs Docker on those VMs.
You only want to do this when bringing up a new cluster.

```
# cd terraform; terraform apply -state={prod/dev}.tfstate -var-file={prod/dev}.tfvars
```

Note this step will potentially destroy all the VMs you have running and take the service
offline.  Do not run it, unless you know what you are doing.

Make a note of the database URIs output from this, as you'll need them
to finish configuring the databases later.

## Configure Google DNS

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

## Build and push the custom Consul images

(If it has been updated at all)

```
# docker build -t weaveworks/consul consul/
# docker push weaveworks/consul
```

## Bring up infrastructure services

This step bring up the various infrastructure services (Weave, Scope, Consul and Swarm).

Note, it may take some time for the DNS records to propagate, so before running these commands, you need to ensure:

```
# dig +short docker.{dev/cloud}.weave.works
```

returns the expected IP addresses.

These steps will bounce the related service so should never be run on the production system.
They are pretty much only useful for blowing away and bringing up a new environment.

```
# cd services; ./service.sh (-prod) weave
# cd services; ./service.sh (-prod) scope
# cd services; ./service.sh (-prod) consul
# cd services; ./service.sh (-prod) swarm
```

## Finishing Database Setup

Note you only need to do this if you have destroyed & recreated the RDS instance, which you should never do!

### Loading database schemas

At the moment, loading the database schemas is a manual process.
You'll need to connect to the RDS instance, and run the commands from
`users/db/schema.sql`, or `app-mapper/db/schema.sql`.

The "easiest" way to do this is to use `./connect.sh`, and run a
docker container, as in:

```
$ ./connect.sh -ENV
```

And in a different window:

```
$ export DOCKER_HOST=localhost:4567
$ cat SCHEMA_FILE_HERE | docker run -i --rm -e PGPASSWORD=RDS_PASSWORD_HERE postgres psql -U postgres -h RDS_INSTANCE_ADDRESS_HERE
```

### Configure Containers to point at new RDS instances

You'll also need to change the container tfvars file to point at the
new RDS instances, and redeploy the changed containers.
