#!/bin/bash

set -euo pipefail

if ! weave status > /dev/null; then
    weave launch
fi
eval $(weave env)

docker-compose up -d
trap "docker-compose stop" EXIT SIGTERM SIGINT

sleep 3 # wait for the db container to start
docker run --rm \
        -v /var/run/weave.sock:/var/run/weave.sock \
	-v "$GOPATH":/go/ \
	--workdir /go/src/github.com/weaveworks/service/app-mapper \
	golang:1.4 \
	/bin/bash -c "go get golang.org/x/tools/cmd/cover && go test -tags integration -cover -coverprofile cover.out -timeout 30s ./... && go tool cover -html=cover.out -o cover.html && cat cover.html" > cover.html
