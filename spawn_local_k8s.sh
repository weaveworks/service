#!/bin/bash

set -e

stop_container() {
    docker stop $1 > /dev/null 2>&1 || true
    docker rm -v $1 > /dev/null 2>&1 || true
}

# Remove all containers forming the cluster
for I in $(docker ps -q -f 'name=k8s*'); do
    stop_container $I
done
stop_container etcd

# Spawn new cluster
# From https://github.com/kubernetes/kubernetes/blob/master/docs/getting-started-guides/docker.md
docker run --name etcd --net=host -d gcr.io/google_containers/etcd:2.0.12 /usr/local/bin/etcd --addr=127.0.0.1:4001 --bind-addr=0.0.0.0:4001 --data-dir=/var/etcd/data
# kubelet requires --docker-endpoint=$DOCKER_HOST, to make it talk to
# weave but ... it doesn't work due to
# https://github.com/weaveworks/weave/issues/1600
docker run \
       --name k8s_master \
       --volume=/:/rootfs:ro \
       --volume=/sys:/sys:ro \
       --volume=/dev:/dev \
       --volume=/var/lib/docker/:/var/lib/docker:rw \
       --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
       --volume=/var/run:/var/run:rw \
       --net=host \
       --pid=host \
       --privileged=true \
       -d \
       gcr.io/google_containers/hyperkube:v1.0.6 \
       /hyperkube kubelet --containerized --hostname-override="127.0.0.1" --address="0.0.0.0" --api-servers=http://localhost:8080 --config=/etc/kubernetes/manifests
docker run --name k8s_proxy -d --net=host --privileged gcr.io/google_containers/hyperkube:v1.0.6 /hyperkube proxy --master=http://127.0.0.1:8080 --v=2
