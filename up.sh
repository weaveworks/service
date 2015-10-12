#!/bin/bash

if [ $# -lt 1 ]
then
	echo "usage: $0 <hostname>/dev/prod"
	exit 1
fi

ENV=$1
shift

case $ENV in
	dev)
		LOCAL_PORT=4502
		;;
	prod)
		LOCAL_PORT=4501
		;;
	*)
		LOCAL_PORT=4567
		;;
esac

if [ $(lsof -i 4 -n -P 2>/dev/null | grep ":$LOCAL_PORT" | wc -l) -le 0 ]
then
	echo "No connection to $ENV (:$LOCAL_PORT) detected."
	echo "Please run ./connect.sh $ENV"
	exit 2
fi

COMPONENTS="app-mapper client frontend monitoring users"
if [ ! -z "$@" ]
then
	COMPONENTS="$@"
fi

COMPOSE_COMMAND=${COMPOSE_COMMAND:-"up -d"}

for COMPONENT in $COMPONENTS
do
	echo "$COMPONENT"

	if [ -f $COMPONENT/docker-compose-$ENV.yml ]
	then
		FILE=$COMPONENT/docker-compose-$ENV.yml
	elif [ -f $COMPONENT/docker-compose.yml ]
	then
		FILE=$COMPONENT/docker-compose.yml
	else
		echo "No docker-compose file found for $COMPONENT"
		exit 3
	fi
	echo "Using $FILE"

	DOCKER_HOST=tcp://127.0.0.1:$LOCAL_PORT docker-compose --file $FILE $COMPOSE_COMMAND
done
