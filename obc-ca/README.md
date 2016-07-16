# OBC Certificate Authority

The OBC Certificate Authority (OBC-CA) consists of three main services:

1. The Enrollment Certificate Authority (ECA) which provides the enrollment service. Any node participating to OBC 
has to enroll before being able to become an active part of the system. 
2. The Transaction Certificate Authority (TCA) which provides the transaction certificates used to anonymize the 
identity of the node.
3. The TLS Certificate Authority (TLSCA) which provides TLS certificates used to establish TLS connections between 
the different nodes forming the OBC network.

In the current version, these three services will be offered by the same server.

For more information, read Section 4.2 of the [protocol-spec](https://github.com/openblockchain/obc-docs/blob/master/protocol-spec.md)

Please, refer also to this tutorial [SandboxSetup.md](https://github.com/openblockchain/obc-docs/blob/master/api/SandboxSetup.md) to learn how to write, build, and run chaincodes in a sandbox. The tutorial guides the developer through the process  of setting up a wallet service (Section 4.3.2 and 4.5 of the [protocol-spec](https://github.com/openblockchain/obc-docs/blob/master/protocol-spec.md)).

## Bootstrap OBC-CA

The bootstrap the OBC Certificate Authority the following steps are needed:

1. Configure the Server;
2. Configure the ECA service;
3. Launch the Server.

The OBC-CA configuration file is **obcca.yaml** and is included in the obc-ca folder. 


### Configure the Server

The first step is to configure the server part.

Here is an example of configuration:
```
server:
        # Protocol version. "0.1" is the only option right now.
        version: "0.1"
        # the path where to store OBC-CA state folder
        rootpath: "/var/openchain/production"
        # The name of the folder that will contain the OBC-CA state           
        cadir: "obc-ca"
        # The port on which the services will reposond.
        port: ":50051"
```

#### TLS Connection

Notice that the server accepts only TLS connections with **server-side authentication only**.
Before being able to establish a TLS connection to the server, a node has to receive, through an off-band channel or 
pre-installed, the TLSCA certificate. This allows the node to perform the server-side authentication.

The TLSCA certificate is generated during boostratp by the TLS certificate authority. 
The file is called **tlsca.cert** and is stored under **rootpath/cadir**. 

### Configure the ECA Service

For **testing** purposes only, the ECA can be instructed to preload a set of enrollment tuples.
An enrollment tuple consists of the following enrollment values:

1. ID;
2. Role (client/peer/validator);
3. Password.

Each tuple represents a node that has already registered to the Registration Authority. Therefore, that node is 
entitled for enrolling. To enroll, only the enrollment pair <ID, Password> is required.  This pair must be distributed 
to the node through an off-band channel. 
Notice that an enrollment pair can be used only once.

Here is an example of configuration:

```
    eca:
        users:
                # <ID>: <Role> <Password>
                alice: 1 NPKYL39uKbkj
                bob: 2 DRJ23pEQl16a
                charlie: 4 7avZQLwcUe9q
```

Note that the integer that precedes the Password indicates the role of the user per  [ca.proto](https://github.com/angrbrd/obc-peer/blob/master/obc-ca/protos/ca.proto)

```
enum Role {
    NONE = 0;
    CLIENT = 1; // powers of 2 to | different roles
    PEER = 2;
    VALIDATOR = 4;
}
```

Finally, the enrollment ID must be unique among all IDs.

### Launch the Server

The following commands builds and runs the server with the configuration defined in **obcca.yaml**.

    cd $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca
    go build -o obcca-server
    ./obcca-server

