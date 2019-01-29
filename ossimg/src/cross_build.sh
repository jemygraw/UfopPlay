#!/usr/bin/env bash
DIR=$(cd ../; pwd)
export GOPATH=$GOPATH:$DIR
GOOS=linux GOARCH=amd64 go build qufop.go
mv qufop ../deploy/
cp qufop.conf ../deploy/
cp ../ossimg.conf ../deploy/
