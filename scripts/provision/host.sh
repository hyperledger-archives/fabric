#!/bin/bash

# Add any logic that is common to any host (vagrant, travis, etc) environments here

# Copy protobuf dir so we can build the protoc-gen-go binary. Then delete the directory.
mkdir -p $GOPATH/src/github.com/golang/protobuf/
cp -r $GOPATH/src/github.com/hyperledger/fabric/vendor/github.com/golang/protobuf/ $GOPATH/src/github.com/golang/
go install -a github.com/golang/protobuf/protoc-gen-go
rm -rf $GOPATH/src/github.com/golang/protobuf

