#!/bin/bash
GOPATH="$(pwd)"
AWS_ACCESS_KEY_ID="AKIAIKG6BZNPFGV3JOLQ"
AWS_SECRET_ACCESS_KEY="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"
export GOPATH AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
go run connect.go

