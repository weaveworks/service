#!/bin/bash

set -e

SCRIPT_DIR=$(cd `dirname -- "$0"`; pwd -P)
LOCAL_K8S_DIR="$SCRIPT_DIR/k8s/local"
LOCAL_K8S_PKI_DIR="$LOCAL_K8S_DIR/pki"

stop_container() {
    docker stop $1 > /dev/null 2>&1 || true
    docker rm -v $1 > /dev/null 2>&1 || true
}

# Remove all containers forming the cluster
for I in $(docker ps -q -f 'name=k8s*'); do
    stop_container $I
done
stop_container local_k8s_proxy
stop_container local_k8s_kubelet
stop_container etcd

# Generate certificates
if ! [ -d "$LOCAL_K8S_PKI_DIR" ]; then
    curl -s -L https://raw.githubusercontent.com/kubernetes/kubernetes/master/cluster/saltbase/salt/generate-cert/make-ca-cert.sh -o /tmp/make-ca-cert.sh
    chmod +x /tmp/make-ca-cert.sh
    CERT_GROUP=`id -g` CERT_DIR="$LOCAL_K8S_PKI_DIR" /tmp/make-ca-cert.sh 10.0.0.1 DNS:kubernetes,DNS:kubernetes.default,DNS:kubernetes.default.svc,DNS:kubernetes.default.svc.cluster.local
fi

# Spawn new cluster
# From https://github.com/kubernetes/kubernetes/blob/master/docs/getting-started-guides/docker.md
# kubelet requires --docker-endpoint=$DOCKER_HOST, to make it talk to
# weave but ... it doesn't work due to
# https://github.com/weaveworks/weave/issues/1600
docker run --name etcd --net=host -d gcr.io/google_containers/etcd:2.0.12 /usr/local/bin/etcd --addr=127.0.0.1:4001 --bind-addr=0.0.0.0:4001 --data-dir=/var/etcd/data
sed "s%\$PKI_HOST_PATH%${LOCAL_K8S_PKI_DIR}%g" "$LOCAL_K8S_DIR"/master.json.in > "$LOCAL_K8S_DIR"/master.json
docker run \
       --name local_k8s_kubelet \
       --volume=/:/rootfs:ro \
       --volume=/sys:/sys:ro \
       --volume=/dev:/dev \
       --volume=/var/lib/docker/:/var/lib/docker:rw \
       --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
       --volume=/var/run:/var/run:rw \
       --net=host \
       --pid=host \
       --privileged=true \
       -v "$LOCAL_K8S_DIR"/master.json:/etc/kubernetes/manifests/master.json \
       -d \
       2opremio/hyperkube:664d2ef \
       /hyperkube kubelet --containerized --hostname-override="127.0.0.1" --address="0.0.0.0" --api-servers=http://localhost:8080 --config=/etc/kubernetes/manifests --cluster-dns=10.0.0.10 --cluster-domain=cluster.local
docker run --name local_k8s_proxy -d --net=host --privileged 2opremio/hyperkube:664d2ef /hyperkube proxy --master=http://127.0.0.1:8080 --v=2
# DNS
# From https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns
# TODO: reuse main etcd instead of spawning another one
sleep 3 # Let hyperkube boot
kubectl create -f "$LOCAL_K8S_DIR"/kube-system.json
kubectl create -f "$LOCAL_K8S_DIR"/skydns
