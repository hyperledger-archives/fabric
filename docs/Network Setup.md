# Openchain - Setting Up a Network For Development

## Overview

This document covers setting up an Openchain network on your local machine for development.

All commands should be run from within the Vagrant environment created in the instructions for [setting up a development environment](https://github.com/openblockchain/obc-getting-started/blob/master/devenv.md).

There are three types of peers we will start:

- Non-validating peer - This peer provides an API that allows clients to send transactions to the Openchain network.
- Validating peer - A validating peer receives transactions from either a non-valdating peer or a client. A validating peer also runs chaincodes and participates in consensus.
- Leader - A leader organizes transactions into blocks that are distributed to validating peers for consensus.

The communication flow is:

Client -> Non-Validating Peer -> Validating Peer -> Leader

## Starting peers

Peers should be started in the following order:

1. Leader
2. Validating peer
3. Non-validating peer

Each peer can be launched in its own terminal. Remember, these commands should be run from your Vagrant environment.

### Launching the leader

Run the following command:
```
docker run --rm -it -e OPENCHAIN_PEER_ID=leader -e OPENCHAIN_VM_ENDPOINT=http://172.17.42.1:4243 -e OPENCHAIN_PEER_CONSENSUS_VALIDATOR_ENABLED=true -e OPENCHAIN_PEER_CONSENSUS_LEADER_ENABLED=true openchain-peer obc-peer peer
```

### Launching the validating peer

1. Find the IP address of your leader by running the following command:
```
docker inspect `docker ps -aq`  | grep IPAddress
```
The top IP address should be for your most recent container which is the leader you just launched.

2. Start the validator with the following command replacing <IP address> with the IP address from step 1.
```
docker run --rm -it -e OPENCHAIN_PEER_ID=validator_01 -e OPENCHAIN_VM_ENDPOINT=http://172.17.42.1:4243 -e OPENCHAIN_PEER_CONSENSUS_VALIDATOR_ENABLED=true -e OPENCHAIN_PEER_CONSENSUS_LEADER_ADDRESS=<IP address>:30303 openchain-peer obc-peer peer
```

The validator is now launched and connected to the leader.

### Launching the non-validating peer

1. Find the IP address of your validating peer by running the following command:
```
docker inspect `docker ps -aq`  | grep IPAddress
```
The top IP address should be for your most recent container which is the validating you just launched.

2 Start the non-validating peer with the following command replacing <IP address> with the IP address from step 1.
```
OPENCHAIN_PEER_LOGGING_LEVEL=DEBUG OPENCHAIN_PEER_DISCOVERY_ROOTNODE=<IP address>:30303  ./obc-peer peer
```

The non-validating peer is now launched and connected to the validator.

## Deploying the sample chaincode

To test your network, run the following command to deploy the sample chaincode
```
./obc-peer chaincode deploy --path=github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_simple --version=0.1.0
```
You should see CONSENSUS print in the terminal of your leader and validating peer.
