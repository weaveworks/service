#!/bin/bash

set -euo pipefail

if ! weave status > /dev/null 2>&1; then
    weave launch
fi
eval $(weave env)

FLAGS="-tags integration -timeout 30s"

if [ -n "${TEST_FILTER+x}" ]; then
    FLAGS="$FLAGS -run $TEST_FILTER"
fi

test -n "$(weave dns-lookup app-mapper-db.weave.local)" || \
  weave run -d --name app-mapper-db --hostname app-mapper-db.weave.local weaveworks/app-mapper-db
docker run --rm \
  -v /var/run/weave/weave.sock:/var/run/weave/weave.sock \
  -v "$GOPATH":/go/ \
  --workdir /go/src/github.com/weaveworks/service/app-mapper \
  golang:1.5.1 \
  /bin/bash -c "go test $FLAGS ./..."
