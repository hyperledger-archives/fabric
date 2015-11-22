##Running Chaincode In Development Environment

Chaincode developers need a way to test and debug their chaincode quickly without having to set up the full Openchain network.

This document contains instructions on how to run and test chaincode in a development sandbox locally.

###Setup Environment

Go to obc-peer directory:

    cd $GOPATH/src/github.com/openblockchain/obc-peer

Edit openchain.yaml to set the peer mode “mode” to “dev”, as shown below:

	 mode: dev
	
Also edit openchain.yaml to set the “chaincoderunmode” to “dev_mode”, as shown below:

    chaincoderunmode: dev_mode

This will run the chaincode independent of the validating peers. We need 3 terminal windows to show activities; the first terminal runs the validating peer; the second terminal runs the chaincode; and the third terminal runs the CLI (or REST API) to execute the transactions.

####Window 1 (validating peer window)
Run obc-peer

    ./obc-peer peer

####Window 2 (chaincode window)
Use the example chaincode_example02 in the source code and build it: 

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02
    go build
    
Run the chaincode commands to start and register it with the validating peer started on window 1 above

    OPENCHAIN_CHAINCODE_ID_URL=mycc OPENCHAIN_CHAINCODE_ID_VERSION=0.0.1 OPENCHAIN_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02

If you don't see “Received REGISTERED, ready for invocations” …. STOP: Something is wrong. Revisit the steps above to resolve the issue.

####Window 3 (CLI or REST API window)
The chaincode is now ready to receive requests. First we tell the chaincode about the validating peer so that they can connect by sending a chaincode deploying transaction to the validating peer. The CLI knows where to find the validating peer based on the properties defined in the openchain.yaml file.

    cd  $GOPATH/src/github.com/openblockchain/obc-peer
 	./obc-peer chaincode deploy -p mycc -v 0.0.1 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
 	
Alternatively, you can run the chaincode deploying transaction through the REST API:

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

The deploying transaction initializes the chaincode using the given transaction name (though the example shows “init”, the name could be arbitrarily chosen by the chaincode developer).  You should see the following output in the chaincode window:

	2015/11/15 15:19:31 Received INIT(uuid:005dea42-d57f-4983-803e-3232e551bf61), initializing chaincode
	Aval = 100, Bval = 200

Run the chaincode invoking transaction on the CLI:

	./obc-peer chaincode invoke -l golang -p mycc -v 0.0.1 -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

Run the chaincode invoking transaction through the REST API:

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

The invoking transaction runs the specified transaction name “invoke” with the arguments. This transaction transfers 10 units from A to B. You should see the following output in the chaincode window:

	2015/11/15 15:39:11 Received RESPONSE. Payload 200, Uuid 075d72a4-4d1f-4a1d-a735-4f6f60d597a9
	Aval = 90, Bval = 210

Run a query on the chaincode to retrieve desired values (the “-p” and “-v” args should match those provided in the Chaincode Window)

    ./obc-peer chaincode query -l golang -p mycc -v 0.0.1 -c '{"Function": "query", "Args": ["b"]}'

You should get  a response similar to:
    
    {"Name":"b","Amount":"210"}

If a name other than “a” or “b” is provided, you should get an error response:

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

Please note that we are still working on finalizing the details of the response and error codes for the REST interface. This will be addressed shortly.
