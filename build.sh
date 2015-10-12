#!/bin/bash

set -eu

for dir in app-mapper client users frontend monitoring; do
    make -C $dir
done
