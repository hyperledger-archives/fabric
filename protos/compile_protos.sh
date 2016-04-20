#!/bin/bash

set -e
set -x

# Compile proto files required by openchain-peer
#protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/timestamp.proto
#protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/empty.proto

# Compile protos in the proto folder
cd $GOPATH/src/github.com/hyperledger/fabric/protos
protoc --go_out=plugins=grpc:. *.proto
