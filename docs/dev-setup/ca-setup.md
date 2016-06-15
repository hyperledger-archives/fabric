# Certificate Authority (CA) Setup

## Overview

The _Certificate Authority_ (CA) provides a number of certificate services to users of a blockchain.  More specifically, these services relate to _user enrollment_, _transactions_ deployed or invoked on the blockchain, and _TLS_-secured connections between users or components of the blockchain.

### Enrollment Certificate Authority

The _enrollment certificate authority_ (ECA) is the place where new users are initially registered with the blockchain and where those users can then request an _enrollment certificate pair_.  One certificate is for data signing, one is for data encryption.  The public keys to be embedded in the certificates have to be of type ECDSA, whereby the key for data encryption is then converted by the user to be used in an [ECIES](https://en.wikipedia.org/wiki/Integrated_Encryption_Scheme) (Elliptic Curve Integrated Encryption System) fashion.

### Transaction Certificate Authority

Once a user is enrolled, he or she can request _transaction certificates_ from the _transaction certificate authority_ (TCA) to be used for deployment and invocation transactions on the blockchain.  Although a single transaction certificate can be used for multiple transactions, for privacy reasons it is recommended to use a new transaction certificate for each transaction.

### TLS Certificate Authority

In addition to enrollment and transaction certificates, users of the blockchain need _TLS certificates_ for TLS-securing their communication channels.  TLS certificates can be requested from the _TLS certificate authority_ (TLSCA).

## Configuration

All CA services are provided by a single process, which can be configured to some extent by setting parameters in the CA configuration file membersrvc.yaml, which is located in root of the same directory as the CA binary.  More specifically, the following parameters can be set:

- `server.gomaxprocs`: limits the number of operating system threads used by the CA.
- `server.rootpath`: the root path of the directory where the CA stores its state.
- `server.cadir`: the name of the directory where the CA stores its state.
- `server.port`: the port at which all CA services listen (multiplexing of services over the same port is provided by [GRPC](http://www.grpc.io)).

Furthermore, logging levels can be enabled/disabled by adjusting the following settings:

- `logging.trace` (off by default, useful for debugging the code only)
- `logging.info`
- `logging.warning`
- `logging.error`
- `logging.panic`

Alternatively, these fields can be set via environment variables, which---if set---have precedence over entries in the yaml file.  The corresponding environment variables are named as follows:

    MEMBERSRVC_CA_SERVER_GOMAXPROCS
    MEMBERSRVC_CA_SERVER_ROOTPATH
    MEMBERSRVC_CA_SERVER_CADIR
    MEMBERSRVC_CA_SERVER_PORT

In addition, the CA may be preloaded with registered users, where each user's name, roles, and password are specified:

    eca:
    	users:
    		alice: 2 DRJ20pEql15a
    		bob: 4 7avZQLwcUe9q

The role value is simply a bitmask of the following:

    CLIENT = 1;
    PEER = 2;
    VALIDATOR = 4;
    AUDITOR = 8;

For example, a peer who is also a validator would have a role value of 6.

When the CA is started for the first time, it will generate all of its required states (e.g., internal databases, CA certificates, blockchain keys, etc.) and writes this state to the directory given in its configuration.  The certificates for the CA services (i.e., for the ECA, TCA, and TLSCA) are self-signed as the current default.  If those certificates shall be signed by some root CA, this can be done manually by using the `*.priv` and `*.pub` private and public keys in the CA state directory, and replacing the self-signed `*.cert` certificates with root-signed ones..  The next time the CA is launched, it will read and use those root-signed certificates.

## Build and Run

The CA can be built with the following command executed in the `membersrvc` directory:

    cd $GOPATH/src/github.com/hyperledger/fabric
    make membersrvc

The CA can be started with the following command:

    build/bin/membersrvc

The CA looks for an `membersrvc.yaml` configuration file in $GOPATH/src/github.com/hyperledger/fabric/membersrvc.  If the CA is started for the first time, it creates all its required state (e.g., internal databases, CA certificates, blockchain keys, etc.) and write each state to the directory given in the CA configuration.
