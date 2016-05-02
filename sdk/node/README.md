# Hyperledger Client SDK for Node.js

The Hyperledger Client SDK (HLC) provides APIs through which a client may easily interact with the Hyperledger blockchain from a Node.js application.

## Terminology

To understand the implementation details inside [hlc.js](./hlc.js) please read and understand the terminology used below.

* Member

  An identity for participating in blockchain transactions. There are different types of members: users, peers, etc.

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

To run the tests follow the instructions below.

1. Build and run the Membership Service (CA) as described [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md#security-setup-optional).

2. Enable the security and privacy on the peer. To do so, modify the [core.yaml](https://github.com/hyperledger/fabric/blob/master/peer/core.yaml) configuration file to set the <b>security.enabled</b> value to 'true' and <b>security.privacy</b> value to 'true'. Subsequently, build and run the peer process with the following commands.

```
    cd $GOPATH/src/github.com/hyperledger/fabric/peer
    go build
    ./peer peer  
```

3. Switch to the HCL directory and run the unit tests with the following commands.

```
    cd $GOPATH/src/github.com/hyperledger/fabric/sdk/node
    node test/unit/chain-tests.js | node_modules/.bin/tap-spec
```

If the first tests fail and you see an error similar to the one below, that implies that you do not have correct port forwarding enabled in Vagrant.

```
    tcp_client_posix.c:173] failed to connect to 'ipv6:[::1]:50051': socket error: connection refused
```

To address this, make sure your Vagrant setup has port forwarding enabled for port 50051 as the tests connect to the membership services on that port. Check your Vagrant file to confirm that the following line is present. If not, modify your Vagrant file to include it, then issue the command `vagrant reload`.

```
    config.vm.network :forwarded_port, guest: 50051, host: 50051 # Membership service
```
