# bin

This directory contains several executable scripts and a
[Makefile](Makefile). You may find it useful to put this directory in your
**PATH**. 

Many simple "scripts" are actually implemented as make targets, and are
documeted at the top of the Makefile. Make targets are provided for various
types of "cleaning", building Docker images, testing, starting Hyperledger
fabric networks and obtaining logs from containers. Some of the make targets
are similar to those found in the fabric/Makefile, however are defined with a
slightly different sensibility.

The following major support scripts are provided. Scripts are typically
documented in a *usage* string embedded at the top of the script.

* [checkAgreement](checkAgreement) is a script that uses the **fabricLogger**
  (see below) to check that all of the peers in a network agree on the
  contents of the blockchain.

* [fabricLogger](fabricLogger) is a simple script that converts a blockchain
  into a text file listing of transaction IDs.
  
* [userModeNetwork](userModeNetwork) is a script defines Hyperledger fabric
  peer networks as simple networks of user-mode processes, that is, networks
  that do not rely on Docker-compose or multiple physical nodes. The concept
  of a user-mode network is discussed [below](#userModeNetwork).
  
* [wait-for-it](wait-for-it) is a script that waits for a network service to
  be available with an optional timeout. **busywork** applications use
  `wait-for-it` to sequence network startup correctly.
  
<a name="userModeNetwork"></a>
## User-Mode Networks

A *user-mode network* is a Hyperledger fabric peer network in which the peers
in the network are simple user-mode processes. Running a user-mode network
does not require *root* privledges, or depend on Docker or any other
virtualization technology ([caveats](#caveats)). The script that sets up these
networks, [userModeNetwork](userModeNetwork), configures the peers such that
their network service interfaces, databases and logging directories are
unique. A description of the [network configuration](#network) is written to
`$BUSYWORK_HOME/network`, and the **busywork** tools refer to this configuration
by default in order to examine and control the network. The databases and
logging directories are stored in the [$BUSYWORK_HOME](BUSYWORK_HOME.md)
directory.

Starting a user-mode network of 10 peers running PBFT batch consensus is as as
simple as

    userModeNetwork 10
	
Several useful options for setting up the network are also provided. Here is
an example of setting up a 4-node network with security, PBFT Sieve consensus
and DEBUG-level logging in the peers

    userModeNetwork -security -sieve -peerLogging debug 4
	
<a name="caveats"></a>
## User-Mode Caveats

Docker is still currently required for chaincode containers, so currently
users *are* still required to have *root* access and/or sudo-less permission
to control Docker. One goal of this project is to completely eliminate the
*requirement* for Docker from the development and test environment however,
and allow networks to be reliably set up and tested by normal users sharing a
development server.

User-mode networks are not secure. Any user on the host machine can access or
disrupt the network by targeting the service ports. We don't see this to be a
major issue for typical development and test environments however.


The mechanism used to determine when the **membersrvc** server is up and
running causes a (harmless) message like the following to appear at the top of
the log file:

    2016/05/19 08:23:40 grpc: Server.Serve failed to create ServerTransport:  connection error: desc = "transport: write tcp 127.0.0.1:50051->127.0.0.1:49084: write: broken pipe"


<a name=busywork-home></a>
## BUSYWORK_HOME

If **BUSYWORK_HOME** is not defined in the environment, **busywork** scripts
and applications create (if necessary) and use `~/.busywork` as the
**BUSYWORK_HOME** directory. The following directories and files will be found
in the **BUSYWORK_HOME** depending on which **busywork** tools are being
used.

* `fabricLog` The is a text representation of the blockchain created by the
  [fabricLogger](fabricLogger) process that driver scripts invoke to validate
  transaction execution.
  
* `membersrvc/` This directory contains the **membersrvc** database (`/data` -
  TBD) and the `/stderr` and `/stdout` logs from the **membersrvc** server.
  
* `network` The network configuraation is described [below](#network).

* `vp[0,...N]/` These directories contain the validating peer databases
  `/data` and their `/stderr` and `stdout` logs.
  
* `*.chain` These files are created by the [checkAgreement](checkAgreement)
  script, and represent the blockchains as recorded on the different peers in
  the network.

<a name="network"></a>
## *busywork* Network Configurations

**busywork** scripts that create peer networks produce a description of the
  network for the benefit of other **busywork** tools. The network description
  is written as `$BUSYWORK_HOME/network`, and contains a single JSON
  object. This is still a work in progress so some fields may be changed
  and/or added in the future, but this is an example of what one of these
  network descriptions looks like at present.
  
```
{
    "networkMode": "user",
    "chaincodeMode": "vm",
    "host": "local",
    "date": "2016/05/17+18:53:07",
    "createdBy": "userModeNetwork",
    "security": "true",
    "consensus": "sieve",
    "membersrvc":
        { "service": "0.0.0.0:50051",
          "profile": "0.0.0.0:44971",
          "pid": "62328"
        },
    "peers": [
        { "id": "vp0",
          "grpc": "0.0.0.0:35547",
          "rest": "0.0.0.0:36097",
          "events": "0.0.0.0:33355",
          "profile": "0.0.0.0:35510",
          "pid": "62358"
        }
        ,
        { "id": "vp1",
          "grpc": "0.0.0.0:35540",
          "rest": "0.0.0.0:40712",
          "events": "0.0.0.0:41232",
          "profile": "0.0.0.0:37337",
          "pid": "62362"
        }
        ,
        { "id": "vp2",
          "grpc": "0.0.0.0:44523",
          "rest": "0.0.0.0:39215",
          "events": "0.0.0.0:42639",
          "profile": "0.0.0.0:40144",
          "pid": "62370"
        }
        ,
        { "id": "vp3",
          "grpc": "0.0.0.0:42380",
          "rest": "0.0.0.0:35422",
          "events": "0.0.0.0:42112",
          "profile": "0.0.0.0:42939",
          "pid": "62378"
        }
    ]
}
```
