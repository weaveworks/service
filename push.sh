#!/bin/bash

set -u

usage() {
    echo "Usage: $0 (-dev|-prod) [components...]"
}

COMPONENTS=
ENV_SET=

while [ $# -gt 0 ]; do
	case "$1" in
		-prod)
		SSH_ARGS="-i infrastructure/prod-keypair.pem"
		ENV_SET=1
		;;
		-dev)
		SSH_ARGS="-i infrastructure/dev-keypair.pem"
		ENV_SET=1
		;;
		*)
		COMPONENTS="$COMPONENTS $1"
		;;
	esac
	shift 1
done

if [ -z "$ENV_SET" ]; then
	usage
	exit 1
fi

if [ -z "$COMPONENTS" ]; then
	COMPONENTS="app-mapper ui-server users frontend monitoring"
fi

echo Pushing $COMPONENTS to remote registry...

# Create a ssh tunnel and trick the docker daemon into thinking that the
# registry is running locally at localhost:5000
LOCAL_REGISTRY_PORT=5000
REGISTRY_HOST=registry.weave.local
ssh $SSH_ARGS -N -L $LOCAL_REGISTRY_PORT:$REGISTRY_HOST:80 $HOST &
sleep 2 # give time for ssh to connect
SSH_PID=$!
trap 'kill $SSH_PID' EXIT
for COMPONENT in $COMPONENTS; do
	LOCAL_REGISTRY_IMAGE=localhost:$LOCAL_REGISTRY_PORT/$COMPONENT
	docker tag -f $REGISTRY_HOST/$COMPONENT $LOCAL_REGISTRY_IMAGE
	docker push $LOCAL_REGISTRY_IMAGE
	docker rmi $LOCAL_REGISTRY_IMAGE
done
