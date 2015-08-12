#!/bin/bash

set -euo pipefail

if ! weave status > /dev/null; then
    weave launch
fi
eval $(weave env)

FLAGS="-tags integration -timeout 30s"

if [ -n "${TEST_FILTER+x}" ]; then
    FLAGS="$FLAGS -run $TEST_FILTER"
fi

docker-compose up -d
trap "docker-compose stop; docker-compose rm -f" EXIT SIGTERM SIGINT

sleep 3 # wait for the db container to start
docker run --rm \
        -v /var/run/weave.sock:/var/run/weave.sock \
	-v "$GOPATH":/go/ \
	--workdir /go/src/github.com/weaveworks/service/app-mapper \
	golang:1.4 \
	/bin/bash -c "go test  $FLAGS ./..."

