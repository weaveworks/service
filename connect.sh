#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 (-prod|<host>)"
	exit 1
fi

if [ "$1" == "-prod" ]; then
    HOST=ubuntu@$(dig +short docker.cloud.weave.works | sort | head -1)
    SSH_ARGS="-i infrastructure/weave-keypair.pem"
    DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
    DOCKER_CONFIG="-H=tcp://$DOCKER_IP_PORT"
else
    HOST=$1
    SSH_ARGS=
    DOCKER_CONFIG=$(ssh $HOST weave config)
    DOCKER_IP_PORT="127.0.0.1:12375"
fi
shift 1

docker_on() {
    ssh $SSH_ARGS $HOST docker $DOCKER_CONFIG "$@"
}

echo "Starting proxy container..."
docker_on rm -f $USER-proxy 2>/dev/null || true
docker_on run -d --name $USER-proxy weaveworks/socksproxy -a scope.weave.works:frontend.weave.local

echo "Weave exposing..."
status=$(ssh $SSH_ARGS $HOST weave ps weave:expose | awk '{print $3}' 2>/dev/null)
if [ -z "$status" ]; then
    ssh $SSH_ARGS $HOST weave expose
fi

function finish {
    echo "Removing proxy container.."
    docker_on rm -f $USER-proxy
}
trap finish EXIT

PROXY_IP=$(ssh $SSH_ARGS $HOST weave dns-lookup $USER-proxy)
echo 'Please configure your browser for proxy http://localhost:8080/proxy.pac and'
echo 'export DOCKER_HOST=tcp://127.0.0.1:4567'
ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 -L4567:$DOCKER_IP_PORT $HOST \
    docker $DOCKER_CONFIG attach $USER-proxy
