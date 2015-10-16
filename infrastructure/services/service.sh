#!/bin/bash

set -eux

SSH_USER="ubuntu"

# This is the revision of weave originally running in prod
# WEAVE_REV="de95859"
# WEAVE_DOCKER_TAG=1.1.1

# This revision of weave is pre-fdp merge, but post bug fixes in
# the proxy required for terraform.
WEAVE_REV="431d04bd40c0"
WEAVE_DOCKER_TAG="git-$WEAVE_REV"

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

run_on() {
	local host=$1
	shift 1
	ssh $SSH_ARGS $SSH_USER@$host "$@"
}

install_weave() {
	# Stop / start weave on all the hosts
	HOSTS=$(dig +short docker.${DOMAIN})
	INTERNAL_IPS=$(dig +short internal.${DOMAIN})
	if [ -z "$HOSTS" ] || [ -z "$INTERNAL_IPS" ]; then
		echo "Failed to find hosts!"
		exit 1
	fi

	for host in $HOSTS; do
		# Stop weave (using the old script)
		run_on $host weave stop || true

		# Download new weave script straight of master.
		# NB this script will not have a version number embedded
		# and will expect to use whatever is tagger with latest,
		# so we need to ensure that whatever image tagged with
		# latest is the right version.
		run_on $host sudo curl -L https://raw.githubusercontent.com/weaveworks/weave/$WEAVE_REV/weave -o /usr/local/bin/weave
		run_on $host sudo chmod a+x /usr/local/bin/weave
		run_on $host docker pull weaveworks/weave:$WEAVE_DOCKER_TAG
		run_on $host docker pull weaveworks/weaveexec:$WEAVE_DOCKER_TAG
		run_on $host docker tag -f weaveworks/weave:$WEAVE_DOCKER_TAG weaveworks/weave:latest
		run_on $host docker tag -f weaveworks/weaveexec:$WEAVE_DOCKER_TAG weaveworks/weaveexec:latest

		# Must launch the router first, and wait a bit - https://github.com/weaveworks/weave/issues/1547
		run_on $host weave launch-router $INTERNAL_IPS
		sleep 5

		# Launch the proxy also listening on :12375, for the swarm master to talk to
		run_on $host weave launch-proxy -H tcp://0.0.0.0:12375 -H unix:///var/run/weave/weave.sock

		# Sleep to allow some time to settle
		sleep 30
	done
}

install_scope() {
	# Start weave on all the hosts
	HOSTS=$(dig +short docker.${DOMAIN})
	INTERNAL_IPS=$(dig +short internal.${DOMAIN})
	if [ -z "$HOSTS" ] || [ -z "$INTERNAL_IPS" ]; then
		echo "Failed to find hosts!"
		exit 1
	fi

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

case $1 in
	weave)
		install_weave
		;;
	scope)
		install_scope
		;;
	consul)
		install_consul
		;;
	swarm)
		install_swarm
		;;
esac
