# Openchain APIs - CLI, REST, and Node.js

## Overview

This document covers the available APIs to interact with an Openchain peer node. The three approaches described are:

1. [CLI Interface](#openchain-cli)
2. [REST Interface](#openchain-rest-api)
3. [Node.js Application](#nodejs)

**Note:**: If you are working with APIs with security enabled, please review the [security setup instructions](https://github.com/openblockchain/obc-peer/blob/master/docs/SandboxSetup.md#security-setup-optional) before proceeding.

## Openchain CLI:

To see what CLI commands are available, execute the following:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer

You will see output similar to below (**NOTE:** rootcommand below is hardcoded in the [main.go](https://github.com/openblockchain/obc-peer/blob/master/main.go). Currently, the build will create an *obc-peer* executable file).

```
    Usage:
      openchain [command]

    Available Commands:
      peer        Run openchain peer.
      status      Status of the openchain peer.
      stop        Stop openchain peer.
      login       Login user on CLI.
      vm          VM functionality of openchain.
      chaincode   chaincode specific commands.
      help        Help about any command

    Flags:
      -h, --help[=false]: help for openchain


    Use "openchain [command] --help" for more information about a command.
```

### Deploy a Chaincode

Deploy creates the docker image and subsequently deploys the chaincode package to the validating peer. An example is below.

`./obc-peer chaincode deploy --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01 --version=0.1.0`

The response to the chaincode deploy command is defined by ChaincodeDeploymentSpec inside [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

```
message ChaincodeDeploymentSpec {
    ChaincodeSpec chaincodeSpec = 1;
    // Controls when the chaincode becomes executable.
    google.protobuf.Timestamp effectiveDate = 2;
    bytes codePackage = 3;
}
```

With security enabled, modify the command to include the -u parameter passing the username of a logged in user as follows:

`./obc-peer chaincode deploy -u jim --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01 --version=0.1.0`

### Verify Results

To verify that the block containing the latest transaction has been added to the blockchain, use the `/chain` REST endpoint from the command line. Target the IP of either a validator or a peer node. In the example below, 172.17.0.2 is the IP address of either the validator or the peer node and 5000 is the REST interface port defined in [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml).

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

The response to this query will be quite large, on the order of 20Mb, as it contains the encoded payload of the chaincode docker package. It will have the following form:

```
{
    "proposerID":"proposerID string",
    "transactions":[{
        "type":1,
        "chaincodeID": {
            "path":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01"
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
    google.protobuf.Timestamp timestamp = 2;
    repeated Transaction transactions = 3;
    bytes stateHash = 4;
    bytes previousBlockHash = 5;
    bytes consensusMetadata = 6;
    NonHashData nonHashData = 7;
}
```

## Openchain REST API:

You can experiment with the Openchain REST API through any tool of your choice. For example, the curl command line utility or a browser based client such as the Firefox Rest Client or Chrome Postman. However, if you choose to work with the REST API directly through [Swagger](http://swagger.io/), please set up the Swagger-UI package locally on your machine per the instructions [here](#to-set-up-swagger-ui). The APIs we are developing are not public at this time and therefore we can not upload them directly to Swagger.io.

**Note:** The Openchain REST interface port is defined as port 5000 inside the [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml). If you are sending REST requests to the peer node from the same machine, use port 5000. If you are sending REST requests through Swagger, the port specified in the Swagger file is port 3000. This is done to emphasize that Swagger will likely not run on the same machine as the peer process or outside Vagrant. In order to send requests from the Swagger-UI or Swagger-Editor interface, set up port forwarding from port 3000 to 5000 on your machine or edit the Swagger file to the port number of your choice.

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
* [Transactions](#transactions)
  * GET /transactions/{UUID}
* [Devops](#devops)
  * POST /devops/deploy
  * POST /devops/invoke
  * POST /devops/query
* [Registrar](#registrar)
  * POST /registrar
  * GET /registrar/{enrollmentID}
  * DELETE /registrar/{enrollmentID}
  * GET /registrar/{enrollmentID}/ecert

#### Block

* **GET /chain/blocks/{Block}**

Use the Block API to retrieve the contents of various blocks from the blockchain. The returned Block message structure is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto).

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

#### Transactions

* **GET /transactions/{UUID}**

Use the /transactions/{UUID} endpoint to retrieve an individual transaction matching the UUID from the blockchain. The returned transaction message is defined inside [openchain.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/openchain.proto) .

```
message Transaction {
    enum Type {
        UNDEFINED = 0;
        CHAINCODE_NEW = 1;
        CHAINCODE_UPDATE = 2;
        CHAINCODE_EXECUTE = 3;
        CHAINCODE_QUERY = 4;
        CHAINCODE_TERMINATE = 5;
    }
    Type type = 1;
    bytes chaincodeID = 2;
    bytes payload = 3;
    string uuid = 4;
    google.protobuf.Timestamp timestamp = 5;

    ConfidentialityLevel confidentialityLevel = 6;
    bytes nonce = 7;

    bytes cert = 8;
    bytes signature = 9;
}
```

#### Devops

* **POST /devops/deploy**
* **POST /devops/invoke**
* **POST /devops/query**

Use the Devops APIs to deploy, invoke, and query chaincodes. The required ChaincodeSpec and ChaincodeInvocationSpec payloads are defined in [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto).

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
    string secureContext = 5;
    ConfidentialityLevel confidentialityLevel = 6;
}
```

```
message ChaincodeInvocationSpec {
    ChaincodeSpec chaincodeSpec = 1;
    //ChaincodeInput message = 2;
}
```

**Note:** The deploy transaction requires a 'path' parameter to locate, build, and deploy the chaincode. On the other hand, invoke and query transactions require a 'name' parameter. These parameters are specified in the ChaincodeID, defined in [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto). The only exception to this rule is if the peer is running in development mode, i.e. the user deploys the chaincode themselves. In that case, the deploy transaction also requires a 'name' parameter.

```
message ChaincodeID {
    //deploy transaction will use the path
    string path = 1;

    //all other requests will use the name (really a hashcode) generated by
    //the deploy transaction
    string name = 2;
}
```

The response to deploy and invoke requests is either a message containing a confirmation of successful execution or an error, containing a reason for the failure. The response to a deployment request also contains the assigned chaincode name, which is to be used in subsequent invocation and query transactions. The response to a query request depends on the chaincode implementation.

An example of a valid ChaincodeSpec message is shown below. The 'path' parameter specifies the location of the chaincode in the filesystem. Eventually, we imagine that the 'path' will represent a location on GitHub.

```
{
  "type": "GOLANG",
  "chaincodeID":{
      "path":"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01",
  }
}
```

An example of a valid ChaincodeInvocationSpec message is shown below. Consult [chaincode.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/chaincode.proto) for more information.

```
{
  "chaincodeSpec":{
      "type": "GOLANG",
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg":{
          "function":"invoke",
          "args":["a", "b", "10"]
      }
  }
}
```

With security enabled, modify the payload to include the secureContext element passing the enrollmentID of a logged in user as follows:

```
{
  "chaincodeSpec":{
      "type": "GOLANG",
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg":{
          "function":"invoke",
          "args":["a", "b", "10"]
      },
  	  "secureContext": "jim"
  }
}
```

**Note:** The deployment process will take a little time as the docker image is being created.

#### Registrar

* **POST /registrar**
* **GET /registrar/{enrollmentID}**
* **DELETE /registrar/{enrollmentID}**
* **GET /registrar/{enrollmentID}/ecert**

Use the Registrar APIs to manage end user registration with the CA. These API endpoints are used to register a user with the CA, determine whether a given user is registered, and to remove any login tokens for a target user preventing them from executing any further transactions. The Registrar APIs are also used to retrieve user enrollment certificates from the system.

The /registrar enpoint is used to register a user with the CA. The required Secret payload is defined in [devops.proto](https://github.com/openblockchain/obc-peer/blob/master/protos/devops.proto).

```
message Secret {
    string enrollId = 1;
    string enrollSecret = 2;
}
```

The response to the registration request is either a confirmation of successful registration or an error, containing a reason for the failure. An example of a valid Secret message to register user 'lukas' is shown below.

```
{
  "enrollId": "lukas",
  "enrollSecret": "NPKYL39uKbkj"
}
```

The GET /registrar/{enrollmentID} endpoint is used to confirm whether a given user is registered with the CA. If so, a confirmation will be returned. Otherwise, an authorization error will result. The DELETE /registrar/{enrollmentID} endpoint is used to delete login tokens for a target user. If the login tokens are deleted successfully, a confirmation will be returned. Otherwise, an authorization error will result. No payload is required for this endpoint.

The GET /registrar/{enrollmentID}/ecert endpoint is used to retrieve the enrollment certificate of a given user from local storage. If the target user has already registered with the CA, the response will include a URL-encoded version of the enrollment certificate. If the target user has not yet registered, an error will be returned. If the client wishes to use the returned enrollment certificate after retrieval, keep in mind that it must be URl-decoded. This can be accomplished with the QueryUnescape method in the "net/url" package.

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
