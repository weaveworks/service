#!/bin/bash

cd /go/src/github.com/weaveworks/service/app-mapper

# Use eval to interpret spaces in the TEST_FLAGS arguments
eval "go test $TEST_FLAGS ./..."
