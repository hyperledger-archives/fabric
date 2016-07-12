## Hyperledger Fabric Documentation
The Hyperledger [fabric](https://github.com/hyperledger/fabric) is an implementation of blockchain technology, that has been collaboratively developed under the Linux Foundation's [Hyperledger Project](http://hyperledger.org). It leverages familiar and proven technologies, and offers a modular architecture that allows pluggable implementations of various function including membership services, consensus, and smart contracts (chaincode) execution. It features powerful container technology to host any mainstream language for smart contracts development.

Below, you'll find the following sections:
- [Getting started](#getting-started)
- [Quickstart](#quickstart-documentation)
- Developer guides
  - [Fabric developer guide](#fabric-developer-guide)
  - [Chaincode developer guide](#chaincode-developer-guide)
  - [API developer guide](#api-developer-guide)
- [Operations guide](#operations-guide)
- [Security administration guides](#security-administration-guides)

## Getting started

If you are new to the project, you can begin by reviewing the following links. If you'd prefer to dive right in, see the [Quickstart](#quickstart-documentation) section, below.
- [Whitepaper WG](https://github.com/hyperledger/hyperledger/wiki/Whitepaper-WG): where the community is developing a whitepaper to explain the motivation and goals for the project.
- [Requirements WG](https://github.com/hyperledger/hyperledger/wiki/Requirements-WG): where the community is developing use cases and requirements.
- [Canonical use cases](biz/usecases.md)
- [Glossary](glossary.md): to understand the terminology that we use throughout the Fabric project's documentation.
- [Fabric FAQs](https://github.com/hyperledger/fabric/tree/master/docs/FAQ)
- [Consensus Algorithms FAQ](FAQ/consensus_FAQ.md)

## Quickstart documentation

- [Development environment set-up](dev-setup/devenv.md): if you are considering helping with development of the Hyperledger Fabric or Fabric-API projects themselves, this guide will help you install and configure all you'll need. The development environment is also useful (but, not necessary) for developing blockchain applications and/or chaincode.
- [Network setup](dev-setup/devnet-setup.md): This document covers setting up a network on your local machine for development.
- [Chaincode development environment](API/SandboxSetup.md): Chaincode developers need a way to test and debug their chaincode without having to set up a complete peer network. This document describes how to write, build, and test chaincode in a local development environment.
- [APIs](API/CoreAPI.md): This document covers the available APIs for interacting with a peer node.


## Fabric developer guide

When you are ready to start building applications or to otherwise contribute to the project, we strongly recommend that you read our [protocol specification](protocol-spec.md) for the technical details. Procedurally, we use the agile methodology with a weekly sprint, organized by [issues](https://github.com/hyperledger/fabric/issues), so take a look to familiarize yourself with the current work.

- [Making code contributions](https://github.com/hyperledger/fabric/blob/master/CONTRIBUTING.md): First, you'll want to familiarize yourself with the project's contribution guidelines.
- [Setting up the development environment](dev-setup/devenv.md): after that, you will want to set up your development environment.
- [Building the fabric core](dev-setup/install.md#building-the-fabric-core-): next, try building the project in your local development environment to ensure that everything is set up correctly.
- [Building outside of Vagrant](dev-setup/install.md#building-outside-of-vagrant-): for the adventurous, you might try to build outside of the standard Vagrant development environment.
- [Logging control](dev-setup/logging-control.md): describes how to tweak the logging levels of various components within the fabric.
- [Generating grpc code](dev-setup/install.md#generating-grpc-code-): we use gRPC for interprocess communications. If you add or modify any of the protobuf specifications, you'll want to know how to generate the gRPC code.
- [Adding or updating Go packages](dev-setup/install.md#adding-or-updating-go-packages-): if you need to add or remove dependencies, you will need to follow this process.
- [Coding in Go](dev-setup/install.md#coding-golang-): Some tips for developing with Go.
- [License header](dev-setup/headers.txt): every source file must include this license header and copyright statement for the principle author(s).

## Chaincode developer guide:

- [Setting up the development environment](dev-setup/devenv.md): when developing and testing chaincode, or an application that leverages the fabric API or SDK, you'll probably want to run the fabric locally on your laptop to test. You can use the same setup that Fabric developers use.
- [Setting Up a Network For Development](dev-setup/devnet-setup.md): alternately, you can follow these instructions for setting up a local network for chaincode development.
- [Writing, Building, and Running Chaincode in a Development Environment](API/SandboxSetup.md): a step-by-step guide to writing and testing chaincode.
- [Chaincode FAQ](FAQ/chaincode_FAQ.md): a FAQ for all of your burning questions relating to chaincode.

## API developer guide

- [APIs - CLI, REST, and Node.js](API/CoreAPI.md)
     - [CLI](API/CoreAPI.md#cli): working with the command-line interface.
     - [REST](API/CoreAPI.md#rest-api): working with the REST API.
     - [Node.js SDK](../sdk/node/README.md): working with the Node.js SDK.

## Operations guide:

- [Setting Up a Network](dev-setup/devnet-setup.md): instructions for setting up a Fabric network of peers.

## Security administration guides:

- [Certificate Authority (CA) Setup](dev-setup/ca-setup.md): setting up a CA.
- [Application ACL](tech/application-ACL.md): working with access control lists.Â
