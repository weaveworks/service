#!/bin/bash

set -eux

SSH_USER="ubuntu"
WEAVE_REV="431d04bd40c0"
SCRIPT_DIR=`dirname "$0"`

case "$1" in
	-prod)
	ENVIRONMENT="prod"
	DOMAIN="cloud.weave.works"
	shift 1
	;;
	-dev)
	ENVIRONMENT="dev"
	DOMAIN="dev.weave.works"
	shift 1
	;;
	*)
	echo "Please specify environment!"
	exit 1
	;;
esac

SSH_ARGS="-i ../${ENVIRONMENT}-keypair.pem -o StrictHostKeyChecking=no"
SYSTEM_ROLE="-l works.weave.role=system"

scp_to() {
	local host=$1
	shift 1
	scp $SSH_ARGS $1 $SSH_USER@$host:$2
}

run_on() {
	local host=$1
	shift 1
	ssh $SSH_ARGS $SSH_USER@$host "$@"
}

install_weave() {
	# Start weave on all the hosts
	HOSTS=$(dig +short docker.${DOMAIN})
	INTERNAL_IPS=$(dig +short internal.${DOMAIN})
	if [ -z "$HOSTS" ] || [ -z "$INTERNAL_IPS" ]; then
		echo "Failed to find hosts!"
		exit 1
	fi

	for host in $HOSTS; do
		run_on $host sudo curl -L https://raw.githubusercontent.com/weaveworks/weave/$WEAVE_REV/weave -o /usr/local/bin/weave
		run_on $host sudo chmod a+x /usr/local/bin/weave
		run_on $host docker pull weaveworks/weave:git-$WEAVE_REV
		run_on $host docker pull weaveworks/weaveexec:git-$WEAVE_REV
		run_on $host docker tag -f weaveworks/weave:git-$WEAVE_REV weaveworks/weave:latest
		run_on $host docker tag -f weaveworks/weaveexec:git-$WEAVE_REV weaveworks/weaveexec:latest
		run_on $host weave stop || true
		run_on $host weave launch-proxy -H tcp://0.0.0.0:12375 -H unix:///var/run/weave/weave.sock
		run_on $host weave launch-router $INTERNAL_IPS
		scp_to $host set_weave_dns.sh /tmp/set_weave_dns.sh
		run_on $host sudo bash /tmp/set_weave_dns.sh
	done
	for host in $HOSTS; do
		run_on $host weave expose
		run_on $host sudo curl -L https://github.com/weaveworks/scope/releases/download/latest_release/scope -o /usr/local/bin/scope
		run_on $host sudo chmod a+x /usr/local/bin/scope
		run_on $host scope stop || true
		run_on $host scope launch
	done
}

install_consul() {
	# Start consul on first N hosts
	CONSUL_N=3
	CONSUL_HOSTS=$(dig +short docker.${DOMAIN} | sort | head -$CONSUL_N)
	for host in $CONSUL_HOSTS; do
		run_on $host docker rm -f consul || true
	done
	i=0
	for host in $CONSUL_HOSTS; do
		run_on $host docker pull weaveworks/consul
		run_on $host DOCKER_HOST=unix:///var/run/weave/weave.sock docker run -d --name consul $SYSTEM_ROLE weaveworks/consul -bootstrap-expect $CONSUL_N -retry-join consul.weave.local -node consul$i
		i=$((i + 1))
	done
}

install_swarm() {
	# Start the swam agent on every host, and the master on one
	SWARM_HOSTS=$(dig +short docker.${DOMAIN})
	SWARM_MASTER=$(dig +short docker.${DOMAIN} | sort | head -1)

	run_on $SWARM_MASTER docker rm -f swarm-master || true
	for host in $SWARM_HOSTS; do
		run_on $host docker rm -f swarm-agent || true
	done

	for host in $SWARM_HOSTS; do
		local PROXY_IP=$(run_on $host /sbin/ifconfig eth0 | grep "inet addr:" | cut -d: -f2 | awk '{ print $1 }')
		run_on $host DOCKER_HOST=unix:///var/run/weave/weave.sock docker run -d --name swarm-agent $SYSTEM_ROLE swarm join --advertise=$PROXY_IP:12375 consul://consul.weave.local:8500/swarm
	done


	run_on $SWARM_MASTER DOCKER_HOST=unix:///var/run/weave/weave.sock docker run -d --name swarm-master $SYSTEM_ROLE swarm manage -H tcp://0.0.0.0:4567 consul://consul.weave.local:8500/swarm
}

install_registry() {
	TFSTATE_FILE="${SCRIPT_DIR}/../terraform/${ENVIRONMENT}.tfstate"
	JQ_KEY_PREFIX='.modules[0].resources["aws_iam_access_key.registry"].primary.attributes'
	ACCESS_KEY=$(jq  -r "${JQ_KEY_PREFIX}.id" < "$TFSTATE_FILE")
	SECRET_KEY=$(jq  -r "${JQ_KEY_PREFIX}.secret" < "$TFSTATE_FILE")
	if [ "$ENVIRONMENT" = "dev" ]; then
	    BUCKET_NAME="weaveworks_registry_dev"
	else
	    BUCKET_NAME="weaveworks_registry"
	fi
	# TODO, Run it through Swarm?
	REGISTRY_HOST=$(dig +short docker.${DOMAIN} | sort | head -1)
	run_on $REGISTRY_HOST docker rm -f registry || true
	run_on $REGISTRY_HOST DOCKER_HOST=unix:///var/run/weave.sock docker run -d --name registry --restart=always \
		-e REGISTRY_HTTP_ADDR=:80 \
		-e REGISTRY_STORAGE=s3 \
		-e REGISTRY_STORAGE_S3_BUCKET=$BUCKET_NAME \
		-e REGISTRY_STORAGE_S3_REGION=us-east-1 \
		-e REGISTRY_STORAGE_S3_ACCESSKEY=$ACCESS_KEY \
		-e REGISTRY_STORAGE_S3_SECRETKEY=$SECRET_KEY \
		registry:2.1.1
}

case $1 in
	weave)
		install_weave
		;;
	consul)
		install_consul
		;;
	swarm)
		install_swarm
		;;
	registry)
		install_registry
		;;
esac
