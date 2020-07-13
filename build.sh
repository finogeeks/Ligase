#!/bin/sh

PROJDIR=`cd $(dirname $0); pwd -P`
cd $PROJDIR
echo `pwd`

export GOPROXY=https://goproxy.io

cd $PROJDIR/cmd/engine-server
go build -v -o $PROJDIR/bin/engine-server
cd $PROJDIR/cmd/federation
go build -v -o $PROJDIR/bin/federation
cd $PROJDIR/cmd/content
go build -v -o $PROJDIR/bin/content

cd $PROJDIR
go mod tidy