#!/bin/bash

set -eu

SCRIPT_DIR=`dirname "$0"`

case "$1" in
	-prod)
	ENVIRONMENT="prod"
	shift 1
	;;
	-dev)
	ENVIRONMENT="dev"
	DOMAIN="dev.weave.works"
	shift 1
	;;
	*)
	echo "Please specify environment!"
	exit 1
	;;
esac

TFSTATE_FILE="${SCRIPT_DIR}/../terraform/${ENVIRONMENT}.tfstate"
JQ_KEY_PREFIX='.modules[0].resources["aws_iam_access_key.registry"].primary.attributes'
ACCESS_KEY=$(jq  -r "${JQ_KEY_PREFIX}.id" < "$TFSTATE_FILE")
SECRET_KEY=$(jq  -r "${JQ_KEY_PREFIX}.secret" < "$TFSTATE_FILE")
if [ "$ENVIRONMENT" = "dev" ]; then
    BUCKET_NAME="weaveworks_registry_dev"
else
    BUCKET_NAME="weaveworks_registry"
fi

echo "-e REGISTRY_STORAGE=s3 -e REGISTRY_STORAGE_S3_BUCKET=$BUCKET_NAME -e REGISTRY_STORAGE_S3_REGION=us-east-1 -e REGISTRY_STORAGE_S3_ACCESSKEY=$ACCESS_KEY -e REGISTRY_STORAGE_S3_SECRETKEY=$SECRET_KEY"
