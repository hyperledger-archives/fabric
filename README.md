## Notice regarding the Linux Foundation's Hyperledger project

The openblockchain project is IBM's proposed contribution to the Linux Foundation's [Hyperledger](https://www.hyperledger.org/) project. We have made it available as open source to enable others to explore our architecture and design. IBM's intention is to engage rigorously in the Linux Foundation's [Hyperledger](https://www.hyperledger.org/) project as the community establishes itself, and decides on a code base. Once established, we will transition our development focus to the [Hyperledger](https://www.hyperledger.org/) effort, and this code will be maintained as needed for IBM's use.

While we invite contribution to the openblockchain project, we believe that the broader blockchain community's focus should be the [Hyperledger](https://www.hyperledger.org/) project.

# Openchain - Peer

## Overview

This project contains the core blockchain fabric.  

## Building the project

Assuming you have followed the [development environment getting started instructions](https://github.com/openblockchain/obc-docs/blob/master/dev-setup/devenv.md)

To access your VM, run
```
vagrant ssh
```

From within the VM, follow these additional steps.

### Go build
```
cd $GOPATH/src/github.com/openblockchain/obc-peer
go build
```

## Run

To see what commands are available, simply execute the following command:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer

You should see some output similar to below (**NOTE**: rootcommand below is hardcoded in the [main.go](./main.go). Current build will actually create an *obc-peer* executable file).

```
    Usage:
      obc-peer [command]

    Available Commands:
      peer        Run obc peer.
      status      Status of the obc peer.
      stop        Stops the obc peer.
      chaincode    Compiles the specified chaincode.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for openchain


    Use "obc-peer [command] --help" for more information about a command.
```

The **peer** command will run peer process. You can then use the other commands to interact with this peer process. For example, status will show the peer status.

## Test

#### Unit Tests
To run all unit tests, in one window, run `./obc-peer peer`. In a second window

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    go test -timeout=20m $(go list github.com/openblockchain/obc-peer/... | grep -v /vendor/)

Note that the first time the tests are run, they can take some time due to the need to download a docker image that is about 1GB in size. This is why the timeout flag is added to the above command.

To run a specific test use the `-run RE` flag where RE is a regular expression that matches the test name. To run tests with verbose output use the `-v` flag. For example, to run TestGetFoo function, change to the directory containing the `foo_test.go` and enter:

    go test -test.v -run=TestGetFoo

#### Behave Tests
OBC also has [Behave](http://pythonhosted.org/behave/) tests that will setup networks of peers with different security and consensus configurations and verify that transactions run properly. To run these tests

```
cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/peer/bddtests
behave
```
Some of the Behave tests run inside Docker containers. If a test fails and you want to have the logs from the Docker containers, run the tests with this option
```
behave -D logs=Y
```

Note, you must run the unit tests first to build the necessary Peer and OBCCA docker images. These images can also be individually built using the commands
```
go test github.com/openblockchain/obc-peer/openchain/container -run=BuildImage_Peer
go test github.com/openblockchain/obc-peer/openchain/container -run=BuildImage_Obcca
```

## Writing Chaincode
Since chaincode is written in Go language, you can set up the environment to accommodate the rapid edit-compile-run of your chaincode. Follow the instructions on the [Sandbox Setup](https://github.com/openblockchain/obc-docs/blob/master/api/SandboxSetup.md) page, which allows you to run your chaincode off the blockchain.

## Setting Up a Network

To set up an Openchain network of several validating peers, follow the instructions on the [Devnet Setup](https://github.com/openblockchain/obc-docs/blob/master/dev-setup/devnet-setup.md) page. This network leverage Docker to manage multiple instances of validating peer on the same machine, allowing you to quickly test your chaincode.


## Working with CLI, REST, and Node.js

When you are ready to start interacting with the Openchain peer node through the available APIs and packages, follow the instructions on the [API Documentation](https://github.com/openblockchain/obc-docs/blob/master/api/Openchain%20API.md) page.

## Configuration

Configuration utilizes the [viper](https://github.com/spf13/viper) and [cobra](https://github.com/spf13/cobra) libraries.

There is an **openchain.yaml** file that contains the configuration for the peer process. Many of the configuration settings can be overridden at the command line by setting ENV variables that match the configuration setting, but by prefixing the tree with *'OPENCHAIN_'*. For example, logging level manipulation through the environment is shown below:

    OPENCHAIN_PEER_LOGGING_LEVEL=CRITICAL ./obc-peer

## Logging

Logging utilizes the [go-logging](https://github.com/op/go-logging) library.  

The available log levels in order of increasing verbosity are: *CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG*

See [specific logging control] (https://github.com/openblockchain/obc-docs/blob/master/dev-setup/logging-control.md) when running OBC.

## Generating grpc code
If you modify any .proto files, run the following command to generate new .pb.go files.
```
/openchain/obc-dev-env/compile_protos.sh
```

## Adding or updating a Go packages
Openchain uses Go 1.6 vendoring for package management. This means that all required packages reside in the /vendor folder within the obc-peer project. Go will use packages in this folder instead of the GOPATH when `go install` or `go build` is run. To manage the packages in the /vendor folder, we use [Govendor](https://github.com/kardianos/govendor). This is installed in the Vagrant environment. The following commands can be used for package management.
```
# Add external packages.
govendor add +external

# Add a specific package.
govendor add github.com/kardianos/osext

# Update vendor packages.
govendor update +vendor

# Revert back to normal GOPATH packages.
govendor remove +vendor

# List package.
govendor list
```

## Building outside of Vagrant
This is not recommended, however some users may wish to build Openchain outside of Vagrant if they use an editor with built in Go tooling. The instructions are

1. Follow all steps required to setup and run a Vagrant image
- Make you you have [Go 1.6](https://golang.org/) or later installed
- Set the maximum number of open files to 10000 or greater for your OS
- Install [RocksDB](https://github.com/facebook/rocksdb/blob/master/INSTALL.md) version 4.1 and it's dependencies:
```
apt-get install -y libsnappy-dev zlib1g-dev libbz2-dev
cd /tmp
git clone https://github.com/facebook/rocksdb.git
cd rocksdb
git checkout tags/v4.1
PORTABLE=1 make shared_lib
INSTALL_PATH=/usr/local make install-shared
```
- Run the following commands:
```
cd $GOPATH/src/github.com/openblockchain/obc-peer
CGO_CFLAGS=" " CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install
```
- Make sure that the Docker daemon initialization includes the options
```
-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock
```
- Be aware that the Docker bridge (the `OPENCHAIN_VM_ENDPOINT`) may not come
up at the IP address currently assumed by the test environment
(`172.17.0.1`). Use `ifconfig` or `ip addr` to find the docker bridge.
