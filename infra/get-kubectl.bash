#!/usr/bin/env bash

VERSION=v1.0.7
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m) ; ARCH=${ARCH/x86_64/amd64}

wget -q --show-progress -O kubectl https://storage.googleapis.com/kubernetes-release/release/${VERSION}/bin/${OS}/${ARCH}/kubectl

echo "You may want to put kubectl to somewhere in your PATH"

