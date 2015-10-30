#!/usr/bin/env bash

export KUBE_AWS_ZONE="eu-west-1"
export AWS_S3_REGION="eu-west-1"
export AWS_S3_BUCKET="weaveworks-scope-kubernetes-dev"
export AWS_IMAGE="ami-58c1cd45" # Ubuntu Vivid 15.04 DEVEL hvm:ebs-ssd (needed for eu-west-1)
export MASTER_SIZE="m3.medium"
export MINION_SIZE="t2.micro"
export NUM_MINIONS="2"
export KUBE_AWS_INSTANCE_PREFIX="kubernetes-dev"

