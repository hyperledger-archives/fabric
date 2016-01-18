#Writing, Building, And Running Chaincode In A Development Environment

Chaincode developers need a way to test and debug their chaincode without having to set up a complete Openchain network.

This document contains instructions on how to write, build, and test chaincode in a local development environment.

We need multiple terminal windows: one terminal runs the validating peer, another terminal runs the chaincode, the third terminal runs the CLI or REST API commands to execute transactions. When running with security enabled, an additional terminal window will be required in order to run the CA server. Detailed instructions are provided in the sections below.

* [Security Setup (optional)](#security-setup-optional)
* [Window 1 (validating peer)](#window-1-validating-peer)
* [Window 2 (chaincode)](#window-2-chaincode)
* [Window 3 (CLI or REST API)](#window-3-cli-or-rest-api)
    * [Chaincode deploy via CLI and REST](#chaincode-deploy-via-cli-and-rest)
    * [Chaincode invoke via CLI and REST](#chaincode-invoke-via-cli-and-rest)
    * [Chaincode query via CLI and REST](#chaincode-query-via-cli-and-rest)
* [Removing temporary files when security is enabled](#removing-temporary-files-when-security-is-enabled)

###Security Setup (optional)
To set up the local development environment with security enabled, you must first build and run the CA server.

    cd $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca
    go build -o obcca-server
    ./obcca-server

The above commands build and run the CA server with the default setup in [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml). The default configuration includes multiple users already registered with the CA. These users are listed in the 'users' section of the configuration file. To register additional users with the CA for testing, modify the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) to add more enrollmentID and enrollmentPW pairs.

###Window 1 (validating peer)
**Note:** When running with security enabled, first modify the [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml) to set the security.enabled setting to 'true' before building the peer executable. Alternatively, you may run the peer with OPENCHAIN_SECURITY_ENABLED=true to enable security.

Build and run the peer process:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    go build
    ./obc-peer peer --peer-chaincodedev   

Alternatively:

    OPENCHAIN_SECURITY_ENABLED=true ./obc-peer peer --peer-chaincodedev

###Window 2 (chaincode)
Use the example chaincode_example02 provided in the source code repository and build it:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02
    go build

When you are ready to start creating your own chaincodes, create a new directory inside the /openchain/example/chaincode directory to hold your chaincode file. You may copy and paste the chaincode_example02 file and make modifications to get started.

Run the chaincode command below to start and register the chaincode with the validating peer (started in window 1):

    OPENCHAIN_CHAINCODE_ID_NAME=mycc OPENCHAIN_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02

You will see the following message on the chaincode console: “Received REGISTERED, ready for invocations”. This message indicates that the chaincode is ready to receive requests. Please follow the steps detailed below to send a chaincode deploy, invoke, or query transaction. If the REGISTERED message is not received, something had gone wrong during the deployment. Revisit the steps above to resolve the issue.

###Window 3 (CLI or REST API)

**Note on REST API port**

The Openchain REST interface port is defined as port 5000 inside the [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml). If you are sending REST requests to the peer node from the same machine, use port 5000. If you are sending REST requests through Swagger, the port specified in the Swagger file is port 3000. This is done to emphasize that Swagger will likely not run on the same machine as the peer process or outside Vagrant. In order to send requests from the Swagger-UI or Swagger-Editor interface, set up port forwarding from port 3000 to 5000 on your machine or edit the Swagger file to the port number of your choice.

**Note on security functionality**

When security is enabled, the CLI commands and REST payloads must be modified to include an enrollmentID of a registered and logged in user, otherwise an error will result. A registered user may be logged in through the CLI or the REST API by following the instructions below. Any user registered with the CA may undergo the registration process utilizing the enrollmentID and enrollmentPW only once. If the registration is attempted a second time, an error will result. This assumes the authentication process to take place at the application layer, while the application will handle the user registration with the CA on behalf of an authenticated user exactly once, storing their enrollment certificate.

To log in a user through the CLI, issue the commands bellow, where 'username' is one of the enrollmentIDs listed in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml).

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer login <username>

The command will prompt for a password, which must match the enrollmentPW listed for the target user in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml). If the password entered does not match the enrollmentPW, an error will result.

To log in the user through the REST API, send a POST request to the /registrar endpoint, containing the enrollmentID and enrollmentPW, listed in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml).

```
POST localhost:3000/registrar

{
  "enrollId": "lukas",
  "enrollSecret": "NPKYL39uKbkj"
}

200 OK
{
    "OK": "Login successful for user 'lukas'."
}
```

#### Chaincode deploy via CLI and REST

First send a chaincode deploy transaction, only once, to the validating peer. The CLI knows how to connect to the validating peer based on the properties defined in the openchain.yaml file. **Note:** The deploy transaction requires a 'path' parameter to locate, build, and deploy the chaincode. However, as these instructions are for local development mode and the chaincode is deployed manually, the 'name' parameter is used instead.

    cd  $GOPATH/src/github.com/openblockchain/obc-peer
 	./obc-peer chaincode deploy -n mycc -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

Alternatively, you can run the chaincode deploy transaction through the REST API.

```
POST localhost:5000/devops/deploy

{
  "type": "GOLANG",
  "chaincodeID":{
      "name":"mycc"
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

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the enrollmentID of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). On the CLI the enrollmentID is passed with the -u parameter and on the REST API the enrollmentID is passed with the 'secureContext' element.

 	  ./obc-peer chaincode deploy -u jim -n mycc -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

```
POST localhost:5000/devops/deploy

{
  "type": "GOLANG",
  "chaincodeID":{
      "name":"mycc"
  },
  "ctorMsg": {
      "function":"init",
      "args":["a", "100", "b", "200"]
  },
  "secureContext": "jim"
}
```

The deploy transaction initializes the chaincode by executing a target initializing function. Though the example shows “init”, the name could be arbitrarily chosen by the chaincode developer.  You should see the following output in the chaincode window:

	2015/11/15 15:19:31 Received INIT(uuid:005dea42-d57f-4983-803e-3232e551bf61), initializing chaincode
	Aval = 100, Bval = 200

#### Chaincode invoke via CLI and REST

Run the chaincode invoking transaction on the CLI as many times as desired:

	./obc-peer chaincode invoke -l golang -n mycc -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

Run the chaincode invoke transaction through the REST API:

```
POST localhost:5000/devops/invoke

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

200 OK
{
    “Status”: “OK. Successfully invoked transaction.”
}
```

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the enrollmentID of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). On the CLI the enrollmentID is passed with the -u parameter and on the REST API the enrollmentID is passed with the 'secureContext' element.

	 ./obc-peer chaincode invoke -u jim -l golang -n mycc -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

```
POST localhost:5000/devops/invoke

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

The invoke transaction runs the specified transaction name “invoke” with the arguments. This transaction transfers 10 units from A to B. You should see the following output in the chaincode window:

	2015/11/15 15:39:11 Received RESPONSE. Payload 200, Uuid 075d72a4-4d1f-4a1d-a735-4f6f60d597a9
	Aval = 90, Bval = 210

#### Chaincode query via CLI and REST

Run a query on the chaincode to retrieve desired values. The “-n” arg should match that provided in the Chaincode Window.

    ./obc-peer chaincode query -l golang -n mycc -c '{"Function": "query", "Args": ["b"]}'

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
          "name":"mycc"
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

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the enrollmentID of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). On the CLI the enrollmentID is passed with the -u parameter and on the REST API the enrollmentID is passed with the 'secureContext' element.

    ./obc-peer chaincode query -u jim -l golang -n mycc -c '{"Function": "query", "Args": ["b"]}'

```
POST localhost:5000/devops/query

{
  "chaincodeSpec":{
      "type": "GOLANG",
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg":{
          "function":"query",
          "args":["a"]
      },
  	  "secureContext": "jim"
  }
}
```

#### Removing temporary files when security is enabled

After the completion of a given chaincode test with security enabled, it is helpful to remove the temporary files created by the CA server runtime. To remove client enrollment certificate, enrollment key, transaction certificate chain, etc. execute the commands below. You must execute these commands if you wish to register a user that has already been registered before.

    rm -rf /var/openchain/production/

As well as:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca
    rm -rf .obcca
