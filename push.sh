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
		HOSTS=$(dig +short docker.cloud.weave.works)
		SSH_ARGS="-i infrastructure/prod-keypair.pem"
		ENV_SET=1
		;;
		-dev)
		HOSTS=$(dig +short docker.dev.weave.works)
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

echo Pushing $COMPONENTS to $HOSTS...


for COMP in $COMPONENTS; do
    IMAGE="quay.io/weaveworks/$COMP:latest"
    echo Pushing $COMP ...
    docker push $IMAGE
    # Workaround for https://github.com/docker/swarm/issues/374 :(
    for HOST in $HOSTS; do
	echo Pulling $COMP in $HOST ...
	ssh $SSH_ARGS ubuntu@$HOST docker pull $IMAGE
    done
done
