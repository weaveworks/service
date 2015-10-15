#!/bin/bash

set -eu

if echo "$DOCKER_HOST" | grep "127.0.0.1" >/dev/null; then
    echo "!! DOCKER_HOST is set to \"$DOCKER_HOST\" !!"
    while true; do
        read -p "!! Please confirm you are not building on dev/prod by saying 'yes':" yn
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
