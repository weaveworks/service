#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

NAME=$1
shift

ACCESS_KEY=$(aws configure get aws_access_key_id)
echo "Using AWS access key ID ${ACCESS_KEY}"

source config-base.bash
source config-${NAME}.bash

CURRENT_REGION=$(aws configure get region)
DESIRED_REGION=${AWS_S3_REGION}
if [ "${CURRENT_REGION}" != "${DESIRED_REGION}" ]
then
	echo "Kubernetes wants aws configure get region (${CURRENT_REGION})"
	echo "to be the same as AWS_S3_REGION (${DESIRED_REGION})."
	echo "Changing that."
	aws configure set region ${DESIRED_REGION}
else
	echo "aws configure get region (${CURRENT_REGION})"
	echo "is the same as AWS_S3_REGION (${DESIRED_REGION})."
	echo "That's fine."
fi

if [ -f $HOME/.kube/config ]
then
	local TIMESTAMP=$(date +%Y%m%d%H%M%S)
	echo "Detected ${HOME}/.kube/config"
	echo "Moving it to ${HOME}/.kube/config.backup.${TIMESTAMP}"
	mv $HOME/.kube/config $HOME/.kube/config.backup.${TIMESTAMP}
else 
	echo "No ${HOME}/.kube/config detected."
	echo "That's fine."
fi

source get-k8s-io.bash

