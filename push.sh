#!/bin/bash

set -u

usage() {
    echo "Usage: $0 (-dev|-prod) [components...]"
}

type pv >/dev/null 2>&1 || { echo >&2 "I require pv but it's not installed.  Aborting."; exit 1; }

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
	COMPONENTS="app-mapper ui-server users frontend"
fi

echo Pushing $COMPONENTS to $HOSTS...

for host in $HOSTS; do
	for comp in $COMPONENTS; do
		IMAGE="weaveworks/$comp:latest"

		LOCALID=$(docker inspect --format='{{.Id}}' $IMAGE)
		REMOTEID=$(ssh $SSH_ARGS ubuntu@$host docker inspect --format='{{.Id}}' $IMAGE || true)
		if [ "$LOCALID" = "$REMOTEID" ]; then
			echo "- Skipping $IMAGE on $host; same as local"
			continue
		fi

		SIZE=$(docker inspect --format='{{.VirtualSize}}' $IMAGE)
		docker save $IMAGE | pv -N "$(printf "%30s" "$IMAGE")" -s $SIZE | ssh -C $SSH_ARGS ubuntu@$host docker load
	done
done
