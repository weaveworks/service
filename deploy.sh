#!/bin/bash

set -eu

usage() {
    echo "Usage: $0 (-local|-dev|-prod)"
}

if [ $# -lt 1 ]; then
    usage
    exit 1
fi

case "$1" in
  -prod)
    ENVIRONMENT="prod"
    echo "TODO"
    exit 1
    ;;
  -dev)
    ENVIRONMENT="dev"
    echo "TODO"
    exit 1
    ;;
  -local)
    ENVIRONMENT="local"
    kubectl create -f k8s/local/db
    kubectl create -f k8s/local/mailcatcher
    ;;
  *)
    usage
    exit 1
    ;;
esac


kubectl create -f k8s/$ENVIRONMENT
