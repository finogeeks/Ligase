#!/bin/sh

base=`pwd`

sh ./config/env.sh $1
cat ./config/fed.yaml

./federation
