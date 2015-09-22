#!/bin/bash

# HACK, but should do for now
if ! grep '172.17.42.1' /etc/dhcp/dhclient.conf > /dev/null ; then
    sed -i 's/domain-name-servers\(,\)\{0,1\}//g' /etc/dhcp/dhclient.conf
    sed -i '1iprepend domain-name-servers 172.17.42.1;' /etc/dhcp/dhclient.conf;
    systemctl restart ifup@eth0.service
fi

