#!/bin/bash

set -u

usage() {
    echo "Usage: $0 (-dev|-prod) [components...]"
}

COMPONENTS=
ENVIRONMENT=

while [ $# -gt 0 ]; do
	case "$1" in
		-prod)
		SSH_ARGS="-i infrastructure/prod-keypair.pem"
		ENVIRONMENT="prod"
		;;
		-dev)
		SSH_ARGS="-i infrastructure/dev-keypair.pem"
		ENVIRONMENT="dev"
		;;
		*)
		COMPONENTS="$COMPONENTS $1"
		;;
	esac
	shift 1
done

if [ -z "$ENVIRONMENT" ]; then
	usage
	exit 1
fi

if [ -z "$COMPONENTS" ]; then
	COMPONENTS="app-mapper ui-server users frontend monitoring"
fi

echo Pushing $COMPONENTS to registry...

# Push to a local registry backed by the same s3 storage as the remote one
LOCAL_REGISTRY_PORT=5000
REGISTRY_HOST=registry.weave.local
REGISTRY_STORAGE_ARGS=$(`dirname "$0"`/infrastructure/services/registry_storage_args.sh -${ENVIRONMENT})
docker run -d --name local_registry -p $LOCAL_REGISTRY_PORT:$LOCAL_REGISTRY_PORT -e REGISTRY_HTTP_ADDR=:$LOCAL_REGISTRY_PORT $REGISTRY_STORAGE_ARGS registry:2.1.1 > /dev/null
trap 'docker rm -f local_registry' EXIT
sleep 4 # give time for the registry to connect
for COMPONENT in $COMPONENTS; do
	LOCAL_REGISTRY_IMAGE=localhost:$LOCAL_REGISTRY_PORT/$COMPONENT
	docker tag -f $REGISTRY_HOST/$COMPONENT $LOCAL_REGISTRY_IMAGE
	docker push $LOCAL_REGISTRY_IMAGE
	docker rmi $LOCAL_REGISTRY_IMAGE
done
