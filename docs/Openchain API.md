# Openchain API - Working with CLI, REST, and Node.js

## Overview

This document covers the available APIs to connect to an Openchain peer node. The three approaches described are:

1. [CLI Interface](#openchain-cli)
2. [REST Interface](#openchain-rest-api)
3. [Node.js](#nodejs)

## Openchain CLI:

To see what CLI commands are currently available, simply execute the following command:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer

You should see some output similar to below (**NOTE**: rootcommand below is hardcoded in the [main.go](https://github.com/openblockchain/obc-peer/blob/master/main.go). Current build will actually create an *obc-peer* executable file).

```
    Usage:
      obc-peer [command]

    Available Commands:
      peer        Run obc peer.
      status      Status of the obc peer.
      stop        Stops the obc peer.
      chainlet    Compiles the specified chainlet.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for openchain


    Use "obc-peer [command] --help" for more information about a command.
```

### Build a chainCode

The build command builds the Docker image for your chainCode and returns the result. The build takes place on the local peer and the result is not broadcast to the entire network. The result will be either a chainCode deployment specification, ChainletDeploymentSpec, or an error. An example is below.

`./obc-peer chaincode build --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple --version=0.1.0`

### Deploy a chainCode

Deploy first calls the build command (above) to create the docker image and subsequently deploys the chainCode package to the validating peer. An example is below.

`./obc-peer chaincode deploy --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple --version=0.1.0`

The difference between the two commands is evident when you execute them. During the build command you will only observe processing on the local peer node, but during the deploy command you will note processing on the leader and validator nodes as well. Activity is seen on the leader and validator nodes as they are processing the transaction and going through consensus. The response to the chainCode build and deploy commands is defined by ChainletDeploymentSpec inside [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChainletDeploymentSpec {

    ChainletSpec chainletSpec = 1;
    // Controls when the chaincode becomes executable.
    google.protobuf.Timestamp effectiveDate = 2;
    bytes codePackage = 3;

}
```

**Note:** We are currently in the process of replacing all instances of "chainlet" with "chainCode" as we found that terminology more suitable. You will still see references to chainlet in the code as we are going through this transition process.

### Verify Results

To verify that the block has been added to the blockchain, use the `/chain` REST endpoint from the Vagrant command line. Target the IP of either a validator or a peer node. In the example below, 172.17.0.2 is the IP address of either the validator or the peer node and 5000 is the REST interface port.

`curl 172.17.0.2:5000/chain`

An example of the response is below.

```
{
    "height":1,
    "currentBlockHash":"4Yc4yCO95wcpWHW2NLFlf76OGURBBxYZMf3yUyvrEXs5TMai9qNKfy9Yn/=="
}
```

The returned BlockchainInfo message is defined inside [api.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/api.proto) file.

```
message BlockchainInfo {

    uint64 height = 1;
    bytes currentBlockHash = 2;
    bytes previousBlockHash = 3;

}
```

To verify that a specific block is inside the blockchain, use the `/chain/blocks/{Block}` REST endpoint. Likewise, target the IP of either the validator node or the peer node on port 5000 within the Vagrant environment.

`curl 172.17.0.2:5000/chain/blocks/0`

or preferably

`curl 172.17.0.2:5000/chain/blocks/0 > block_0`

The response to this query will be quite large, on the order of 20Mb, as it contains the encoded payload of the chainCode docker package. It will have the following form:

```
{
    "proposerID":"proposerID string",
    "transactions":[{
        "type":1,
        "chainletID": {
            "url":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple",
            "version":"0.5.0"
        },
        "payload":"ClwIARJYCk9naXRod...",
        "uuid":"abdcec99-ae5e-415e-a8be-1fca8e38ba71"
    }],
    "stateHash":"PY5YcQRu2g1vjiAqHHshoAhnq8CFP3MqzMslcEAJbnmXDtD+LopmkrUHrPMOGSF5UD7Kxqhbg1XUjmQAi84paw=="
}
```

The Block is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto) file.

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

You can experiment with the Openchain REST API through any tool of your choice. For example, the curl command line utility or a browser based client such as the Firefox Rest Client or Chrome Postman. However, if you choose to work with the REST API directly through Swagger for a nicer UI, please set up the Swagger-UI package locally on your machine per the instructions [here](#to-set-up-swagger-ui). The APIs we are working on are not public at this time and therefore we can not upload them directly to Swagger.io.

### REST Endpoints

* [Block](#block)
  * GET /chain/blocks/{Block}
* [Chain](#chain)
  * GET /chain
* [Devops](#devops)
  * POST /devops/build
  * POST /devops/deploy
* [State](#state)
  * GET /state/{chaincodeID}/{key}

#### Block

* **GET /chain/blocks/{Block}**

Invoke the Block API to retrieve the contents of various blocks from the blockchain data structure. The returned Block message structure is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto).

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

Invoke the Chain API to retrieve the information on the current state of the blockchain. The returned BlockchainInfo message is defined inside [api.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/api.proto) .

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

Invoke the Devops APIs to trigger a chainCode build or a chainCode deploy respectively. The required ChainletSpec payload is defined in [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChainletSpec {

    enum Type {
        UNDEFINED = 0;
        GOLANG = 1;
        NODE = 2;
    }

    Type type = 1;
    ChainletID chainletID = 2;
    ChainletMessage ctorMsg = 3;

}
```

The response to both build and deploy requests is a ChainletDeploymentSpec defined in [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChainletDeploymentSpec {

    ChainletSpec chainletSpec = 1;
    // Controls when the chaincode becomes executable.
    google.protobuf.Timestamp effectiveDate = 2;
    bytes codePackage = 3;

}
```

An example of a valid ChainletSpec message is shown below. The url parameter represents the file path of the directory that contains the chainCode file. At this point, the assumption is that the chainCode is located on the peer node and it is a file path. Eventually, we imagine that the url will represent a pointer to a location on github.

```
{
  "type": "GOLANG",
  "chainletID":{
      "url":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple",
      "version":"0.1.0"
  }
}
```

**Note:** The build/deploy process will take a little time as the docker image is being created. The response, containing the encoded payload, will be quite large.

#### State

* **GET /state/{chaincodeID}/{key}**

Invoke the State endpoint to retrieve the value stored for a given key of a specific chainCode. Think of the chaincodeID and key as a compound key. You can create a valid query for the state as you know the value of the state being set inside the testing code. Take a look at buildTestLedger2 method inside [api_test.go](https://github.com/openblockchain/obc-peer/blob/master/openchain/api_test.go). This method creates the blockchain you are querying against. You will see calls to SetState(chaincodeID string, key string, value []byte) method, which sets the sate of a particular chainCode and key to a specific value. You will see the following being set, among others:

```
MyContract1 + code --> code example
MyContract + x --> hello
MyOtherContract + y --> goodbuy
```

By assigning the following chaincodeID and key parameters within the REST call, you can confirm that the state is being set appropriately.

**Note:** This endpoint will likely go away as the Openchain API matures. Accessing the state of a chainCode directly from the application layer presents security issues. Instead of allowing the application developer to directly access the state, we anticipate to retrieve the state of a given chainCode via a getter function within the chainCode itself. This getter function will be implemented by the chainCode developer to respond with whatever they see fit for describing the value of the state. The getter function will be invoked as a query transaction (equivalent to call in Ethereum) on the chainCode from the application layer and will therefore not be recorded as a transaction on the blockchain.

### To set up Swagger-UI

Swagger is a convenient package that allows us to describe and document our API in a single file. The Openchain REST API is described inside [rest_api.json](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/rest_api.json). To interact with the peer node directly through the Swagger-UI, please follow the instructions below.

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

8. Construct a test blockchain on the local peer node by running the TestServerOpenchain_API_GetBlockCount test implemented inside [api_test.go](https://github.com/openblockchain/obc-peer/blob/master/openchain/api_test.go). This test will create a blockchain with 5 blocks.

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

10. Go back to the Swagger-UI interface inside your browser and load the API description.

## Node.js

You can interface to the obc-peer process from a Node.js application in one of two ways. Both approaches rely on the Swagger API description document, [rest_api.json](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/rest_api.json). Use the approach the you find the most convenient.

### [OpenchainSample_1](https://github.com/openblockchain/obc-peer/blob/master/docs/OpenchainSample_1.js)

* Demonstrates interfacing to the obc-peer project from a Node.js app.
* Utilizes the Node.js swagger-js plugin: https://github.com/swagger-api/swagger-js

**To run:**

1. Build and install [obc-peer](https://github.com/openblockchain/obc-peer/blob/master/README.md).

2. Run local peer node only (not complete network) with:

    `./obc-peer peer`

3. Set up a test blockchain data structure (with 5 blocks only) by running a test from within Vagrant as follows:

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

5. Download [OpenchainSample_1.zip](https://github.com/openblockchain/obc-peer/blob/master/docs/OpenchainSample_1.zip)

    ```
    unzip OpenchainSample_1.zip -d OpenchainSample_1
    cd OpenchainSample_1
    ```

6. Update the api_url within [openchain.js](https://github.com/openblockchain/obc-peer/blob/master/docs/OpenchainSample_1.js) to the appropriate URL if it is not already the default

    `var api_url = 'http://localhost:5554/rest_api.json';`

7. Run the Node.js app

    `node ./openchain.js`

You will observe several responses on the console, thought the program will appear to hang for a few moments at the end. This is normal, as is it waiting for a build request for a Docker container to complete.

### [OpenchainSample_2](https://github.com/openblockchain/obc-peer/blob/master/docs/OpenchainSample_1.js)

* Demonstrates an alternative way of interfacing to the obc-peer project from a Node.js app.
* Utilizes the TypeScript description of Openchain REST api generated through the Swagger-Editor.
* Utilizes the Node.js TypeScript extension: https://github.com/theblacksmith/typescript-require

**To run:**

1. Build and install [obc-peer](https://github.com/openblockchain/obc-peer/blob/master/README.md).

2. Run local peer node only (not complete network) with:

    `./obc-peer peer`

3. Set up a test blockchain data structure (with 5 blocks only) by running a test from within Vagrant as follows:

    ```
    cd /opt/gopath/src/github.com/openblockchain/obc-peer
    go test -v -run TestServerOpenchain_API_GetBlockCount github.com/openblockchain/obc-peer/openchain
    ```

5. Download [OpenchainSample_2.zip](https://github.com/openblockchain/obc-peer/blob/master/docs/OpenchainSample_2.zip)

    ```
    unzip OpenchainSample_2.zip -d OpenchainSample_2
    cd OpenchainSample_1
    ```

6. Run the Node.js app

    `node ./openchain.js`

You will observe several responses on the console, thought the program will appear to hang for a few moments at the end. This is normal, as is it waiting for a build request for a Docker container to complete.

### To Regenerate TypeScript

If you update the [rest_api.json](https://github.com/angrbrd/obc-peer/blob/master/openchain/rest/rest_api.json) Swagger description, you must regenerate the associated TypeScript file for your Node.js application. The current TypeScript file describing the Openchain API is [api.ts](https://github.com/openblockchain/obc-peer/blob/master/openchain/rest/api.ts).

Swagger produces TypeScriptfile files with its Swagger-Editor package. If you would like to use Swagger-Editor, please set it up locally on your machine. The APIs we are working on are not public at this time and therefore we can not upload them directly to Swagger.io. To set up the Swagger-Editor please follow the steps below.

1. Download the latest release of [Swagger-Editor](https://github.com/swagger-api/swagger-editor).
2. Unpack .zip
3. cd swagger-editor/swagger-editor
4. http-server -a 0.0.0.0 -p 8000 --cors
5. Go to the Swagger-Editor in your browser, and import the API description.
6. Generate the TypeScript file for Node.js from the "Generate Client" menu.
