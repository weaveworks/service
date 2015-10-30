#!/usr/bin/env bash

export KUBE_AWS_ZONE="eu-west-1a" # availability zone
export AWS_S3_REGION="eu-west-1"  # region
export AWS_S3_BUCKET="weaveworks-scope-kubernetes-dev"
export AWS_IMAGE="ami-ef8ab698" # http://cloud-images.ubuntu.com/locator/ec2/
export MASTER_SIZE="m3.medium"
export MINION_SIZE="t2.micro"
export NUM_MINIONS="2"
export KUBE_AWS_INSTANCE_PREFIX="kubernetes_dev"

