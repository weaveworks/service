#!/usr/bin/env bash

export KUBE_AWS_ZONE="eu-central-1a" # availability zone
export AWS_S3_REGION="eu-central-1"  # region
export AWS_S3_BUCKET="weaveworks-scope-kubernetes-dev-$(date | shasum | cut -c 1-7)"
export AWS_IMAGE="ami-58c1cd45" # http://cloud-images.ubuntu.com/locator/ec2/
export MASTER_SIZE="m3.medium"
export MINION_SIZE="t2.micro"
export NUM_MINIONS="2"
export KUBE_AWS_INSTANCE_PREFIX="kubernetes_dev"

