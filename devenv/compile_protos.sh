#!/bin/bash

set -e
set -x

# Compile proto files required by openchain-peer
protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/timestamp.proto
protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/empty.proto

# Compile protos in the proto folder
cd $GOPATH/src/github.com/openblockchain/obc-peer/protos
protoc --go_out=plugins=grpc:. *.proto

# Compile all other protos
cd $GOPATH/src/github.com/openblockchain/obc-peer/
for f in $(find $GOPATH/src/github.com/openblockchain/obc-peer/openchain -name '*.proto'); do
	protoc --proto_path=$GOPATH/src/github.com/openblockchain/obc-peer/ --go_out=plugins=grpc:. $f
done
