#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 <host> [extra ssh arguments...]"
	exit 1
fi

HOST=$1
shift 1

SSH_ARGS=$@

echo "Starting proxy container..."
PROXY_CONTAINER=$(ssh $SSH_ARGS $HOST weave run -d weaveworks/socksproxy -a scope.weave.works:frontend.weave.local)

function finish {
	echo "Removing proxy container.."
	ssh $SSH_ARGS $HOST docker rm -f $PROXY_CONTAINER
}
trap finish EXIT

PROXY_IP=$(ssh $SSH_ARGS $HOST -- "docker inspect --format='{{.NetworkSettings.IPAddress}}' $PROXY_CONTAINER")
echo 'Please configure your browser for proxy http://localhost:8080/proxy.pac'
ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 $HOST docker attach $PROXY_CONTAINER
