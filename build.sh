#!/bin/bash

set -eu

if [ -n "${DOCKER_HOST+x}" ] && echo "$DOCKER_HOST" | grep "127.0.0.1" >/dev/null; then
    echo "DOCKER_HOST is set to \"$DOCKER_HOST\"!"
    echo "If you are trying to build for dev/prod, this is probably a mistake."
    while true; do
        read -p "Are you sure you want to continue? " yn
        case $yn in
            yes ) break;;
            no ) exit;;
            * ) echo "Please type 'yes' or 'no'.";;
        esac
    done
fi

for dir in app-mapper client users frontend monitoring; do
    make -C $dir
done
