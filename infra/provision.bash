#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if [ $# -ne 2 ]
then
	echo "usage: $(basename $0) <name> up/down"
	exit 1
fi

NAME=$1
WHAT=$2

ACCESS_KEY=$(aws configure get aws_access_key_id)
echo "Using AWS access key ID ${ACCESS_KEY}"

source config-base.bash
source config-${NAME}.bash
source get-k8s-io.bash


case $WHAT in
terraform)
	KUBE_ROOT=$(dirname "${BASH_SOURCE}")/kubernetes
	source "${KUBE_ROOT}/cluster/kube-env.sh"
	source "${KUBE_ROOT}/cluster/${KUBERNETES_PROVIDER}/util.sh"
	VPCID=$(get_vpc_id)
	echo VpcId = ${VPCID}
	;;

up)
	if [ -f ${HOME}/.kube/config ]
	then
		echo
		TIMESTAMP=$(date +%Y%m%d%H%M%S)
		echo "Detected ${HOME}/.kube/config"
		echo "Moving it to ${HOME}/.kube/config.backup.${TIMESTAMP}"
		mv ${HOME}/.kube/config ${HOME}/.kube/config.backup.${TIMESTAMP}
		echo
	else
		echo
		echo "No ${HOME}/.kube/config detected"
		echo "That's fine"
		echo
	fi
	create_cluster
	if [ -f ${HOME}/.kube/config ]
	then
		echo
		echo "Writing ${NAME}.kubeconfig"
		cp ${HOME}/.kube/config ${NAME}.kubeconfig
		echo
	else
		echo
		echo "${HOME}/.kube/config is missing"
		echo "This is very strange and/or bad"
		echo
	fi
	;;

down)
	while true
	do
		read -p "Are you sure you want to bring $NAME down? " yn
		case $yn in
			yes) break;;
			no) exit;;
			*) echo "Please type 'yes' or 'no'";;
		esac
	done
	kubernetes/cluster/kube-down.sh
	;;
esac
