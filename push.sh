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
		HOSTS=$(dig +short docker.cloud.weave.works)
		SSH_ARGS="-i infrastructure/prod-keypair.pem"
		ENVIRONMENT=prod
		;;
		-dev)
		HOSTS=$(dig +short docker.dev.weave.works)
		SSH_ARGS="-i infrastructure/dev-keypair.pem"
		ENVIRONMENT=dev
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

echo Pushing $COMPONENTS to $HOSTS...


for COMP in $COMPONENTS; do
	IMAGE="quay.io/weaveworks/$COMP"
	echo Pushing $COMP ...
	docker tag -f $IMAGE:latest $IMAGE:$ENVIRONMENT
	docker push $IMAGE:$ENVIRONMENT
	# Workaround for https://github.com/docker/swarm/issues/374 :(
	for HOST in $HOSTS; do
		echo Pulling $COMP in $HOST ...
		ssh $SSH_ARGS ubuntu@$HOST docker pull $IMAGE:$ENVIRONMENT
		ssh $SSH_ARGS ubuntu@$HOST docker tag -f $IMAGE:$ENVIRONMENT $IMAGE:latest
	done
done
