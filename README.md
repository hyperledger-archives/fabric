# Openchain - Peer

##Overview

This project contains the core blockchain fabric.  

## Building the project

### Go Package dependencies
You may need to manually install some go packages that the peer project is dependent on.  Simply view the [Gomfile](./Gomfile) in this directory and see the packages the project depends on.  Then simply issue a '*go get ...*' command for each package listed, and example is shown below:

    go get github.com/spf13/viper

## Openchain-peer commands

To see what commands are available, simply execute the following command:

    obc-peer

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
