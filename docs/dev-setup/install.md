# Installation
Install the blockchain fabric by completing the following tasks:

* [Building the fabric core](#building-the-fabric-core-)
* [Code contributions](#code-contributions-)
* [Writing Chaincode](#writing-chaincode-)
* [Setting Up a Network](#setting-up-a-network-)
* [Working with CLI, REST, and Node.js](#working-with-cli-rest-and-nodejs-)
* [Configuration](#configuration-)
* [Logging](#logging-)

## Building the fabric core <a name="build"></a>
The following instructions assume that you have followed the [development environment getting started instructions](devenv.md).

To access your VM, run
```
vagrant ssh
```

From within the VM, you can build, run, and test your environment.

#### 1. Go build
```
cd $GOPATH/src/github.com/hyperledger/fabric
make peer
```

#### 2. Run/Execute

To see what commands are available, simply execute the following commands:
```
cd $GOPATH/src/github.com/hyperledger/fabric/peer
./peer
```

You should see some output similar to below (**NOTE**: The root command below is hardcoded in the [main.go](../../peer/main.go) and the build creates the `peer` executable).

```
    Usage:
      peer [command]

    Available Commands:
      node        node specific commands.
      network     network specific commands.
      chaincode   chaincode specific commands.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for peer
          --logging-level="": Default logging level and overrides, see core.yaml for full syntax


    Use "peer [command] --help" for more information about a command.

```

The `node start` command will initiate a peer process, with which one can interact by executing other commands. For example, the `node status` command will return the status of the running peer. The full list of commands is the following:

```
      node
        start       Starts the node.
        status      Returns status of the node.
        stop        Stops the running node.
      network
        login       Logs in user to CLI.
        list        Lists all network peers.
      chaincode
        deploy      Deploy the specified chaincode to the network.
        invoke      Invoke the specified chaincode.
        query       Query using the specified chaincode.
      help        Help about any command
```

#### 3. Test
New code must be accompanied by test cases both in unit and Behave tests.

#### 3.1 Go Unit Tests
Use the following sequence to run all unit tests

    cd $GOPATH/src/github.com/hyperledger/fabric
    make unit-test

To run a specific test use the `-run RE` flag where RE is a regular expression that matches the test case name. To run tests with verbose output use the `-v` flag. For example, to run the `TestGetFoo` test case, change to the directory containing the `foo_test.go` and call/excecute

    go test -v -run=TestGetFoo

#### 3.2 Node.js Unit Tests

You must also run the Node.js unit tests to insure that the Node.js client SDK is not broken by your changes. To run the Node.js unit tests, follow the instructions [here](https://github.com/hyperledger/fabric/tree/master/sdk/node#unit-tests).

#### 3.3 Behave Tests
[Behave](http://pythonhosted.org/behave/) tests will setup networks of peers with different security and consensus configurations and verify that transactions run properly. To run these tests

```
cd $GOPATH/src/github.com/hyperledger/fabric
make behave
```
Some of the Behave tests run inside Docker containers. If a test fails and you want to have the logs from the Docker containers, run the tests with this option
```
behave -D logs=Y
```

Note, in order to run behave directly, you must run 'make images' first to build the necessary `peer` and `member services` docker images. These images can also be individually built when `go test` is called with the following parameters:
```
go test github.com/hyperledger/fabric/core/container -run=BuildImage_Peer
go test github.com/hyperledger/fabric/core/container -run=BuildImage_Obcca
```

## Code contributions <a name="contrib"></a>
We welcome contributions to the Hyperledger Project in many forms. There's always plenty to do! Full details of how to contribute to this project are documented in the [CONTRIBUTING.md](../../CONTRIBUTING.md) file.

## Setting Up a Network <a name="devnet"></a>

To set up an development network composed of several validating peers, follow the instructions on the [Devnet Setup](devnet-setup.md) page. This network leverages Docker to manage multiple peer instances on the same machine, allowing you to quickly test your chaincode.

### Writing Chaincode <a name="chaincode"></a>
Since chaincode is written in Go language, you can set up the environment to accommodate the rapid edit-compile-run of your chaincode. Follow the instructions on the [Sandbox Setup](../API/SandboxSetup.md) page, which allows you to run your chaincode off the blockchain.

## Working with CLI, REST, and Node.js <a name="cli"></a>

When you are ready to start interacting with the peer node through the available APIs and packages, follow the instructions on the [CoreAPI Documentation](../API/CoreAPI.md) page.

## Configuration <a name="config"></a>

Configuration utilizes the [viper](https://github.com/spf13/viper) and [cobra](https://github.com/spf13/cobra) libraries.

There is a **core.yaml** file that contains the configuration for the peer process. Many of the configuration settings can be overridden on the command line by setting ENV variables that match the configuration setting, but by prefixing with *'CORE_'*. For example, logging level manipulation through the environment is shown below:

    CORE_PEER_LOGGING_LEVEL=CRITICAL ./peer

## Logging <a name="logging"></a>

Logging utilizes the [go-logging](https://github.com/op/go-logging) library.  

The available log levels in order of increasing verbosity are: *CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG*

See [specific logging control](logging-control.md) instructions when running the peer process.
