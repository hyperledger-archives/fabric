#!/bin/bash

set -e
set -x

# Compile proto files required by fabric
protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/timestamp.proto
protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/empty.proto

# Compile protos in the proto folder
cd $GOPATH/src/github.com/hyperledger/fabric/protos
protoc --go_out=plugins=grpc:. *.proto

# Compile all other protos
cd $GOPATH/src/github.com/hyperledger/fabric/
for f in $(find $GOPATH/src/github.com/hyperledger/fabric/  -name '*.proto'); do
	protoc --proto_path=$GOPATH/src/github.com/hyperledger/fabric/ --go_out=plugins=grpc:. $f
done
