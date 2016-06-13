#Writing, Building, and Running Chaincode in a Development Environment

Chaincode developers need a way to test and debug their chaincode without having to set up a complete peer network. This document describes how to write, build, and test chaincode in a local development environment.

Multiple terminal windows inside the Vagrant development environment are required. One Vagrant terminal runs the validating peer; another Vagrant terminal runs the chaincode; the third Vagrant terminal runs the CLI or REST API commands to execute transactions. When running with security enabled, an additional fourth Vagrant terminal window is required to run the <b>Certificate Authority (CA)</b> server. Detailed instructions are provided in the sections below:

* [Security Setup (optional)](#security-setup-optional)
* [Vagrant Terminal 1 (validating peer)](#vagrant-terminal-1-validating-peer)
* [Vagrant Terminal 2 (chaincode)](#vagrant-terminal-2-chaincode)
* [Vagrant Terminal 3 (CLI or REST API)](#vagrant-terminal-3-cli-or-rest-api)
    * [Chaincode deploy via CLI and REST](#chaincode-deploy-via-cli-and-rest)
    * [Chaincode invoke via CLI and REST](#chaincode-invoke-via-cli-and-rest)
    * [Chaincode query via CLI and REST](#chaincode-query-via-cli-and-rest)
* [Removing temporary files when security is enabled](#removing-temporary-files-when-security-is-enabled)

See the [logging control](../dev-setup/logging-control.md) reference for information on controlling
logging output from the `peer` and chaincodes.

###Security Setup (optional)
From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

To set up the local development environment with security enabled, you must first build and run the <b>Certificate Authority (CA)</b> server:

    cd $GOPATH/src/github.com/hyperledger/fabric
    make membersrvc && membersrvc

Running the above commands builds and runs the CA server with the default setup, which is defined in the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml) configuration file. The default configuration includes multiple users who are already registered with the CA; these users are listed in the `eca.users` section of the configuration file. To register additional users with the CA for testing, modify the `eca.users` section of the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml) file to include additional `enrollmentID` and `enrollmentPW` pairs. Note the integer that precedes the `enrollmentPW`. That integer indicates the role of the user, where 1 = client, 2 = non-validating peer, 4 = validating peer, and 8 = auditor.

###Vagrant Terminal 1 (validating peer)
**Note:** To run with security enabled, first modify the [core.yaml](https://github.com/hyperledger/fabric/blob/master/peer/core.yaml) configuration file to set the `security.enabled` value to `true` before building the peer executable. Alternatively, you can enable security by running the peer with environment variable `CORE_SECURITY_ENABLED=true`. To enable privacy and confidentiality of transactions (requires security to also be enabled), modify the [core.yaml](https://github.com/hyperledger/fabric/blob/master/peer/core.yaml) configuration file to set the `security.privacy` value to `true` as well. Alternatively, you can enable privacy by running the peer with environment variable `CORE_SECURITY_PRIVACY=true`. If you are enabling security and privacy on the peer process with environment variables, it is important to include these environment variables in the command when executing all subsequent peer operations (e.g. deploy, invoke, or query).

From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

Build and run the peer process to enable security and privacy after setting `security.enabled` and `security.privacy` settings to `true`.

    cd $GOPATH/src/github.com/hyperledger/fabric
    make peer
    peer node start --peer-chaincodedev

Alternatively, enable security and privacy on the peer with environment variables:

    CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer node start --peer-chaincodedev

###Vagrant Terminal 2 (chaincode)

From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

Build the <b>chaincode_example02</b> code, which is provided in the source code repository:

    cd $GOPATH/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02
    go build

When you are ready to start creating your own chaincode, create a new subdirectory inside of /fabric/examples/go/chaincode to store your chaincode files. You can copy the <b>chaincode_example02</b> file to the new directory and modify it.

Run the following chaincode command to start and register the chaincode with the validating peer (started in Vagrant terminal 1):

    CORE_CHAINCODE_ID_NAME=mycc CORE_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02

The chaincode console will display the message "Received REGISTERED, ready for invocations", which indicates that the chaincode is ready to receive requests. Follow the steps below to send a chaincode deploy, invoke or query transaction. If the "Received REGISTERED" message is not displayed, then an error has occurred during the deployment; revisit the previous steps to resolve the issue.

###Vagrant Terminal 3 (CLI or REST API)

#### **Note on REST API port**

The default REST interface port is 5000. It can be configured in [core.yaml](https://github.com/hyperledger/fabric/blob/master/peer/core.yaml) using the `rest.address` property. If using Vagrant, the REST port mapping is defined in [Vagrantfile](https://github.com/hyperledger/fabric/blob/master/devenv/Vagrantfile).

#### **Note on security functionality**

Current security implementation assumes that end user authentication takes place at the application layer and is not handled by the fabric. Authentication may be performed through any means considered appropriate for the target application. Upon successful user authentication, the application will perform user registration with the CA exactly once. If registration is attempted a second time for the same user, an error will result. During registration, the application sends a request to the certificate authority to verify the user registration and if successful, the CA responds with the user certificates and keys. The enrollment and transaction certificates received from the CA will be stored locally inside `/var/hyperledger/production/crypto/client/` directory. This directory resides on a specific peer node which allows the user to transact only through this specific peer while using the stored crypto material. If the end user needs to perform transactions through more then one peer node, the application is responsible for replicating the crypto material to other peer nodes. This is necessary as registering a given user with the CA a second time will fail.

With security enabled, the CLI commands and REST payloads must be modified to include the `enrollmentID` of a registered user who is logged in; otherwise an error will result. A registered user can be logged in through the CLI or the REST API by following the instructions below. To log in through the CLI, issue the following commands, where `username` is one of the `enrollmentID` values listed in the `eca.users` section of the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml) file.

From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

Register the user though the CLI, substituting for `<username>` appropriately:

    cd $GOPATH/src/github.com/hyperledger/fabric/peer
    peer network login <username>

The command will prompt for a password, which must match the `enrollmentPW` listed for the target user in the `eca.users` section of the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml) file. If the password entered does not match the `enrollmentPW`, an error will result.

To log in through the REST API, send a POST request to the `/registrar` endpoint, containing the `enrollmentID` and `enrollmentPW` listed in the `eca.users` section of the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml) file.

<b>REST Request:</b>
```
POST localhost:5000/registrar

{
  "enrollId": "jim",
  "enrollSecret": "6avZQLwcUe9b"
}
```

<b>REST Response:</b>
```
200 OK
{
    "OK": "Login successful for user 'jim'."
}
```

#### Chaincode deploy via CLI and REST

First, send a chaincode deploy transaction, only once, to the validating peer. The CLI connects to the validating peer using the properties defined in the core.yaml file. **Note:** The deploy transaction typically requires a `path` parameter to locate, build, and deploy the chaincode. However, because these instructions are specific to local development mode and the chaincode is deployed manually, the `name` parameter is used instead.
```
peer chaincode deploy -n mycc -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
```

Alternatively, you can run the chaincode deploy transaction through the REST API.

<b>REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "deploy",
  "params": {
    "type": 1,
    "chaincodeID":{
        "name": "mycc"
    },
    "ctorMsg": {
        "function":"init",
        "args":["a", "100", "b", "200"]
    }
  },
  "id": 1
}
```

<b>REST Response:</b>
```
{
    "jsonrpc": "2.0",
    "result": {
        "status": "OK",
        "message": "mycc"
    },
    "id": 1
}
```

**Note:** When security is enabled, modify the CLI command and the REST API payload to pass the `enrollmentID` of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#note-on-security-functionality). On the CLI, the `enrollmentID` is passed with the `-u` parameter; in the REST API, the `enrollmentID` is passed with the `secureContext` element. If you are enabling security and privacy on the peer process with environment variables, it is important to include these environment variables in the command when executing all subsequent peer operations (e.g. deploy, invoke, or query).

 	  CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode deploy -u jim -n mycc -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

<b>REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "deploy",
  "params": {
    "type": 1,
    "chaincodeID":{
        "name": "mycc"
    },
    "ctorMsg": {
        "function":"init",
        "args":["a", "100", "b", "200"]
    },
    "secureContext": "jim"
  },
  "id": 1
}
```

The deploy transaction initializes the chaincode by executing a target initializing function. Though the example shows "init", the name could be arbitrarily chosen by the chaincode developer. You should see the following output in the chaincode window:

	2015/11/15 15:19:31 Received INIT(uuid:005dea42-d57f-4983-803e-3232e551bf61), initializing chaincode
	Aval = 100, Bval = 200

#### Chaincode invoke via CLI and REST

Run the chaincode invoking transaction on the CLI as many times as desired. The `-n` argument should match the value provided in the chaincode window (started in Vagrant terminal 2):

	peer chaincode invoke -l golang -n mycc -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

Alternatively, run the chaincode invoking transaction through the REST API.

<b>REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "invoke",
  "params": {
      "type": 1,
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg": {
         "function":"invoke",
         "args":["a", "b", "10"]
      }
  },
  "id": 3
}
```

<b>REST Response:</b>
```
{
    "jsonrpc": "2.0",
    "result": {
        "status": "OK",
        "message": "5a4540e5-902b-422d-a6ab-e70ab36a2e6d"
    },
    "id": 3
}
```

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the `enrollmentID` of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#note-on-security-functionality). On the CLI, the `enrollmentID` is passed with the `-u` parameter; in the REST API, the `enrollmentID` is passed with the `secureContext` element. If you are enabling security and privacy on the peer process with environment variables, it is important to include these environment variables in the command when executing all subsequent peer operations (e.g. deploy, invoke, or query).

 	  CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode invoke -u jim -l golang -n mycc -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'

<b> REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "invoke",
  "params": {
      "type": 1,
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg": {
         "function":"invoke",
         "args":["a", "b", "10"]
      },
      "secureContext": "jim"
  },
  "id": 3
}
```

The invoking transaction runs the specified chaincode function name "invoke" with the arguments. This transaction transfers 10 units from A to B. You should see the following output in the chaincode window:

	2015/11/15 15:39:11 Received RESPONSE. Payload 200, Uuid 075d72a4-4d1f-4a1d-a735-4f6f60d597a9
	Aval = 90, Bval = 210

#### Chaincode query via CLI and REST

Run a query on the chaincode to retrieve the desired values. The `-n` argument should match the value provided in the chaincode window (started in Vagrant terminal 2):

    peer chaincode query -l golang -n mycc -c '{"Function": "query", "Args": ["b"]}'

The response should be similar to the following:

    {"Name":"b","Amount":"210"}

If a name other than "a" or "b" is provided in a query sent to `chaincode_example02`, you should see an error response similar to the following:

    {"Error":"Nil amount for c"}

Alternatively, run the chaincode query transaction through the REST API.

<b> REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "query",
  "params": {
      "type": 1,
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg": {
         "function":"query",
         "args":["a"]
      }
  },
  "id": 5
}
```

<b>REST Response:</b>
```
{
    "jsonrpc": "2.0",
    "result": {
        "status": "OK",
        "message": "90"
    },
    "id": 5
}
```

**Note:** When security is enabled, modify the CLI command and REST API payload to pass the `enrollmentID` of a logged in user. To log in a registered user through the CLI or the REST API, follow the instructions in the [note on security functionality](#note-on-security-functionality). On the CLI, the `enrollmentID` is passed with the `-u` parameter; in the REST API, the `enrollmentID` is passed with the `secureContext` element. If you are enabling security and privacy on the peer process with environment variables, it is important to include these environment variables in the command when executing all subsequent peer operations (e.g. deploy, invoke, or query).

 	  CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode query -u jim -l golang -n mycc -c '{"Function": "query", "Args": ["b"]}'

<b>REST Request:</b>
```
POST host:port/chaincode

{
  "jsonrpc": "2.0",
  "method": "query",
  "params": {
      "type": 1,
      "chaincodeID":{
          "name":"mycc"
      },
      "ctorMsg": {
         "function":"query",
         "args":["a"]
      },
      "secureContext": "jim"
  },
  "id": 5
}
```

#### Removing temporary files when security is enabled

After the completion of a chaincode test with security enabled, remove the temporary files that were created by the CA server process. To remove the client enrollment certificate, enrollment key, transaction certificate chain, etc., run the following commands. Note, that you must run these commands if you want to register a user who has already been registered previously.

From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

And then run:

    rm -rf /var/hyperledger/production
