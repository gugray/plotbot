#!/bin/bash

rm -rf bin
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0 # Needed to run on Alpine
go build -o bin/plotbot
