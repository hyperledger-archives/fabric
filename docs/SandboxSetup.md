#Writing, Building, and Running Chaincode in a Development Environment

Chaincode developers need a way to test and debug their chaincode without having to set up a complete Openchain network. This document contains instructions on how to write, build, and test chaincode in a local development environment.

Multiple terminal windows are required: one terminal runs the validating peer; another terminal runs the chaincode; the third terminal runs the CLI or REST API commands to execute transactions. When running with security enabled, an additional fourth terminal window is required to run the <b>Certificate Authority (CA)</b> server. Detailed instructions are provided in the sections below:

* [Security Setup (optional)](#security-setup-optional)
* [Window 1 (validating peer)](#window-1-validating-peer)
* [Window 2 (chaincode)](#window-2-chaincode)
* [Window 3 (CLI or REST API)](#window-3-cli-or-rest-api)
    * [Chaincode deploy via CLI and REST](#chaincode-deploy-via-cli-and-rest)
    * [Chaincode invoke via CLI and REST](#chaincode-invoke-via-cli-and-rest)
    * [Chaincode query via CLI and REST](#chaincode-query-via-cli-and-rest)
* [Removing temporary files when security is enabled](#removing-temporary-files-when-security-is-enabled)

###Security Setup (optional)
To set up the local development environment with security enabled, you must first build and run the CA server:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca
    go build -o obcca-server
    ./obcca-server

Running the above commands builds and runs the CA server with the default setup, which is defined in the  [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) configuration file. The default configuration includes multiple users who are already registered with the CA; these users are listed in the 'users' section of the configuration file. To register additional users with the CA for testing, modify the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) file to add enrollmentID and enrollmentPW pairs.

###Window 1 (validating peer)
**Note:** To run with security enabled, first modify the [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml) configuration file to set the <b>security.enabled</b> value to 'true' before building the peer executable. Alternatively, you can enable security by running the peer with OPENCHAIN_SECURITY_ENABLED=true.

Build and run the peer process to enable security:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    go build
    ./obc-peer peer --peer-chaincodedev   

Alternatively, enable security on the peer:

    OPENCHAIN_SECURITY_ENABLED=true ./obc-peer peer --peer-chaincodedev

###Window 2 (chaincode)
Build the <b>chaincode_example02</b> code, which is provided in the source code repository:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02
    go build

When you are ready to start creating your own chaincode, create a new subdirectory in /openchain/example/chaincode to store your chaincode files. You can copy the <b>chaincode_example02</b> file to the new directory and modify it.

Run the following chaincode command to start and register the chaincode with the validating peer (started in window 1):

    OPENCHAIN_CHAINCODE_ID_NAME=mycc OPENCHAIN_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02

The chaincode console will display the message "Received REGISTERED, ready for invocations", which indicates that the chaincode is ready to receive requests. Follow the steps below to send a chaincode deploy, invoke or query transaction. If the "Received REGISTERED" message is not displayed, then an error has occurred during the deployment; revisit the previous steps to resolve the issue.

###Window 3 (CLI or REST API)

**Note on REST API port**

The Openchain REST interface port is defined as port 5000 in the [openchain.yaml](https://github.com/openblockchain/obc-peer/blob/master/openchain.yaml) configuration file. If you are sending REST requests to the peer node from the same machine, use port 5000. If you are sending REST requests through Swagger, then the port specified in the Swagger file is port 3000. The different ports emphasizes that Swagger will likely not run on the same machine as the peer process, or outside Vagrant. To send requests from the Swagger-UI or Swagger-Editor interface, set up port forwarding from port 3000 to port 5000 on your machine, or edit the Swagger configuration file to specify the port number of your choice.

**Note on security functionality**

When security is enabled, the CLI commands and REST payloads must be modified to include the <b>enrollmentID</b> of a registered user who is logged in; otherwise an error will result. A registered user can be logged in through the CLI or the REST API by following the instructions below. Any user who is registered with the CA can register their enrollmentID and enrollmentPW only once; if registration is attempted a second time, an error will result. This process requires that the authentication process takes place at the application layer, with the application handling the user registration with the CA on behalf of an authenticated user exactly once, while storing the enrollment certificate.

To log in through the CLI, issue the following commands, where 'username' is one of the <b>enrollmentID</b> values listed in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) file:

    cd $GOPATH/src/github.com/openblockchain/obc-peer
    ./obc-peer login <username>

The command will prompt for a password, which must match the <b>enrollmentPW</b> listed for the target user in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) file. If the password entered does not match the <b>enrollmentPW</b>, an error will result.

To log in through the REST API, send a POST request to the <b>/registrar<b> endpoint, containing the <b>enrollmentID</b> and <b>enrollmentPW</b> listed in the 'users' section of the [obcca.yaml](https://github.com/openblockchain/obc-peer/blob/master/obc-ca/obcca.yaml) file.

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

First, send a chaincode deploy transaction, only once, to the validating peer. The CLI connects to the validating peer using the properties defined in the openchain.yaml file. **Note:** The deploy transaction typically requires a 'path' parameter to locate, build and deploy the chaincode. However, because these instructions are for local development mode, where the chaincode is deployed manually, the 'name' parameter is used instead.

    cd  $GOPATH/src/github.com/openblockchain/obc-peer
 	./obc-peer chaincode deploy -n mycc -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

Alternatively, you can run the chaincode deploy transaction through the REST API:

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

**Note:** When security is enabled, modify the CLI command and the REST API payload to pass the <b>enrollmentID</b> of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). In the CLI, the <b>enrollmentID</b> is passed with the <b>-u</b> parameter; in the REST API, the <b>enrollmentID</b> is passed with the </b>'secureContext'</b> element.

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

The deploy transaction initializes the chaincode by executing a target initializing function. Though the example shows "init", the name could be arbitrarily chosen by the chaincode developer. You should see the following output in the chaincode window:

	2015/11/15 15:19:31 Received INIT(uuid:005dea42-d57f-4983-803e-3232e551bf61), initializing chaincode
	Aval = 100, Bval = 200

#### Chaincode invoked via CLI and REST

Run the chaincode invoking transaction on the CLI as many times as desired:

	./obc-peer chaincode invoke -l golang -n mycc -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

Alternatively, run the chaincode invoke transaction through the REST API:

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

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the <b>enrollmentID</b> of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). In the CLI, the <b>enrollmentID</b> is passed with the <b>-u</b> parameter; in the REST API, the <b>enrollmentID</b> is passed with the </b>'secureContext'</b> element.

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

The invoke transaction runs the specified transaction name "invoke" with the arguments. This transaction transfers 10 units from A to B. You should see the following output in the chaincode window:

	2015/11/15 15:39:11 Received RESPONSE. Payload 200, Uuid 075d72a4-4d1f-4a1d-a735-4f6f60d597a9
	Aval = 90, Bval = 210

#### Chaincode query via CLI and REST

Run a query on the chaincode to retrieve the desired values. The <b>-n</b> argument should match the value provided in the chaincode window:

    ./obc-peer chaincode query -l golang -n mycc -c '{"Function": "query", "Args": ["b"]}'

The response should be similar to the following:

    {"Name":"b","Amount":"210"}

If a name other than "a" or "b" is provided in a query sent to <b>chaincode_example02</b>, you should see an error response similar to the following:

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

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the <b>enrollmentID</b> of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#window-3-cli-or-rest-api). In the CLI, the <b>enrollmentID</b> is passed with the <b>-u</v> parameter; in the REST API, the <b>enrollmentID</b> is passed with the <b>'secureContext'</b> element:

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

After the completion of a chaincode test with security enabled, remove the temporary files that were created by the CA server runtime process. To remove the client enrollment certificate, enrollment key, transaction certificate chain, etc., run the following commands. Note that you must run these commands if you want to register a user who has already been registered previously:

    rm -rf /var/openchain/production/

And then run:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca
    rm -rf .obcca
