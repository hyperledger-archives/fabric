## Setting Up a Network For Development

This document covers setting up an Openchain network on your local machine for development using Docker container.

All commands should be run from within the Vagrant environment created in the instructions for [setting up a development environment](https://github.com/openblockchain/obc-getting-started/blob/master/devenv.md).

### Setting up an Openchain Docker image
First clean out any active Openchain containers (peer and chaincode) using "docker ps -a" and "docker rm" commands. Second, remove any old Openchain images with "docker images" and "docker rmi" commands.

Now we are ready to build a new Openchain docker image:

    cd $GOPATH/src/github.com/openblockchain/obc-peer/openchain/container
    go test -run BuildImage_Peer

Check the available images, and you should see "openchain-peer" image.


### Starting up validating peers
Find out which IP address your docker0 interface is on with *ip add* command. For example, your output might contain something like *inet 172.17.0.1/16 scope global docker0*. That means docker0 interface is on IP address 172.17.0.1. Use that IP address for the OPENCHAIN_VM_ENDPOINT option. 

The ID value of OPENCHAIN_PEER_ID must be lowercase since we use the ID as part of chaincode containers we build, and docker does not accept uppercase. The ID must also be unique for each validating peer.

We are also using the default consensus, called NOOPS, which doesn't really do consensus. If you want to use some other consensus plugin, see note 1 below.

Start up the first validating peer:

```
docker run --rm -it -e OPENCHAIN_VM_ENDPOINT=http://172.17.0.1:4243 -e OPENCHAIN_PEER_ID=vp1 -e OPENCHAIN_PEER_ADDRESSAUTODETECT=true openchain-peer obc-peer peer
```

Start up the second validating peer: We need to get the IP address of the first validating peer, which will act as the root node that the new peer will connect to. The address is printed out on the terminal window of the first peer (eg 172.17.0.2). We'll use "vp2" as the ID for the second validating peer.

```
docker run --rm -it -e OPENCHAIN_VM_ENDPOINT=http://172.17.0.1:4243 -e OPENCHAIN_PEER_ID=vp2 -e OPENCHAIN_PEER_ADDRESSAUTODETECT=true -e OPENCHAIN_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 openchain-peer obc-peer peer
```

You can start up a few more validating peers in the similar manner as you wish. Remember to change the ID.

### Deploying the sample chaincode
We deploy chaincode to the network with CLI. We can use the sample chaincode to test the network. You may find the chaincode here  $GOPATH/src/github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02

Now deploy the chaincode to the network. We can deploy to any validating peer by specifying OPENCHAIN_PEER_ADDRESS:

```
cd $GOPATH/src/github.com/openblockchain/obc-peer
OPENCHAIN_PEER_ADDRESS=172.17.0.2:30303 ./obc-peer chaincode deploy -p github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02 -v 0.0.1 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
```

You can watch for the message "Received build request for chaincode spec" on the output screen of all validating peers.

We can run an invoke transaction to move some 10 from 'a' to 'b':

```
OPENCHAIN_PEER_ADDRESS=172.17.0.2:30303 ./obc-peer chaincode invoke -p github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02 -v 0.0.1 -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'
```

We can also run a query to see the current value 'a' has:

```
./obc-peer chaincode query -l golang -p github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02 -v 0.0.1 -c '{"Function": "query", "Args": ["a"]}'
```
