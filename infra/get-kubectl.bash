#!/usr/bin/env bash

VERSION=v1.0.7
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m) ; ARCH=${ARCH/x86_64/amd64}
KUBECTL=kubernetes/platforms/${OS}/${ARCH}/kubectl

wget -q --show-progress https://github.com/kubernetes/kubernetes/releases/download/${VERSION}/kubernetes.tar.gz
tar zxvf kubernetes.tar.gz ${KUBECTL}
mv ${KUBECTL} .
rm -rf kubernetes/
rm kubernetes.tar.gz

echo "You may want to mv kubectl to somewhere in your PATH"
