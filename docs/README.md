
## Getting started

Welcome to the fabric documentation README. This page contains: <br>
1) Getting started doc links; 2) Quickstart doc links; and 3) Table of Contents links to the complete library.

If you are new to the Linux Foundation Hyperledger Project, please start by reading the  [whitepaper](https://github.com/hyperledger/hyperledger/wiki/Whitepaper-WG). In addition, we encourage you to review our [glossary](glossary.md) to understand the terminology that we use throughout the website and project.

When you are ready to start building applications or to otherwise contribute to the project, we strongly recommend that you read our [protocol specification](protocol-spec.md) for the technical details. Procedurally, we use the agile methodology with a weekly sprint, organized by [issues](https://github.com/hyperledger/fabric/issues), so take a look to familiarize yourself with the current work.

## Quickstart documentation
In addition to the <b>Getting started</b> documentation, the following quickstart topics are available:
- [Fabric FAQs](FAQ)
- [Canonical use cases](biz/usecases.md)
- [Development environment set-up](dev-setup/devenv.md)
- [Chain code development environment](API/SandboxSetup.md)
- [APIs](API/CoreAPI.md)
- [Network setup](dev-setup/devnet-setup.md)
- [Technical implementation details](https://github.com/hyperledger/fabric/tree/master/docs/tech)

## Table of Contents

This table of contents provides links to the complete documentation library: <br>
Overview docs; Reference docs; API and chaincode developer docs; Network operations docs; Security administration docs; Use cases and demos; 

### Overview docs:

- [Hyperledger project](https://github.com/hyperledger/hyperledger)
- [Whitepaper](https://github.com/hyperledger/hyperledger/wiki/Whitepaper-WG)
- [Fabric README](../README.md)
- [Protocol specification](protocol-spec.md)
- [Glossary](glossary.md) 
- [Figures & Diagrams](/docs/images/) 

### Reference docs:

- [Contributing](CONTRIBUTING.md)
- [Communicating](../README.md#communication-)
- [Project Lifecycle](https://github.com/hyperledger/hyperledger/wiki/Project-Lifecycle)
- [License](LICENSE)
- [Maintainers](MAINTAINERS.md)

### API and chaincode developer docs:

- [Setting up the development environment](dev-setup/devenv.md): <br>
     Overview (Vagrant/Docker) <br>
     Prerequisites (Git, Go, Vagrant, VirtualBox, BIOS) <br>
     Steps (GOPATH, Windows, Clone Peer, VM Vagrant <br>
- [Building the fabric core](../README.md#building-the-fabric-core-)
- [Building outside of Vagrant](../README.md#building-outside-of-vagrant-)
- [Code contributions](../README.md#code-contributions-)
- [Coding Golang](../README.md#coding-golang-)
- [Headers](dev-setup/headers.txt)
- [Writing Chaincode](../README.md#writing-chaincode-)
- [Writing, Building, and Running Chaincode in a Development Environment](API/SandboxSetup.md)
- [Chaincode FAQ](FAQ/chaincode_FAQ.md)
- [Setting Up a Network](../README.md#setting-up-a-network-)
- [Setting Up a Network For Development](dev-setup/devnet-setup.md): <br>
     - Docker <br>
          Validating Peers <br>
          Run chaincode <br>
          Consensus Plugin <br>
- [Working with CLI, REST, and Node.js](../README.md#working-with-cli-rest-and-nodejs-)
- [APIs - CLI, REST, and Node.js](../API/CoreAPI.md): 
     - [CLI](API/CoreAPI.md#cli)
     - [REST](API/CoreAPI.md#rest-api)
     - [Node.js Application](API/CoreAPI.md#nodejs-application)
- [Configuration](../README.md#configuration-)
- [Logging](../README.md#logging-)
- [Logging control](../README.md#dev-setup/logging-control.md): 
     - Overview 
     - Peer
     - Go 
- [Generating grpc code](../README.md#generating-grpc-code-)
- [Adding or updating Go packages](../README.md#adding-or-updating-go-packages-)
- [SDK](wiki-images)

### Network operations docs:

- [Setting Up a Network](../README.md#setting-up-a-network-)
- [Consensus Algorithms](FAQ/consensus_FAQ.md)

### Security administration docs:

- [Certificate Authority (CA) Setup](dev-setup/obcca-setup.md): <br>
     Enrollment CA <br>
     Transaction CA <br>
     TLS CA <br>
     Configuration <br>
     Build and Run <br> 
- [Application ACL](tech/application-ACL.md):
     Fabric Support <br>
     Certificate Handler <br>
     Transaction Handler <br>
     Client <br>
     Transaction Format <br>
     Validators <br>
     Deploy Transaction <br>
     Execute Transaction <br>
     Chaincode Execution <br>

### Use cases and demos:
- [Canonical Use Cases](/biz/usecases.md) <br>
     B2B Contract <br>
     Manufacturing Supply Chain <br> 
     Asset Depository <br>
- [Extended Use Cases](/biz/usecases.md) <br>
     One Trade, One Contract <br>
     Direct Communication <br>
     Separation of Asset Ownership and Custodian's Duties <br>
     Interoperability of Assets <br>
- [Marbles Demo Application](https://github.com/IBM-Blockchain/marbles )
- [Commercial Paper Demo Application](https://github.com/IBM-Blockchain/cp-web )

### Protocol specification: 
[Protocol Specification](protocol-spec.md) 
- Introduction (fabric, terminology)
- Fabric: 
     - Architecture (Membership Services, Blockchain Services, Chaincode Services, Events, API, CLI)
     - Topology (SVP, MVP, Multichain)
     - Protocol (Messages, Ledger, Chaincode, Consensus, Events)
     - Security (Business Security, User Security, Transaction Security, App. ACL, Wallet, Network Security (TLS))
     - PBFT (Core PBFT, PI, Sieve,)
     - API (REST service, REST API, CLI)
     - Application Model (Composition, Sample)
     - Future (Systems Integration, Performance & Scalability, Consensus Plugins, Additional Languages)

