#!/bin/bash

set -eu

if [ $# -ne 1 ]; then
	echo "Usage: $0 <host>"
	exit 1
fi

HOST=$1
FRONTEND_IP=$(ssh $HOST weave dns-lookup frontend)
if [ -z "$FRONTEND_IP" ]; then
	echo "Could not find frontend.weave.local: is it running?"
	exit 1
fi

echo "Starting proxy container..."
PROXY_CONTAINER=$(ssh $HOST weave run -d --add-host=run.weave.works:$FRONTEND_IP weaveworks/proxy)

function finish {
	echo "Removing proxy container.."
	ssh $HOST docker rm -f $PROXY_CONTAINER
}
trap finish EXIT

PROXY_IP=$(ssh $HOST -- "docker inspect --format='{{.NetworkSettings.IPAddress}}' $PROXY_CONTAINER")

echo 'Please configure your browser for proxy http://localhost:8080/proxy.pac'
ssh -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 $HOST docker attach $PROXY_CONTAINER
