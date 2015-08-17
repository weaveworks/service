#!/bin/bash

set -eux

SSH_ARGS="-i ../weave-keypair.pem -o StrictHostKeyChecking=no"
SSH_USER="ubuntu"

run_on() {
    local host=$1
    shift 1
    ssh $SSH_ARGS $SSH_USER@$host "$@"
}

install_weave() {
    # Start weave on all the hosts
    HOSTS=$(dig +short docker.cloud.weave.works)
    INTERNAL_IPS=$(dig +short internal.cloud.weave.works)
    for host in $HOSTS; do
        run_on $host sudo curl -L https://github.com/weaveworks/weave/raw/master/weave -o /usr/local/bin/weave
        run_on $host sudo chmod a+x /usr/local/bin/weave
        run_on $host weave stop || true
        run_on $host weave launch-proxy -H tcp://0.0.0.0:12375 -H unix:///var/run/weave.sock
        run_on $host weave launch-router $INTERNAL_IPS
    done
    for host in $HOSTS; do
        run_on $host weave expose
    done
}

install_consul() {
    # Start consul on first N hosts
    CONSUL_N=3
    CONSUL_HOSTS=$(dig +short docker.cloud.weave.works | sort | head -$CONSUL_N)
    for host in $CONSUL_HOSTS; do
        run_on $host docker rm -f consul || true
    done
    i=0
    for host in $CONSUL_HOSTS; do
        run_on $host docker pull weaveworks/consul
        run_on $host DOCKER_HOST=unix:///var/run/weave.sock docker run -d --name consul weaveworks/consul -bootstrap-expect $CONSUL_N -retry-join consul.weave.local -node consul$i
        i=$((i + 1))
    done
}

install_swarm() {
    # Start the swam agent on every host, and the master on one
    SWARM_HOSTS=$(dig +short docker.cloud.weave.works)
    SWARM_MASTER=$(dig +short docker.cloud.weave.works | sort | head -1)

    run_on $SWARM_MASTER docker rm -f swarm-master || true
    for host in $SWARM_HOSTS; do
         run_on $host docker rm -f swarm-agent || true
     done

     for host in $SWARM_HOSTS; do
         local PROXY_IP=$(run_on $host /sbin/ifconfig eth0 | grep "inet addr:" | cut -d: -f2 | awk '{ print $1 }')
         run_on $host DOCKER_HOST=unix:///var/run/weave.sock docker run -d --name swarm-agent swarm join --advertise=$PROXY_IP:12375 consul://consul.weave.local:8500/swarm
    done


    run_on $SWARM_MASTER DOCKER_HOST=unix:///var/run/weave.sock docker run -d --name swarm-master swarm manage -H tcp://0.0.0.0:4567 consul://consul.weave.local:8500/swarm
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
esac
