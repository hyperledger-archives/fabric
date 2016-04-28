## Incubation Notice
This is a Hyperledger project in Incubation. It was proposed to the community and documented [here](https://goo.gl/RYQZ5N). Information on what Incubation entails can be found in the Hyperledger Project Lifecycle [document](https://goo.gl/4edNRc).

## Overview
This project contains the core blockchain fabric code, development environment scripts, and documents for developers to contribute fabric code or work on their own applications.

* [Building the fabric core](#building-the-fabric-core-)
* [Building outside of Vagrant](#building-outside-of-vagrant-)
* [Code contributions](#code-contributions-)
* [Communication](#communication-)
* [Coding Golang](#coding-golang-)
* [Writing Chaincode](#writing-chaincode-)
* [Setting Up a Network](#setting-up-a-network-)
* [Working with CLI, REST, and Node.js](#working-with-cli-rest-and-nodejs-)
* [Configuration](#configuration-)
* [Logging](#logging-)
* [Generating grpc code](#generating-grpc-code-)
* [Adding or updating Go packages](#adding-or-updating-go-packages-)

## License <a name="license"></a>
This software is made available under the [Apache License Version 2.0](LICENSE).

## Building the fabric core <a name="build"></a>
The following instructions assume that you have followed the [development environment getting started instructions](docs/dev-setup/devenv.md).

To access your VM, run
```
vagrant ssh
```

From within the VM, you can build, run, and test your environment.

#### 1. Go build
```
cd $GOPATH/src/github.com/hyperledger/fabric/peer
go build
```

#### 2. Run

To see what commands are available, simply execute the following command:
```
cd $GOPATH/src/github.com/hyperledger/fabric/peer
./peer
```

You should see some output similar to below (**NOTE**: The root command below is hardcoded in the [main.go](./peer/main.go). Current build will actually create a *peer* executable file).

```
    Usage:
      peer [command]

    Available Commands:
      peer        Runs the peer.
      status      Returns status of the peer.
      stop        Stops the running peer.
      login       Logs in a user on CLI.
      network     Lists all network peers.
      chaincode   chaincode specific commands.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for peer
          --logging-level="": Default logging level and overrides, see core.yaml for full syntax


    Use "peer [command] --help" for more information about a command.

```

The **peer** command will run the peer process. You can then use the other commands to interact with the peer process. For example, the `status` command will return the status of the running peer.

#### 3. Test
New code must be accompanied by test cases both in unit and Behave tests.

#### 3.1 Unit Tests
To run all unit tests, in one window, run `./peer peer`. In a second window

    cd $GOPATH/src/github.com/hyperledger/fabric
    go test -timeout=20m $(go list github.com/hyperledger/fabric/... | grep -v /vendor/ | grep -v /examples/)

Note that the first time the tests are run, they can take some time due to the need to download a docker image that is about 1GB in size. This is why the timeout flag is added to the above command.

To run a specific test use the `-run RE` flag where RE is a regular expression that matches the test case name. To run tests with verbose output use the `-v` flag. For example, to run the `TestGetFoo` test case, change to the directory containing the `foo_test.go` and enter:

    go test -v -run=TestGetFoo

#### 3.2 Behave Tests
[Behave](http://pythonhosted.org/behave/) tests will setup networks of peers with different security and consensus configurations and verify that transactions run properly. To run these tests

```
cd $GOPATH/src/github.com/hyperledger/fabric/bddtests
behave
```
Some of the Behave tests run inside Docker containers. If a test fails and you want to have the logs from the Docker containers, run the tests with this option
```
behave -D logs=Y
```

Note, you must run the unit tests first to build the necessary Peer and Member Services docker images. These images can also be individually built using the commands
```
go test github.com/hyperledger/fabric/core/container -run=BuildImage_Peer
go test github.com/hyperledger/fabric/core/container -run=BuildImage_Obcca
```

## Building outside of Vagrant <a name="vagrant"></a>
This is not recommended, however some users may wish to build outside of Vagrant if they use an editor with built in Go tooling. The instructions are below.

- Follow all steps required to setup and run a Vagrant image:
  - Make sure you you have [Go 1.6](https://golang.org/) or later installed
  - Set the maximum number of open files to 10000 or greater for your OS
  - Install [RocksDB](https://github.com/facebook/rocksdb/blob/master/INSTALL.md) version 4.1 and it's dependencies:
```
apt-get install -y libsnappy-dev zlib1g-dev libbz2-dev
cd /tmp
git clone https://github.com/facebook/rocksdb.git
cd rocksdb
git checkout v4.1
PORTABLE=1 make shared_lib
INSTALL_PATH=/usr/local make install-shared
```
- Run the following commands:
```
cd $GOPATH/src/github.com/hyperledger/fabric/peer
CGO_CFLAGS=" " CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install
```
- Make sure that the Docker daemon initialization includes the options
```
-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock
```
- Be aware that the Docker bridge (the `CORE_VM_ENDPOINT`) may not come
up at the IP address currently assumed by the test environment
(`172.17.0.1`). Use `ifconfig` or `ip addr` to find the docker bridge.


## Code contributions <a name="contrib"></a>
We welcome contributions to the Hyperledger Project in many forms. There's always plenty to do! Full details of how to contribute to this project are documented in the [CONTRIBUTING.md](CONTRIBUTING.md) file.

## Communication <a name="communication"></a>
We use [Hyperledger Slack](https://slack.hyperledger.org/) for communication and Google Hangouts&trade; for screen sharing between developers.

For general purpose questions, use [StackOverflow](http://stackoverflow.com/questions/tagged/hyperledger). Even if questions are answered through Hyperledger Slack, please post them to StackOverflow so answers don't have to be repeated continuously.

## Coding Golang <a name="coding"></a>
- We require a file [header](docs/dev-setup/headers.txt) in all source code files. Simply copy and paste the header when you create a new file.
- We code in Go&trade; and strictly follow the [best practices](http://golang.org/doc/effective_go.html)
and will not accept any deviations. You must run the following tools against your Go code and fix all errors and warnings:
	- [golint](https://github.com/golang/lint)
	- [go vet](https://golang.org/cmd/vet/)

## Writing Chaincode <a name="chaincode"></a>
Since chaincode is written in Go language, you can set up the environment to accommodate the rapid edit-compile-run of your chaincode. Follow the instructions on the [Sandbox Setup](docs/API/SandboxSetup.md) page, which allows you to run your chaincode off the blockchain.

## Setting Up a Network <a name="devnet"></a>

To set up an development network composed of several validating peers, follow the instructions on the [Devnet Setup](docs/dev-setup/devnet-setup.md) page. This network leverages Docker to manage multiple peer instances on the same machine, allowing you to quickly test your chaincode.


## Working with CLI, REST, and Node.js <a name="cli"></a>

When you are ready to start interacting with the peer node through the available APIs and packages, follow the instructions on the [API Documentation](docs/API/CoreAPI.md) page.

## Configuration <a name="config"></a>

Configuration utilizes the [viper](https://github.com/spf13/viper) and [cobra](https://github.com/spf13/cobra) libraries.

There is a **core.yaml** file that contains the configuration for the peer process. Many of the configuration settings can be overridden on the command line by setting ENV variables that match the configuration setting, but by prefixing with *'CORE_'*. For example, logging level manipulation through the environment is shown below:

    CORE_PEER_LOGGING_LEVEL=CRITICAL ./peer

## Logging <a name="logging"></a>

Logging utilizes the [go-logging](https://github.com/op/go-logging) library.  

The available log levels in order of increasing verbosity are: *CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG*

See [specific logging control](docs/dev-setup/logging-control.md) instructions when running the peer process.

## Generating grpc code <a name="grpc"></a>

If you modify any .proto files, run the following command to generate new .pb.go files.

```
$GOPATH/src/github.com/hyperledger/fabric/devenv/compile_protos.sh
```

## Adding or updating Go packages <a name="vendoring"></a>

The fabric uses Go 1.6 vendoring for package management. This means that all required packages reside in the /vendor folder within the fabric project. Go will use packages in this folder instead of the GOPATH when `go install` or `go build` is run. To manage the packages in the /vendor folder, we use [Govendor](https://github.com/kardianos/govendor). This is installed in the Vagrant environment. The following commands can be used for package management.

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
