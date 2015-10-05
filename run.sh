#!/bin/bash

set -eux

for dir in app-mapper client users frontend monitoring; do
    make -C $dir image.tar
done

case "$1" in
  -prod)
    ENVIRONMENT="prod"
    export DOCKER_HOST=tcp://127.0.0.1:4567
    ;;
  -dev)
    ENVIRONMENT="dev"
    export DOCKER_HOST=tcp://127.0.0.1:4567
    ;;
  -local)
    ENVIRONMENT="local"
    if ! weave status > /dev/null; then
        weave launch
        weave expose
    fi
    eval $(weave env)
    ;;
  *)
    echo "Please specify environment! (-local|-dev|-prod)"
    exit 1
    ;;
esac
shift 1

for dir in app-mapper client users frontend; do
    docker load -i=$dir/image.tar
done

(cd terraform; terraform apply --var-file=$ENVIRONMENT.tfvars --state=$ENVIRONMENT.terraform.tfstate)
