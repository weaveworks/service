#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 (-prod|-dev|<host>)"
	exit 1
fi

kubectl_on() {
    ssh $SSH_ARGS $HOST kubectl "$@"
}

function k8s_finish {
    echo "Removing proxy service.."
    kubectl_on delete rc fons-proxy
    kubectl_on delete service fons-proxy
}


LOCAL_PORT=4567
if [ "$1" == "-prod" ]; then
    HOST=ubuntu@$(dig +short docker.cloud.weave.works | sort | head -1)
    SSH_ARGS="-i infrastructure/prod-keypair.pem"
    LOCAL_PORT=4501
    DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
    DOCKER_CONFIG="-H=tcp://$DOCKER_IP_PORT"
elif [ "$1" == "-dev" ]; then
    HOST=ubuntu@$(dig +short docker.dev.weave.works | sort | head -1)
    SSH_ARGS="-i infrastructure/dev-keypair.pem"
    LOCAL_PORT=4502
    DOCKER_IP_PORT=$(ssh $SSH_ARGS $HOST weave dns-lookup swarm-master):4567
    DOCKER_CONFIG="-H=tcp://$DOCKER_IP_PORT"
else
    # Use local k8s cluster
    # TODO: extend it to -prod and -dev
    HOST=$1
    shift 1
    SSH_ARGS=
    LOCAL_API_SERVER_PORT=9090
    # TODO: contacting the API server will be different in non-local k8s. We will
    # probably want to use 'kubectl proxy' to handle authentication and so on ...
    API_SERVER_PORT=8080

    k8s_finish 2>/dev/null || true
    echo "Starting proxy service..."

    # TODO: It probably makes more sense to use something like https://github.com/robszumski/k8s-service-proxy instead of
    # weaveworks/socksproxy
    kubectl_on run $USER-proxy --image=weaveworks/socksproxy --command -- /proxy -a scope.weave.works:frontend
    trap k8s_finish EXIT

    # Override needed because 'kubectl expose' doesn't allow providing multiple ports
    PROXY_SERVICE_PORTS=$(printf %q '{ "apiVersion": "v1", "spec": { "ports": [{"name": "socks", "port":8080}, {"name": "pac", "port":8000}] } }')
    kubectl_on expose rc $USER-proxy --port=8080 --overrides="$PROXY_SERVICE_PORTS"
    PROXY_IP=$(kubectl_on get service $USER-proxy -o template --template='{{.spec.clusterIP}}')
    PROXY_NAME_TEMPLATE=$(printf %q '{{ (index .items  0).metadata.name }}')
    PROXY_POD=$(kubectl_on get pod --selector=run=$USER-proxy -o template --template="$PROXY_NAME_TEMPLATE")
    echo "Please configure your browser for proxy http://localhost:8080/proxy.pac and"
    echo "provide '-s localhost:$LOCAL_API_SERVER_PORT' to kubectl"
    ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 -L$LOCAL_API_SERVER_PORT:localhost:$API_SERVER_PORT $HOST \
	kubectl logs -f $PROXY_POD -c $USER-proxy
    exit 0
fi
shift 1

docker_on() {
    ssh $SSH_ARGS $HOST docker $DOCKER_CONFIG "$@"
}

echo "Starting proxy container..."
docker_on rm -f $USER-proxy 2>/dev/null || true
docker_on run -d --name $USER-proxy -l works.weave.role=system weaveworks/socksproxy -a scope.weave.works:frontend.weave.local

function finish {
    echo "Removing proxy container.."
    docker_on rm -f $USER-proxy
}
trap finish EXIT

PROXY_IP=$(ssh $SSH_ARGS $HOST weave dns-lookup $USER-proxy)
echo "Please configure your browser for proxy http://localhost:8080/proxy.pac and"
echo "export DOCKER_HOST=tcp://127.0.0.1:$LOCAL_PORT"
ssh $SSH_ARGS -L8000:$PROXY_IP:8000 -L8080:$PROXY_IP:8080 -L$LOCAL_PORT:$DOCKER_IP_PORT $HOST \
    docker $DOCKER_CONFIG attach $USER-proxy
