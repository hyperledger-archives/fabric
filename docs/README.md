
## Getting started

Welcome to the Linux Foundation Hyperledger Project documentation README. This page contains: 
- Getting started doc links 
- Quickstart doc links
- Table of Contents links to the complete library

If you are new to the project, you can begin by reviewing the following documents:
- [Whitepaper WG](https://github.com/hyperledger/hyperledger/wiki/Whitepaper-WG)
- [Requirements WG](https://github.com/hyperledger/hyperledger/wiki/Requirements-WG)
- [Glossary](glossary.md): to understand the terminology that we use throughout the website and project.

When you are ready to start building applications or to otherwise contribute to the project, we strongly recommend that you read our [protocol specification](protocol-spec.md) for the technical details. Procedurally, we use the agile methodology with a weekly sprint, organized by [issues](https://github.com/hyperledger/fabric/issues), so take a look to familiarize yourself with the current work.

## Quickstart documentation
In addition to the <b>Getting started</b> documentation, the following quickstart topics are available:
- [Fabric FAQs](FAQ)
- [Canonical use cases](biz/usecases.md)
- [Development environment set-up](dev-setup/devenv.md)
- [Chaincode development environment](API/SandboxSetup.md)
- [APIs](API/CoreAPI.md)
- [Network setup](dev-setup/devnet-setup.md)

## Table of Contents

This table of contents provides links to the complete documentation library: <br>
Overview docs; Reference docs; API and chaincode developer docs; Network operations docs; Security administration docs; Use cases and demos

### Overview docs:

- [Hyperledger project](https://github.com/hyperledger/hyperledger)
- [Whitepaper](https://github.com/hyperledger/hyperledger/wiki/Whitepaper-WG)
- [Fabric README](../README.md)
- [Glossary](glossary.md) 
- [Figures & Diagrams](/docs/images/) 
- [Protocol specification](protocol-spec.md):
     - Introduction (fabric, terminology)
     - Fabric: 
          - Architecture (Membership Services, Blockchain Services, Chaincode Services, Events, API, CLI)
          - Topology (SVP, MVP, Multichain)
          - Protocol (Messages, Ledger, Chaincode, Consensus, Events)
          - Security (Business Security, User Security, Transaction Security, App. ACL, Wallet, Network Security (TLS))
          - PBFT (Core PBFT, PI, Sieve)
          - API (REST service, REST API, CLI)
          - Application Model (Composition, Sample)
          - Future (Systems Integration, Performance & Scalability, Consensus Plugins, Additional Languages)

### Reference docs:

- [Contributing](../CONTRIBUTING.md)
- [Communicating](../README.md#communication-)
- [Project Lifecycle](https://github.com/hyperledger/hyperledger/wiki/Project-Lifecycle)
- [License](../LICENSE)
- [Maintainers](../MAINTAINERS.txt)

### API and chaincode developer docs:

- [Setting up the development environment](dev-setup/devenv.md): 
     - Overview (Vagrant/Docker) 
     - Prerequisites (Git, Go, Vagrant, VirtualBox, BIOS)
     - Steps (GOPATH, Windows, Clone Peer, VM Vagrant
- [Building the fabric core](dev-setup/install.md#building-the-fabric-core-)
- [Building outside of Vagrant](dev-setup/install.md#building-outside-of-vagrant-)
- [Code contributions](../CONTRIBUTING.md)
- [Coding Golang](dev-setup/install.md#coding-golang-)
- [Headers](dev-setup/headers.txt)
- [Writing Chaincode](dev-setup/install.md#writing-chaincode-)
- [Writing, Building, and Running Chaincode in a Development Environment](API/SandboxSetup.md)
- [Chaincode FAQ](FAQ/chaincode_FAQ.md)
- [Setting Up a Network](dev-setup/install.md#setting-up-a-network-)
- [Setting Up a Network For Development](dev-setup/devnet-setup.md):
     - Docker
     - Validating Peers
     - Run chaincode
     - Consensus Plugin
- [Working with CLI, REST, and Node.js](dev-setup/install.md#working-with-cli-rest-and-nodejs-)
- [APIs - CLI, REST, and Node.js](API/CoreAPI.md): 
     - [CLI](API/CoreAPI.md#cli)
     - [REST](API/CoreAPI.md#rest-api)
     - [Node.js Application](API/CoreAPI.md#nodejs-application)
- [Configuration](dev-setup/install.md#configuration-)
- [Logging](dev-setup/install.md#logging-)
- [Logging control](dev-setup/logging-control.md): 
     - Overview 
     - Peer
     - Go 
- [Generating grpc code](dev-setup/install.md#generating-grpc-code-)
- [Adding or updating Go packages](dev-setup/install.md#adding-or-updating-go-packages-)
- [SDK](wiki-images)

### Network operations docs:

- [Setting Up a Network](dev-setup/install.md#setting-up-a-network-)
- [Consensus Algorithms](FAQ/consensus_FAQ.md)

### Security administration docs:

- [Certificate Authority (CA) Setup](dev-setup/ca-setup.md):
     - Enrollment CA
     - Transaction CA
     - TLS CA
     - Configuration
     - Build and Run <br> 
- [Application ACL](tech/application-ACL.md):
     - Fabric Support
     - Certificate Handler
     - Transaction Handler
     - Client
     - Transaction Format
     - Validators
     - Deploy Transaction
     - Execute Transaction
     - Chaincode Execution
