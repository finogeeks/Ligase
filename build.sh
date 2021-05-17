#!/bin/sh

PROJDIR=`cd $(dirname $0); pwd -P`
echo `pwd`

#export GOPROXY=https://goproxy.io

go build -v -o $PROJDIR/bin/engine-server $PROJDIR/cmd/engine-server
go build -v -o $PROJDIR/bin/federation $PROJDIR/cmd/federation
go build -v -o $PROJDIR/bin/content $PROJDIR/cmd/content

go mod tidy