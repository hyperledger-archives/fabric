# Openchain API - Working with CLI, REST, and Node.js

## Overview

This document covers the available APIs to connect to an Openchain peer node. The three approaches described are:

1. [CLI Interface](#openchain-cli)
2. [REST Interface](#openchain-rest-api)
3. [Node.js](#nodejs)

## Openchain CLI:

To see what CLI commands are available, execute the following commands:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer

You will see output similar to below (**NOTE**: rootcommand below is hardcoded in the [main.go](https://github.com/openblockchain/obc-peer/blob/master/main.go). Current build will actually create an *obc-peer* executable file).

```
    Usage:
      obc-peer [command]

    Available Commands:
      peer        Run obc peer.
      status      Status of the obc peer.
      stop        Stops the obc peer.
      chaincode   Compiles the specified chaincode.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for openchain


    Use "obc-peer [command] --help" for more information about a command.
```

### Build a chainCode

The build command builds the Docker image for your chainCode and returns the result. The build takes place on the local peer and the result is not broadcast to the entire network. The result will be either a chainCode deployment specification, ChaincodeDeploymentSpec, or an error. An example is below.

`./obc-peer chaincode build --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01 --version=0.1.0`

### Deploy a chainCode

Deploy first calls the build command (above) to create the docker image and subsequently deploys the chainCode package to the validating peer. An example is below.

`./obc-peer chaincode deploy --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01 --version=0.1.0`

The difference between the two commands is evident when you execute them. During the build command you will observe processing on the local peer node only, but during the deploy command you will note processing on the leader and validator nodes as well. Activity is seen on the leader and validator nodes as they are processing the transaction and going through consensus. The response to the chainCode build and deploy commands is defined by ChaincodeDeploymentSpec inside [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChaincodeDeploymentSpec {

    ChaincodeSpec chaincodeSpec = 1;
    // Controls when the chaincode becomes executable.
    google.protobuf.Timestamp effectiveDate = 2;
    bytes codePackage = 3;

}
```

### Verify Results

To verify that the block has been added to the blockchain, use the `/chain` REST endpoint from the command line. Target the IP of either a validator or a peer node. In the example below, 172.17.0.2 is the IP address of either the validator or the peer node and 5000 is the REST interface port defined in [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml).

`curl 172.17.0.2:5000/chain`

An example of the response is below.

```
{
    "height":1,
    "currentBlockHash":"4Yc4yCO95wcpWHW2NLFlf76OGURBBxYZMf3yUyvrEXs5TMai9qNKfy9Yn/=="
}
```

The returned BlockchainInfo message is defined inside [api.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/api.proto).

```
message BlockchainInfo {

    uint64 height = 1;
    bytes currentBlockHash = 2;
    bytes previousBlockHash = 3;

}
```

To verify that a specific block is inside the blockchain, use the `/chain/blocks/{Block}` REST endpoint. Likewise, target the IP of either the validator node or the peer node on port 5000.

`curl 172.17.0.2:5000/chain/blocks/0`

or preferably

`curl 172.17.0.2:5000/chain/blocks/0 > block_0`

The response to this query will be quite large, on the order of 20Mb, as it contains the encoded payload of the chainCode docker package. It will have the following form:

```
{
    "proposerID":"proposerID string",
    "transactions":[{
        "type":1,
        "chaincodeID": {
            "url":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01",
            "version":"0.5.0"
        },
        "payload":"ClwIARJYCk9naXRod...",
        "uuid":"abdcec99-ae5e-415e-a8be-1fca8e38ba71"
    }],
    "stateHash":"PY5YcQRu2g1vjiAqHHshoAhnq8CFP3MqzMslcEAJbnmXDtD+LopmkrUHrPMOGSF5UD7Kxqhbg1XUjmQAi84paw=="
}
```

The Block message is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto).

```
message Block {

    string proposerID = 1;
    google.protobuf.Timestamp Timestamp = 2;
    repeated Transaction transactions = 3;
    bytes stateHash = 4;
    bytes previousBlockHash = 5;

}
```

## Openchain REST API:

You can experiment with the Openchain REST API through any tool of your choice. For example, the curl command line utility or a browser based client such as the Firefox Rest Client or Chrome Postman. However, if you choose to work with the REST API directly through [Swagger](http://swagger.io/) for a nicer UI, please set up the Swagger-UI package locally on your machine per the instructions [here](#to-set-up-swagger-ui). The APIs we are developing are not public at this time and therefore we can not upload them directly to Swagger.io.

**Note on port number:** The Openchain REST interface port is defined as port 5000 inside [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml). If you are sending REST requests to the peer node from the same host machine, use port 5000. If you are not sending REST requests from the same host machine or are running your peer inside Vagrant, make sure that you have port forwarding enabled from the desired host port to the peer REST port 5000. Further note, that if you are working directly with the REST interface Swagger definition file, [rest_api.json](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/rest_api.json), the port specified in the file is port 3000. In order to send requests directly from the Swagger-UI or Swagger-Editor interface, either set up port forwarding from port 3000 to port 5000 on your host machine or edit the Swagger file to the port number of your choice and set up the appropriate port forwarding for that new port.

**Note on test blockchain** If you want to test the REST APIs locally, construct a test blockchain by running the TestServerOpenchain_API_GetBlockCount test implemented inside [api_test.go](https://github.com/openblockchain/obc-peer/blob/master/openchain/api_test.go). This test will create a test blockchain with 5 blocks. Subsequently restart the peer process.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

### REST Endpoints

* [Block](#block)
  * GET /chain/blocks/{Block}
* [Chain](#chain)
  * GET /chain
* [Devops](#devops)
  * POST /devops/build
  * POST /devops/deploy
  * POST /devops/invoke
  * POST /devops/query
* [State](#state)
  * GET /state/{chaincodeID}/{key}

#### Block

* **GET /chain/blocks/{Block}**

Use the Block API to retrieve the contents of various blocks from the blockchain data structure. The returned Block message structure is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto).

```
message Block {

    string proposerID = 1;
    google.protobuf.Timestamp Timestamp = 2;
    repeated Transaction transactions = 3;
    bytes stateHash = 4;
    bytes previousBlockHash = 5;

}
```

#### Chain

* **GET /chain**

Use the Chain API to retrieve the current state of the blockchain. The returned BlockchainInfo message is defined inside [api.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/api.proto) .

```
message BlockchainInfo {

    uint64 height = 1;
    bytes currentBlockHash = 2;
    bytes previousBlockHash = 3;

}
```

#### Devops

* **POST /devops/build**
* **POST /devops/deploy**
* **POST /devops/invoke**
* **POST /devops/query**

Use the Devops APIs to build, deploy, invoke, or query a chainCode. The required ChaincodeSpec and ChaincodeInvocationSpec payloads are defined in [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChaincodeSpec {

    enum Type {
        UNDEFINED = 0;
        GOLANG = 1;
        NODE = 2;
    }

    Type type = 1;
    ChaincodeID chaincodeID = 2;
    ChaincodeInput ctorMsg = 3;
    int32 timeout = 4;
}
```

```
message ChaincodeInvocationSpec {

    ChaincodeSpec chaincodeSpec = 1;
    //ChaincodeInput message = 2;

}
```

The response to a build, deploy, and invoke request is either a message, containing a confirmation of successful execution or an error, containing a reason for the failure. The response to a query request depends on the chaincode implementation, but typically returns values of requested chainCode state variables.

An example of a valid ChaincodeSpec message is shown below. The url parameter represents the path to the directory containing the chainCode. Currently, the assumption is that the chainCode is located on the peer node and the url is a file path. Eventually, we imagine that the url will represent a pointer to a location on github.

```
{
  "type": "GOLANG",
  "chaincodeID":{
      "url":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01",
      "version":"0.1.0"
  }
}
```

An example of a valid ChaincodeInvocationSpec message is shown below. Consult [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto) for more information.

```
{
  "chaincodeSpec":{
      "type": "GOLANG",
      "chaincodeID":{
          "url":"mycc",
          "version":"0.0.1"
      },
      "ctorMsg":{
          "function":"invoke",
          "args":["a", "b", "10"]
      }
  }
}
```

**Note:** The build/deploy process will take a little time as the docker image is being created.

#### State

* **GET /state/{chaincodeID}/{key}**

Use the State API to retrieve the value stored for a key of a specific chainCode. Think of the chaincodeID and key pair as a compound key. You can create a valid query for the state as you know the value of the state being set inside the testing code described in **Note on test blockchain** [here](#openchain-rest-api).

 Take a look at buildTestLedger2 method inside [api_test.go](https://github.com/openblockchain/obc-peer/blob/master/openchain/api_test.go). This method creates the blockchain you are querying against. You will see calls to SetState(chaincodeID string, key string, value []byte) method, which sets the sate of a chainCode and key to a value. You will see the following being set, among others:

```
MyContract1 + code --> code example
MyContract + x --> hello
MyOtherContract + y --> goodbuy
```

By assigning the following chaincodeID and key parameters in the REST call, you can confirm that the state is being set appropriately.

**Note:** This endpoint will likely go away as the Openchain API matures. Accessing the state of a chainCode directly from the application layer presents security issues. Instead of allowing the application developer to directly access the state, we anticipate to retrieve the state of a chainCode via a function within the chainCode itself. This function will be implemented by the chainCode developer to respond with whatever they see fit for describing the value of the state. The function will be invoked as a query transaction (equivalent to a call in Ethereum) on the chainCode from the application layer and will therefore not be recorded on the blockchain.

### To set up Swagger-UI

[Swagger](http://swagger.io/) is a convenient package that allows us to describe and document our API in a single file. The Openchain REST API is described in [rest_api.json](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/rest_api.json). To interact with the peer node directly through the Swagger-UI, please follow the instructions below.

1. Make sure you have Node.js installed on your local machine. If it is not installed, please download the [Node.js](https://nodejs.org/en/download/) package and install it.

2. Install the Node.js http-server package with the command below:

    `npm install http-server -g`

3. Start up an http-server on your local machine to serve up the rest_api.json.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer/openchain/rest
    http-server -a 0.0.0.0 -p 5554 --cors
    ```

4. Make sure that you are successfully able to access the API description document within your browser at this link:

    `http://localhost:5554/rest_api.json`

5. Download the Swagger-UI package with the following command:

    `git clone https://github.com/swagger-api/swagger-ui.git`

6. Navigate to the /swagger-ui/dist directory and click on the index.html file to bring up the Swagger-UI interface inside your browser.

7. Start up the peer node with no connections to a leader or validator as follows.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    ./obc-peer peer
    ```

8. Construct a test blockchain on the local peer node by running the TestServerOpenchain_API_GetBlockCount test implemented inside [api_test.go](https://github.com/openblockchain/obc-peer/blob/master/openchain/api_test.go). This test will create a blockchain with 5 blocks. Subsequently restart the peer process.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

9. Go back to the Swagger-UI interface inside your browser and load the API description.

## Node.js

You can interface to the obc-peer process from a Node.js application in one of two ways. Both approaches rely on the Swagger API description document, [rest_api.json](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/rest_api.json). Use the approach the you find the most convenient.

### [OpenchainSample_1](https://github.com/openblockchain/obc-peer/blob/master/docs/Openchain%20Samples/openchain_1.js)

* Demonstrates interfacing to the obc-peer project from a Node.js app.
* Utilizes the Node.js swagger-js plugin: https://github.com/swagger-api/swagger-js

**To run:**

1. Build and install [obc-peer](https://github.com/openblockchain/obc-peer/blob/master/README.md).

2. Run local peer node only (not complete network) with:

    `./obc-peer peer`

3. Set up a test blockchain data structure (with 5 blocks only) by running a test from within Vagrant as follows.  Subsequently restart the peer process.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

4. Set up HTTP server to serve up the Openchain API Swagger doc at a non-public URL:

    ```
    npm install http-server -g
    cd /opt/gopath/src/github.com/openblockchain/obc-peer/openchain/rest
    http-server -a 0.0.0.0 -p 5554 --cors
    ```

5. Download [OpenchainSample_1.zip](https://github.com/openblockchain/obc-peer/blob/master/docs/Openchain%20Samples/OpenchainSample_1.zip)

    ```
    unzip OpenchainSample_1.zip -d OpenchainSample_1
    cd OpenchainSample_1
    ```

6. Update the api_url within [openchain.js](https://github.com/openblockchain/obc-peer/blob/master/docs/Openchain%20Samples/openchain_1.js) to the appropriate URL if it is not already the default

    `var api_url = 'http://localhost:5554/rest_api.json';`

7. Run the Node.js app

    `node ./openchain.js`

You will observe several responses on the console and the program will appear to hang for a few moments at the end. This is normal, as is it waiting for a build request for a Docker container to complete.

### [OpenchainSample_2](https://github.com/openblockchain/obc-peer/blob/master/docs/Openchain%20Samples/openchain_2.js)

* Demonstrates an alternative way of interfacing to the obc-peer project from a Node.js app.
* Utilizes the TypeScript description of Openchain REST API generated through the Swagger-Editor.
* Utilizes the DefinitelyTyped TypeScript definitions manager package: https://github.com/DefinitelyTyped/tsd

**To run:**

1. Build and install [obc-peer](https://github.com/openblockchain/obc-peer/blob/master/README.md).

2. Run local peer node only (not complete network) with:

    `./obc-peer peer`

3. Set up a test blockchain data structure (with 5 blocks only) by running a test from within Vagrant as follows. Subsequently restart the peer process.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

4. Download [OpenchainSample_2.zip](https://github.com/openblockchain/obc-peer/blob/master/docs/Openchain%20Samples/OpenchainSample_2.zip)

    ```
    unzip OpenchainSample_2.zip -d OpenchainSample_2
    cd OpenchainSample_1
    ```

5. Run the Node.js app

    `node ./openchain.js`

You will observe several responses on the console and the program will appear to hang for a few moments at the end. This is normal, as is it waiting for a build request for a Docker container to complete.

### To Regenerate TypeScript

If you update the [rest_api.json](https://github.com/angrbrd/obc-peer/blob/master/openchain/rest/rest_api.json) Swagger description, you must regenerate the associated TypeScript file for your Node.js application. The current TypeScript file describing the Openchain API is [api.ts](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/api.ts).

Swagger produces TypeScript files with its Swagger-Editor package. If you would like to use Swagger-Editor, please set it up locally on your machine. The APIs we are working on are not public at this time and therefore we can not upload them directly to Swagger.io. To set up the Swagger-Editor please follow the steps below.

1. Download the latest release of [Swagger-Editor](https://github.com/swagger-api/swagger-editor).
2. Unpack .zip
3. cd swagger-editor/swagger-editor
4. http-server -a 0.0.0.0 -p 8000 --cors
5. Go to the Swagger-Editor in your browser, and import the API description.
6. Generate the TypeScript file for Node.js from the "Generate Client" menu.
