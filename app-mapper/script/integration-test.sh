#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(cd `dirname -- "$0"`; pwd -P)

RM_APP_MAPPER_DB=""
finish() {
    kubectl delete pod app-mapper-test > /dev/null 2>&1
    if [ -n "$RM_APP_MAPPER_DB" ]; then
	docker rm -v -f app-mapper-db > /dev/null 2>&1
    fi
}
trap finish EXIT

get_flags() {
    local FLAGS="-tags='integration $1' -timeout 30s"
    if [ -n "${TEST_FILTER+x}" ]; then
	FLAGS="$FLAGS -run $TEST_FILTER"
    fi
    echo $FLAGS
}


run_docker_integration_tests() {
    if ! weave status > /dev/null 2>&1; then
	weave launch
    fi
    eval $(weave env)
    if [ -z "$(weave dns-lookup app-mapper-db.weave.local)" ]; then
	docker run -d --name app-mapper-db --hostname app-mapper-db.weave.local weaveworks/app-mapper-db > /dev/null
	RM_APP_MAPPER_DB=true
    fi
    docker run --rm \
	   -v /var/run/weave/weave.sock:/var/run/weave/weave.sock \
	   -v "$GOPATH":/go/ \
	   --workdir /go/src/github.com/weaveworks/service/app-mapper \
	   golang:1.5.1 \
	   /bin/bash -c "go test $(get_flags docker) ./..."
}


get_pod_phase() {
    kubectl get pod $1 --template='{{ .status.phase }}' -o template
}


run_k8s_integration_tests() {
    if ! kubectl get svc app-mapper-db > /dev/null 2>&1; then
	(cd "$SCRIPT_DIR"/../../ && ./infra/local-k8s up)
    fi

    # We need to do all the crap below because k8s doesn't nicely support
    # run-once commands nor building pods from templates

    # Spawn tests in kubernetes pod
    sed "s%\$GOPATH%${GOPATH}%g" "$SCRIPT_DIR"/app-mapper-test.json.in | sed \
        "s%\$K8S_TEST_SCRIPT%${SCRIPT_DIR}/k8s_test_script.sh%g" | sed \
        "s%\$TEST_FLAGS%$(get_flags k8s)%g"  | kubectl create -f -

    # Wait for the pod's creation
    until [ "$(get_pod_phase app-mapper-test 2>&1 )" = "Running" ]; do sleep 1; done

    kubectl logs -f app-mapper-test

    # Wait for the pod to complete execution
    while [ "$(get_pod_phase app-mapper-test)" = "Running" ]; do sleep 1; done

    # Fail in case of error
    [ "$(get_pod_phase app-mapper-test)" = "Succeeded" ] || exit 1
}


echo 'Running integration tests using the kubernetes app provisioner'
run_k8s_integration_tests

echo 'Running integration tests using the docker app provisioner'
run_docker_integration_tests
