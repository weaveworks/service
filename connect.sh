#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 <hostname>/dev/prod"
	exit 1
fi

ENV=$1
shift

echo "Connecting to $ENV"

case $ENV in
	prod)
        LOCAL_PORT=4501
		HOST=ubuntu@$(dig +short docker.cloud.weave.works | sort | head -1)
		SSH_ARGS="-i infrastructure/prod-keypair.pem"
		DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
		DOCKER_CONFIG="-H=tcp://$DOCKER_IP_PORT"
		;;
	dev)
        LOCAL_PORT=4502
		HOST=ubuntu@$(dig +short docker.dev.weave.works | sort | head -1)
		SSH_ARGS="-i infrastructure/dev-keypair.pem"
		DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
		DOCKER_CONFIG="-H=tcp://$DOCKER_IP_PORT"
		;;
	*)
        LOCAL_PORT=4567
        HOST=$ENV
        SSH_ARGS=
		# https://github.com/weaveworks/weave/issues/1527
		# DOCKER_CONFIG=$(ssh $HOST weave config)
        DOCKER_CONFIG="-H=tcp://127.0.0.1:12375"
        DOCKER_IP_PORT="127.0.0.1:12375"
        status=$(ssh $SSH_ARGS $HOST weave ps weave:expose | awk '{print $3}' 2>/dev/null)
        if [ -z "$status" ]; then
            echo "Running 'weave expose' on $HOST"
            ssh $SSH_ARGS $HOST weave expose
        fi
		;;
esac

function docker_do {
	ssh $SSH_ARGS $HOST docker $DOCKER_CONFIG "$@"
}

echo "Starting proxy container for $USER"
docker_do rm -f $USER-proxy 2>/dev/null || true
docker_do run -d --name $USER-proxy -l works.weave.role=system weaveworks/socksproxy -a scope.weave.works:frontend.weave.local

function finish {
	echo "Removing proxy container for $USER"
	docker_do rm -f $USER-proxy
}

trap finish EXIT

PROXY_IP=$(ssh $SSH_ARGS $HOST weave dns-lookup $USER-proxy)
echo "Please configure your browser for proxy http://localhost:8080/proxy.pac and"
echo "export DOCKER_HOST=tcp://127.0.0.1:$LOCAL_PORT"
ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 -L$LOCAL_PORT:$DOCKER_IP_PORT $HOST docker $DOCKER_CONFIG attach $USER-proxy
