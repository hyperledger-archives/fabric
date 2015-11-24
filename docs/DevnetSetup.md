## Setting Up a Network For Development

This document covers setting up an Openchain network on your local machine for development using Docker container.

All commands should be run from within the Vagrant environment created in the instructions for [setting up a development environment](https://github.com/openblockchain/obc-getting-started/blob/master/devenv.md).

### Setting up an Openchain Docker image
First clean out any old Openchain images in docker (using docker ps and docker rmi commands) and build a new one:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/container
    go test -run BuildImage_Peer

### Starting up validating peers
Find out which IP address your docker0 interface is on with *ip add* command. For example, *inet 172.17.0.1/16 scope global docker0*. Use that IP address for the OPENCHAIN_VM_ENDPOINT option. The id value of OPENCHAIN_PEER_ID must be lowercase since we use the id as part of chaincode containers we build and docker does not accept uppercase. The id must also be unique for each validating peer.

We are also using the default consensus NOOPS (which doesn't really do consensus). If you want to use some other consensus plugin, see note 1 below.

Start up the first validating peer:

```
docker run --rm -it -e OPENCHAIN_VM_ENDPOINT=http://172.17.0.1:4243 -e OPENCHAIN_PEER_ID=vp1 openchain-peer obc-peer peer dev
```
You can ignore the error *ERRO 005 Error creating connection to peer address=:  grpc: target is unspecified* since it is the first peer and doesn't connect to any other peers.

Start up the second validating peer: We need to get the IP address of the first validating peer, which will act as the root node that the new peer will connect to. The address is shown on the terminal window of the first peer (eg 172.17.0.2)

```
docker run --rm -it -e OPENCHAIN_VM_ENDPOINT=http://172.17.0.1:4243 -e OPENCHAIN_PEER_ID=vp2 -e OPENCHAIN_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 openchain-peer obc-peer peer dev
```

You can start up a few more validating peer in the similar manner as you wish.

### Deploying the sample chaincode
We can use the example chaincode to test the network.

```
cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02
go build
```

Now deploy the chaincode to the network. We can deploy to any validating peer by specifying OPENCHAIN_PEER_ADDRESS:

```
OPENCHAIN_PEER_ADDRESS=172.17.0.2:30303 ./obc-peer chaincode deploy -p github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02 -v 0.0.1 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
```

You can watch for the message "Received build request for chaincode spec" on the output screen of all validating peers.

We can run a transaction:

```
OPENCHAIN_PEER_ADDRESS=172.17.0.2:30303 ./obc-peer chaincode invoke -l golang -p github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02 -v 0.0.1 -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'
```
