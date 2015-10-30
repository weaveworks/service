#!/usr/bin/env bash

VERSION=v1.0.7
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m) ; ARCH=${ARCH/x86_64/amd64}
KUBECTL=kubernetes/platforms/${OS}/${ARCH}/kubectl

if [ ! -f kubernetes.tar.gz ]
then
	wget -q --show-progress https://github.com/kubernetes/kubernetes/releases/download/${VERSION}/kubernetes.tar.gz
fi

if [ ! -f ${KUBECTL} ]
then
	tar zxvf kubernetes.tar.gz ${KUBECTL}
fi

if [ ! -f kubectl ]
then
	cp ${KUBECTL} .
fi

echo "You may want to put kubectl to somewhere in your PATH"
