#!/bin/sh

PROJDIR=`cd $(dirname $0); pwd -P`
cd $PROJDIR
echo `pwd`

cd rpc/grpc/proto
mkdir -p ../pb

protoc -I ./ ./*.proto --go_out=plugins=grpc:../pb
res=$?
if [ $res -ne 0 ];then
    echo "生成go代码失败！"
    exit $res
# else
#     cd ../pg
#     rm -rf *.go
#     mv ../proto/out/*.go .
#     sed -i '' 's/,omitempty//g' *.go
#     echo "生成go代码成功！"
fi