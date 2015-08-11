#!/bin/bash

set -eux

for dir in app-mapper client users frontend proxy; do
    make -C $dir image.tar
done

REPLICAS=${REPLICAS:-1}

start_container() {
    IMAGE=$1
    NAME=$2
    shift 2

    for i in $(seq $REPLICAS); do
        if docker inspect $NAME$i >/dev/null 2>&1; then
            docker rm -f $NAME$i
        fi
        docker run $@ -d --name=$NAME$i --hostname=$NAME.weave.local $IMAGE
    done
}

if ! weave status > /dev/null; then
    weave launch
fi
eval $(weave env)

(cd users; docker-compose stop; docker-compose rm -f; docker-compose up -d)
(cd app-mapper; docker-compose stop; docker-compose rm -f; docker-compose up -d)
start_container weaveworks/ui-server ui-server
start_container weaveworks/frontend frontend --add-host=dns.weave.local:$(weave docker-bridge-ip)
