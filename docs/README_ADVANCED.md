# Openchain - Advanced Concepts

## Overview

This document covers the advanced development techniques associated with Peer development.  Make sure you have read [the root level README](../README.md) to assure proper development environment setup.

## Running a single validator node.

### Launch the Validator node

To start your validator node, issue the following command.

>docker run --rm -it -e OPENCHAIN_PEER_ID=validator_01 -e OPENCHAIN_VM_ENDPOINT=http://172.17.42.1:4243 openchain-peer obc-peer peer

The above command will launch a new Peer in validator mode which will be connected to by your local Peer in a later step.

Get the IPAddress of the running peer container using the following command:

>docker inspect `docker ps -aq`  | grep IPAddress

Note the IPAddress value which for now on will be referred to as VALIDATOR_IPADDRESS.

### Launch your local Peer and connect to Validator

To start your local Peer and connect it to the validator node created above, issue the following command (Replaced VALIDATOR_IPADDRESS with the value you noted earlier):

> OPENCHAIN_PEER_DISCOVERY_ROOTNODE=<VALIDATOR_IPADDRESS>:30303 ./obc-peer peer

### Deploying a contract to your validator

