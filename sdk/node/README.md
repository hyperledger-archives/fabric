# Hyperledger Client SDK for Node.js

The Hyperledger Client SDK (HLC) provides APIs through which a client may easily interact with the Hyperledger blockchain from a Node.js application.

## Terminology

To understand the implementation details inside [hlc.js](./hlc.js) please read and understand the terminology below.

* Member

  An identity for participating in blockchain transactions. There are different types of members: users, peers, validators, etc.

* Member Services

  Services related to registering, enrolling, and otherwise managing members.

* Registration

   The act of adding a new member identity (with specific privileges) to the system. This is done by an administrator, or more accurately, a member (called a registrar) with the 'registrar' privilege. The registrar specifies the new member privileges when registering the new member. The output of registration is an enrollment secret (i.e. a one-time password).

* Enrollment

  The act of completing registration and obtaining credentials which are used to transact on the blockchain. Enrollment may be done by the end user, in which case only the end user has access to his/her credentials. Alternatively, the registrar may have delegated authority to act on behalf of the end user, in which case the registrar also performs enrollment for the user.

## Pluggability

These APIs have been designed to support two pluggable components.

1. Pluggable key value store which is used to retrieve and store keys associated with a member.

2. Pluggable member service which is used to register and enroll members.

## Unit Tests

The HLC SDK includes unit tests implemented with the [tape framework](https://github.com/substack/tape) and output prettified with [tap-spec](https://github.com/scottcorgan/tap-spec).

To run the unit tests follow the instructions below.

1. Build and run the Membership Service (Certificate Authority) as described [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#security-setup-optional).

2. Enable the security and privacy on the peer. To do so, modify the [core.yaml](https://github.com/hyperledger/fabric/blob/master/peer/core.yaml) configuration file to set the <b>security.enabled</b> value to 'true' and <b>security.privacy</b> value to 'true'. Subsequently, build and run the peer process with the following commands. Alternatively, you may start the peer by setting the appropriate environment variables as described [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#vagrant-terminal-1-validating-peer).

    ```
    cd $GOPATH/src/github.com/hyperledger/fabric/peer
    go build
    ./peer peer  
    ```

3. Switch to the HCL directory and install the necessary Node.js module dependencies.

    ```
    cd $GOPATH/src/github.com/hyperledger/fabric/sdk/node
    npm install
    ```

4. Run the Node.js unit tests with the following commands.

    ```
    cd $GOPATH/src/github.com/hyperledger/fabric/sdk/node
    node test/unit/chain-tests.js | node_modules/.bin/tap-spec
    ```

5. If the tests fail and you see errors regarding port forwarding, similar to the one below, that implies that you do not have correct port forwarding enabled in Vagrant.

    ```
    tcp_client_posix.c:173] failed to connect to 'ipv6:[::1]:50051': socket error: connection refused
    ```

To address this, make sure your Vagrant setup has port forwarding enabled for port 50051 as the tests connect to the membership services on that port. Check your [Vagrantfile](https://github.com/hyperledger/fabric/blob/master/devenv/Vagrantfile) to confirm that the following line is present. If not, modify your Vagrantfile to include it, then issue the command `vagrant reload`.

    ```
    config.vm.network :forwarded_port, guest: 50051, host: 50051 # Membership service
    ```

If you see errors stating that the client has already been registered/enrolled, keep in mind that you can perform the enrollment process only once, as the enrollmentSecret is a one-time-use password. You will see these errors if you have performed a user registration/enrollment and subsequently deleted the crypto tokens stored on the client side. The next time you try to enroll, errors similar to the ones below will be seen.

    Error: identity or token do not match

or

    Error: user is already registered

To address this, remove any stored crypto material from the CA server by following the instructions [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#removing-temporary-files-when-security-is-enabled). If you are running a clean test, you will also want to remove any of the crypto tokens stored on the client side by deleting the KeyValStore directory. That directory is configurable, and is set to `/tmp/keyValStore` within the unit tests.
