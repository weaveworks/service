#!/bin/sh
WEAVE_IP=$(/sbin/ifconfig ethwe | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
echo /bin/consul agent -server -config-dir=/config -advertise $WEAVE_IP $@
exec /bin/consul agent -server -config-dir=/config -advertise $WEAVE_IP $@
