#!/bin/bash

SSH='ssh -i weave-keypair.pem'

case "$1" in
    --docker)
    TARGET="localhost:2375"
    shift 1
    ;;
    --weave)
    TARGET="localhost:12375"
    shift 1
    ;;
    *)
    TARGET="$($SSH ubuntu@cloud.weave.works docker inspect --format='{{.NetworkSettings.IPAddress}}' swarm-master):4567"
esac

$SSH -q -f -N -L 12375:$TARGET ubuntu@cloud.weave.works

echo export DOCKER_HOST=tcp://localhost:12375
