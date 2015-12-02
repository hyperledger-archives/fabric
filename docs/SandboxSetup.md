#Writing, Building, And Running Chaincode In Development Environment

Chaincode developers need a way to test and debug their chaincode quickly without having to set up a complete Openchain network.

This document contains instructions on how to write, build, and test chaincode in a development sandbox locally.

We need 3 terminal windows: the first terminal runs the validating peer; the second terminal runs the chaincode; and the third terminal runs the CLI (or REST API) to execute transactions.

###Window 1 (validating peer)
Run obc-peer:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer peer --peer-chaincodedev

###Window 2 (chaincode)
Use the example chaincode_example02 provided in the source code repository and build it:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02
    go build

When you are ready to start creating your own chaincodes, create a new directory inside the /openchain/example/chaincode directory to hold your chaincode file. You may copy and paste the chaincode_example02 file and make modifications to get started.

Run the chaincode command below to start and register the chaincode with the validating peer (started in window 1):

    OPENCHAIN_CHAINCODE_ID_URL=mycc OPENCHAIN_CHAINCODE_ID_VERSION=0.0.1 OPENCHAIN_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02

If you don't see “Received REGISTERED, ready for invocations” message, do not proceed farther. Something is wrong. Revisit the steps above to resolve the issue.

###Window 3 (CLI or REST API)
The chaincode is ready to receive requests. Please follow the steps detailed below to send a chaincode deploy transaction, chaincode invoke transaction, and chaincode query.

**Note:** The Openchain REST interface port is defined as port 5000 inside the openchain.yaml file. If you are sending REST requests to the peer node from the same host machine, use port 5000. If you are not issuing REST requests from the same host machine or are running your peer inside Vagrant, make sure that you have port forwarding enabled from the desired host port to the Vagrant REST port 5000. Further note, that if you are working directly with the REST interface Swagger definition file, rest_api.json, the port specified in the file is port 3000. In order to send requests directly from the Swagger-UI or Swagger-Editor interface, either set up port forwarding from port 3000 to 5000 on your host machine or edit the Swagger file to the port number of your choice and set up the appropriate port forwarding for that port.

#### Chaincode deploy via CLI and REST

First we send a chaincode deploy transaction, only once, to the validating peer. The CLI knows how to connect to the validating peer based on the properties defined in the openchain.yaml file.

    cd  $GOPATH/src/github.com/openblockchain/obc-peer
 	./obc-peer chaincode deploy -p mycc -v 0.0.1 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

Alternatively, you can run the chaincode deploy transaction through the REST API.

```
POST localhost:5000/devops/deploy

{
  "type": "GOLANG",
  "chaincodeID":{
      "url":"mycc",
      "version":"0.0.1"
  },
  "ctorMsg": {
      "function":"init",
      "args":["a", "100", "b", "200"]
  }
}

200 OK
{
    “Status”: “OK. Successfully deployed chainCode.”
}
```

The deploy transaction initializes the chaincode using the given transaction name. Though the example shows “init”, the name could be arbitrarily chosen by the chaincode developer.  You should see the following output in the chaincode window:

	2015/11/15 15:19:31 Received INIT(uuid:005dea42-d57f-4983-803e-3232e551bf61), initializing chaincode
	Aval = 100, Bval = 200

#### Chaincode invoke via CLI and REST

Run the chaincode invoking transaction on the CLI as many times as desired:

	./obc-peer chaincode invoke -l golang -p mycc -v 0.0.1 -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

Run the chaincode invoke transaction through the REST API:

```
POST localhost:5000/devops/invoke

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

200 OK
{
    “Status”: “OK. Successfully invoked transaction.”
}
```

The invoke transaction runs the specified transaction name “invoke” with the arguments. This transaction transfers 10 units from A to B. You should see the following output in the chaincode window:

	2015/11/15 15:39:11 Received RESPONSE. Payload 200, Uuid 075d72a4-4d1f-4a1d-a735-4f6f60d597a9
	Aval = 90, Bval = 210

#### Chaincode query via CLI and REST

Run a query on the chaincode to retrieve desired values. The “-p” and “-v” args should match those provided in the Chaincode Window.

    ./obc-peer chaincode query -l golang -p mycc -v 0.0.1 -c '{"Function": "query", "Args": ["b"]}'

You should get  a response similar to:

    {"Name":"b","Amount":"210"}

If a name other than “a” or “b” is provided in a query sent to chaincode_example02, you should get an error response:

    {"Error":"Nil amount for c"}

Run the chaincode query transaction through the REST API:

```
POST localhost:5000/devops/query

{
  "chaincodeSpec":{
      "type": "GOLANG",
      "chaincodeID":{
          "url":"mycc",
          "version":"0.0.1"
      },
      "ctorMsg":{
          "function":"query",
          "args":["a"]
      }
  }
}

200 OK
{
    "OK": {
        "Name": "a",
        "Amount": "70"
    }
}
```

**Note:** We are still working on finalizing the details of the response and error codes for the REST interface. This will be addressed shortly.
