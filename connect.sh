#!/bin/bash

set -eu

if [ $# -lt 1 ]; then
	echo "Usage: $0 (-prod|-dev|<host>)"
	exit 1
fi

SCRIPT_DIR=$(cd `dirname -- "$0"`; pwd -P)
PROXY_NAME=$USER-proxy

if [ "$1" == "-prod" ]; then
    KUBECTL_ARGS="--kubeconfig=$SCRIPT_DIR/infra/prod.kubeconfig"
elif [ "$1" == "-dev" ]; then
    KUBECTL_ARGS="--kubeconfig=$SCRIPT_DIR/infra/dev.kubeconfig"
else
    KUBECTL_ARGS="-s http://$1:8080"
fi

kubectl_on() {
    kubectl $KUBECTL_ARGS "$@"
}

PORT_FORWARDER_PID=
finish() {
    echo "Cleaning up..."
    kubectl_on delete rc $PROXY_NAME
}

finish > /dev/null 2>&1 || true
echo "Starting http proxy..."

kubectl_on run $PROXY_NAME --image=weaveworks/socksproxy --command -- /proxy -h '*.default.svc.cluster.local' -a scope.weave.works:frontend.default.svc.cluster.local
trap finish EXIT

# Wait for replication controller to start running
while [ "$(kubectl_on get rc $PROXY_NAME -o template --template='{{.status.replicas}}' 2>&1 )" -lt 1 ]; do sleep 1; done
PROXY_POD=$(kubectl_on get pod --selector=run=$PROXY_NAME -o template --template='{{ (index .items  0).metadata.name }}')
# Wait for pod to start running
while [ "$(kubectl_on get pod $PROXY_POD -o template --template='{{.status.phase}}' 2>&1 )" != 'Running' ]; do sleep 1; done

echo "Please configure your browser to use proxy auto-config http://localhost:8080/proxy.pac and"
echo "provide '$KUBECTL_ARGS' to kubectl in order to connect to the cluster"

kubectl_on port-forward $PROXY_POD 8000:8000 8080:8080 &
kubectl_on logs -f $PROXY_POD -c $PROXY_NAME
