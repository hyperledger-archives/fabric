# Openchain - Peer

##Overview

This project contains the core blockchain fabric.  

## Building the project

### Go Package dependencies
You may need to manually install some go packages that the peer project is dependent on.  Simply view the [Gomfile](./Gomfile) in this directory and see the packages the project depends on.  Then simply issue a '*go get ...*' command for each package listed, and example is shown below:

    go get github.com/spf13/viper

### Protobuf dependencies
The system currently utilizes the [google.protobuf.Timestamp](https://github.com/google/protobuf/blob/master/src/google/protobuf/timestamp.proto) and the [google.protobuf.Empty](https://github.com/google/protobuf/blob/master/src/google/protobuf/empty.proto) protobuf packages. These proto files will be available on the openchain-vm in the */usr/include/google/protobuf* directory (dependent on how you installed protoc). If you haven't already installed the protos, then navigate to the openchain-dev-env directory within the VM and execute the golang_grpcSetup script, as shown below. The script will take approximately 5 minutes to complete, upon which you will see all of the necessary proto files within the */usr/include/google/protobuf* directory.

    cd /openchain/openchain-dev-env/
    ./golang_grpcSetup.sh

It is assumed we may leverage additional protobuf packages in the future (any.proto, duration.proto, etc.). To utilize these dependencies, you will need to manually compile the proto files and make the destination the proper location in the VM's GOPATH location. For example, compiling the timestamp.proto and emptry.proto as referenced below and writing to the proper GOPATH location would use the following command:

    protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/timestamp.proto
    protoc --go_out=plugins=grpc:$GOPATH/src /usr/include/google/protobuf/empty.proto

You should now have the following newly generated files:

    $GOPATH/src/google/protobuf/timestamp.pb.go
    $GOPATH/src/google/protobuf/empty.pb.go

## Openchain-peer commands

To see what commands are available, simply execute the following command:

    openchain-peer

You should see some output similar to below (**NOTE**: rootcommand below is hardcoded in the [main.go](./main.go).  Current build will actually create an *openchain-peer* executable file).

```
    Usage:
      openchain [command]

    Available Commands:
      peer        Run Openchain peer.
      status      Status of the Openchain peer.
      stop        Stops the Openchain peer.
      chainlet    Compiles the specified chainlet.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for openchain


    Use "openchain [command] --help" for more information about a command.
```

The **peer** command will run peer process.  You can then use the other commands to interact with this peer process.  For example, status will show the peer status.


## Configuration

Configuration utilizes the [viper](https://github.com/spf13/viper) and [cobra](https://github.com/spf13/cobra) libraries.

There is a **config.yaml** file that contains the configuration for the peer process.  Many of the configuration settings can be overriden at the command line by setting ENV variables that match the configuration setting, but by prefixing the tree with *'OPENCHAIN_'*.  For example, logging level manipulation through the environment is shown below:

    OPENCHAIN_PEER_LOGGING_LEVEL=CRITICAL ./openchain-peer

## Logging
Logging utilizes the [go-logging](https://github.com/op/go-logging) library.  

The available log levels in order of increasing verbosity are: *CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG*

## Generating grpc code

From the <WORKSPACE>/protos directory, execute:

    protoc --go_out=plugins=grpc:. server_admin.proto

    protoc --go_out=plugins=grpc:. openchain.proto
