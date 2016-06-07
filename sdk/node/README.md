Hyperledger Client SDK for Node.js
=========

The Hyperledger Client (HLC) provides a powerful and easy to use API to interact with a Hyperledger blockchain.

The sections in this document are as follows:

* The [Getting Started](#Getting Started) section is intended to help you quickly get a feel for HLC, how to use it, and some of it's common capabilities.  This is demonstrated by example.

* The [Getting Setup](#Getting Setup) section shows you how to setup up your environment and to run the unit tests.  Looking at the unit tests will also help you learn more of the APIs by example, including asset management and confidentiality.

* The [Going Deeper](#Going Deeper) section discusses HLC's pluggability or extensibility design.  It also describes the main object hierarchy to help you get started in navigating the [reference documentation](doc/modules/_hlc_.html).  The top-level class is [Chain](doc/classes/_hlc_.chain.html).

   WARNING: To view the reference documentation correctly, you may need to open the following URLs directly in your browser.  Be sure to replace YOUR-FABRIC-DIR with the path to your fabric directory.

   `file:///YOUR-FABRIC-DIR/sdk/node/doc/modules/_hlc_.html`

   `file:///YOUR-FABRIC-DIR/sdk/node/doc/classes/_hlc_.chain.html`

* The [Looking Ahead](#Looking Ahead) section describes some future work to be done.

<a name="Getting Started">
## Getting Started
</a>

This purpose of this section is to help you quickly get a feel for HLC and how you may use it.  It is not intended to demonstrate all of it's power, but to demonstrate a common use case by example.

### Some basic terminology

First, there is some basic terminology you should understand.  In order to transact on a hyperledger blockchain, you must first have an identity which has been both **registered** and **enrolled**.

Think of **registration** as *issuing a user invitation* to join a blockchain.  It consists of adding a user name (also called an *enrollment ID*).  This can be done programatically with the **Member.register** method, or by adding the enrollment ID to the member services configuration file in `fabric/membersrvc/membersrvc.yaml`.

Think of **enrollment** as *accepting a user invitation* to join a blockchain.  This is always done by the entity that will transact on the blockchain.  This can be done programatically via the **Member.enroll** method.

### Learn by example

The best way to quickly learn HLC is by example.

The following example demonstrates a typical web app.  The web app authenticates a user and then transacts on a blockchain on behalf of that user.

```
/**
 * This example shows how to do the following in a web app.
 * 1) At initialization time, enroll the web app with the block chain.
 *    The identity must have already been registered.
 * 2) At run time, after a user has authenticated with the web app:
 *    a) register and enroll an identity for the user;
 *    b) use this identity to deploy, query, and invoke a chaincode.
 */

// To include the package from your hyperledger fabric directory:
//    var hlc = require("myFabricDir/sdk/node");
// To include the package from npm:
//      var hlc = require('hlc');
var hlc = require('hlc');

// Create a client chain.
// The name can be anything as it is only used internally.
var chain = hlc.newChain("targetChain");

// Configure the KeyValStore which is used to store sensitive keys
// as so it is important to secure this storage.
// The FileKeyValStore is a simple file-based KeyValStore, but you
// can easily implement your own to store whereever you want.
chain.setKeyValStore( hlc.newFileKeyValStore('/tmp/keyValStore') );

// Set the URL for member services
chain.setMemberServicesUrl("grpc://localhost:50051");

// Add a peer's URL
chain.addPeer("grpc://localhost:30303");

// Enroll "WebAppAdmin" which is already registered because it is
// listed in fabric/membersrvc/membersrvc.yaml with it's one time password.
// If "WebAppAdmin" has already been registered, this will still succeed
// because it stores the state in the KeyValStore
// (i.e. in '/tmp/keyValStore' in this sample).
chain.enroll("WebAppAdmin", "DJY27pEnl16d", function(err, webAppAdmin) {
   if (err) return console.log("ERROR: failed to register %s: %s",err);
   // Successfully enrolled WebAppAdmin during initialization.
   // Set this user as the chain's registrar which is authorized to register other users.
   chain.setRegistrar(webAppAdmin);
   // Now begin listening for web app requests
   listenForUserRequests();
});

// Main web app function to listen for and handle requests
function listenForUserRequests() {
   for (;;) {
      // WebApp-specific logic goes here to await the next request.
      // ...
      // Assume that we received a request from an authenticated user
      // 'userName', and determined that we need to invoke the chaincode
      // with 'chaincodeID' and function named 'fcn' with arguments 'args'.
      handleUserRequest(userName,chaincodeID,fcn,args);
   }
}

// Handle a user request
function handleUserRequest(userName, chaincodeID, fcn, args) {
   // Register and enroll this user.
   // If this user has already been registered and/or enrolled, this will
   // still succeed because the state is kept in the KeyValStore
   // (i.e. in '/tmp/keyValStore' in this sample).
   var registrationRequest = {
        enrollmentID: userName,
        // Customize account & affiliation
        account: "bank_a",
        affiliation: "00001"
   };
   chain.registerAndEnroll( registrationRequest, function(err, user) {
      if (err) return console.log("ERROR: %s",err);
      // Issue an invoke request
      var invokeRequest = {
        // Name (hash) required for invoke
        chaincodeID: chaincodeID,
        // Function to trigger
        fcn: fcn,
        // Parameters for the invoke function
        args: args
     };
     // Invoke the request from the user object.
     var tx = user.invoke(invokeRequest);
     // Listen for the 'submitted' event
     tx.on('submitted', function(results) {
        console.log("submitted invoke: %j",results);
     });
     // Listen for the 'complete' event.
     tx.on('complete', function(results) {
        console.log("completed invoke: %j",results;
     });
     // Listen for the 'error' event.
     tx.on('error', function(err) {
        console.log("error on invoke: %j",err);
     });
   });
}

```

<a name="Getting Set Up">
## Getting Set Up
</a>

### Building the client SDK

If you set the **FABRIC** environment variable to point to your `hyperledger/fabric` directory, you can build the client SDK as follows:

```
cd $FABRIC && make node-sdk
```

The node package is then located in the `$FABRIC/sdk/node` directory.


### Running the unit tests

HLC includes unit tests implemented with the [tape framework](https://github.com/substack/tape).


#### Setting up the environment

We assume the sandbox setting is used as described here: [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode).

Build and run the Membership Service (Certificate Authority) as described [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#security-setup-optional).

We also assume that the peer is running at security level 256, which is the default value.

Don't forget to enable security and privacy as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode).


#### registrar

This test case exercises registering users with member services.  It also tests registering a registrar which can then register other users.

Run the test as follows assuming membership services is running on the default ports:


```
cd $FABRIC/sdk/node
node test/unit/registrar.js
```

#### chain-tests

This test case exercises chaincode *chaincode_example02* as described in in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode).

In particular, you must first start the chaincode as follows:


```
cd $FABRIC/examples/chaincode/go/chaincode_example02
go build
CORE_CHAINCODE_ID_NAME=mycc CORE_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02
```

And run chain-tests as follows:


```
cd $FABRIC/sdk/node
node test/unit/chain-tests.js
```

To activate TLS connection with the member services the following actions are needed:

- Modify *$FABRIC/membersrvc/membersrvc.yaml* as follows:

``` 
server:
     tls:
        certfile: "/var/hyperledger/production/.membersrvc/tlsca.cert"
        keyfile: "/var/hyperledger/production/.membersrvc/tlsca.priv"
```
This is needed to instruct the member services on which tls cert and key to use.  

- Modify *$FABRIC/peer/core.yaml* as follows:

``` 
peer:
    pki:
        tls:
            enabled: true
            rootcert:
                file: "/var/hyperledger/production/.membersrvc/tlsca.cert"
```
This is needed to allow the peer to connect to the member services using TLS, otherwise the connection will fail.

- Bootstrap your member services and the peer. This is needed in order to have the file *tlsca.cert* generated by the member services

- Copy */var/hyperledger/production/.membersrvc/tlsca.cert* to *$FABRIC/sdk/node*.

At this point *chain-tests.js* will load automatically the *tlsca.cert* file and connect to the member services using TLS.

N.B. If you cleanup the folder */var/hyperledger/production* then don't forget to copy again the *tlsca.cert* file as described above.


#### asset-mgmt

This test case exercises chaincode *asset_management*. When running the chaincode as described in [SanboxSetup.md](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-2-chaincode), name it *assetmgmt* as this is the name used in unit tests.

In particular, you must first start the chaincode as follows:


```
cd $FABRIC/examples/chaincode/go/asset_management
go build
CORE_CHAINCODE_ID_NAME=assetmgmt CORE_PEER_ADDRESS=0.0.0.0:30303 ./asset_management
```


And run the asset management tests as follows:

```
cd $FABRIC/sdk/node
node test/unit/asset-mgmt.js
```


#### Troublingshooting

1. If the tests fail and you see errors regarding port forwarding, similar to the one below, that implies that you do not have correct port forwarding enabled in Vagrant.

    ```
    tcp_client_posix.c:173] failed to connect to 'ipv6:[::1]:50051': socket error: connection refused
    ```

   To address this, make sure your Vagrant setup has port forwarding enabled for port 50051 as the tests connect to the membership services on that port. Check your [Vagrantfile](https://github.com/hyperledger/fabric/blob/master/devenv/Vagrantfile) to confirm that the following line is present. If not, modify your Vagrantfile to include it, then issue the command `vagrant reload`.

    ```
    config.vm.network :forwarded_port, guest: 50051, host: 50051 # Membership service
    ```

1. If you see errors stating that the client has already been registered/enrolled, keep in mind that you can perform the enrollment process only once, as the enrollmentSecret is a one-time-use password. You will see these errors if you have performed a user registration/enrollment and subsequently deleted the crypto tokens stored on the client side. The next time you try to enroll, errors similar to the ones below will be seen.

   ```
   Error: identity or token do not match
   ```
   ```
   Error: user is already registered
   ```

   To address this, remove any stored crypto material from the CA server by following the instructions [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#removing-temporary-files-when-security-is-enabled). You will also need to remove any of the crypto tokens stored on the client side by deleting the KeyValStore directory. That directory is configurable and is set to `/tmp/keyValStore` within the unit tests.

<a name="Going Deeper">
## Going Deeper
</a>

#### Pluggability

HLC was designed to support two pluggable components:

1. Pluggable key value store which is used to retrieve and store keys associated with a member.  The key value store is used to store sensitive private keys, so care must be taken to properly protect access.

2. Pluggable member service which is used to register and enroll members.  Member services enables hyperledger to be a permissioned blockchain, providing security services such as anonymity, unlinkability of transactions, and confidentiality

#### HLC objects and reference documentation

HLC is written primarily in typescript and is object-oriented.  The source can be found in the `fabric/sdk/node/src` directory.

To go deeper, you can view the reference documentation in your browser by opening the [reference documentation](doc/modules/_hlc_.html) and clicking on **"hlc"** on the right-hand side under **"Globals"**.

The following is a high-level description of the HLC objects (classes and interfaces) to help guide you through the object hierarchy.

* The main top-level class is [Chain](doc/classes/_hlc_.chain.html). It is the client's representation of a chain.  HLC allows you to interact with multiple chains and to share a single [KeyValStore](doc/interfaces/_hlc_.keyvalstore.html) and [MemberServices](doc/interfaces/_hlc_.memberservices.html) object with multiple Chain objects as needed.  For each chain, you add one or more [Peer](doc/classes/_hlc_.peer.html) objects which represents the endpoint(s) to which HLC connects to transact on the chain.

* The [KeyValStore](doc/interfaces/_hlc_.keyvalstore.html) is a very simple interface which HLC uses to store and retrieve all persistent data.  This data includes private keys, so it is very important to keep this storage secure.  The default implementation is a simple file-based version found in the [FileKeyValStore](doc/classes/_hlc_.filekeyvalstore.html) class.

* The [MemberServices](doc/interfaces/_hlc_.memberservices.html) interface is implemented by the [MemberServicesImpl](doc/classes/_hlc_.memberservicesimpl.html) class and provides security and identity related features such as privacy, unlinkability, and confidentiality.  This implementation issues *ECerts* (enrollment certificates) and *TCerts* (transaction certificates).  ECerts are for enrollment identity and TCerts are for transactions.

* The [Member](doc/classes/_hlc_.member.html) class most often represents an end user who transacts on the chain, but it may also represent other types of members such as peers.  From the Member class, you can *register* and *enroll* members or users.  This interacts with the [MemberServices](doc/interfaces/_hlc_.memberservices.html) object.  You can also deploy, query, and invoke chaincode directly, which interacts with the [Peer](doc/classes/_hlc_.peer.html).  The implementation for deploy, query and invoke simply creates a temporary [TransactionContext](doc/classes/_hlc_.transactioncontext.html) object and delegates the work to it.

* The [TransactionContext](doc/classes/_hlc_.transactioncontext.html) class implements the bulk of the deploy, invoke, and query logic.  It interacts with MemberServices to get a TCert to perform these operations.  Note that there is a one-to-one relationship between TCert and TransactionContext; in other words, a single TransactionContext will always use the same TCert.  If you want to issue multiple transactions with the same TCert, then you can get a [TransactionContext](doc/classes/_hlc_.transactioncontext.html) object from a [Member](doc/classes/_hlc_.member.html) object directly and issue multiple deploy, invoke, or query operations on it.  Note however that if you do this, these transactions are linkable, which means someone could tell that they came from the same user, though not know which user.  For this reason, you will typically just call deploy, invoke, and query on the User or Member object.

<a name="Looking Ahead">
## Looking Ahead
</a>

The following is a list of known remaining work to be done.

1. Publish **hlc** to the npm registry or publish a link into the hyperledger github.  Until then, you may use the following in your node program: `var hlc = require("YOUR-FABRIC-DIRECTORY/sdk/node");`

1. Complete **TransactionContext.deploy** to work in network mode.  As you can see from the unit tests, it currently requires running in dev mode.

1. Implement events appropriately, both custom and non-custom.  The 'complete' event for `deploy` and `invoke` is currently implemented by simply waiting 5 seconds.  It needs to receive a complete event from the server with the result of the transaction and make this available to the caller.

1. Support SHA2.  HLC currently supports SHA3.
