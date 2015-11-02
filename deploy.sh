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
    ;;
  -dev)
    ENVIRONMENT="dev"
    ;;
  -local)
    ENVIRONMENT="local"
    ./spawn_local_k8s.sh
    kubectl create -f k8s/local/db
    # TODO: The following should be done in a general way also for prod and dev
    kubectl create -f k8s
    exit 0
    ;;
  *)
    usage
    exit 1
    ;;
esac
shift 1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PLANFILE=$(mktemp ${DIR}/saas.deploy.plan.XXXXXXXX)
trap 'rm -f "$PLANFILE"' EXIT

(cd terraform; terraform plan -var-file $ENVIRONMENT.tfvars -parallelism=1 -state $ENVIRONMENT.tfstate -out $PLANFILE)

while true; do
    read -p "Do you wish to apply the plan? " yn
    case $yn in
        yes ) break;;
        no ) exit;;
        * ) echo "Please type 'yes' or 'no'.";;
    esac
done

(cd terraform; terraform apply -state $ENVIRONMENT.tfstate -parallelism=1 $PLANFILE)
