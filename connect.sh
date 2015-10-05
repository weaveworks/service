#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 (-prod|<host>)"
	exit 1
fi

SSH_ARGS=""

if [ "$1" == "-prod" ]; then
    HOST=ubuntu@$(dig +short docker.cloud.weave.works | sort | head -1)
    SSH_ARGS="-i infrastructure/weave-keypair.pem"
    DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
else
    HOST=$1
    DOCKER_IP_PORT=127.0.0.1:12375
fi
shift 1

docker_on() {
    ssh $SSH_ARGS $HOST -- env DOCKER_HOST=tcp://$DOCKER_IP_PORT docker "$@"
}

echo "Starting proxy container..."
docker_on rm -f $USER-proxy 2>/dev/null || true
docker_on run -d --name $USER-proxy weaveworks/socksproxy -a scope.weave.works:frontend.weave.local

function finish {
	echo "Removing proxy container.."
	docker_on rm -f $USER-proxy
}
trap finish EXIT

PROXY_IP=$(ssh $SSH_ARGS $HOST weave dns-lookup $USER-proxy)
echo 'Please configure your browser for proxy http://localhost:8080/proxy.pac and'
echo 'export DOCKER_HOST=tcp://127.0.0.1:4567'
ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 -L4567:$DOCKER_IP_PORT $HOST -- \
    env DOCKER_HOST=tcp://$DOCKER_IP_PORT docker attach $USER-proxy
