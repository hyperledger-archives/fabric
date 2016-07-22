## Setting Up a Network For Development

This document covers setting up a network on your local machine for development using Docker containers.

All commands should be run from within the Vagrant environment described in [Setting Up Development Environment](devenv.md).
See [Logging Control](logging-control.md) for information on controlling
logging output from the `peer` and chaincodes.


**Note:** When running with security enabled, follow the security setup instructions described in [Chaincode Development](../API/SandboxSetup.md#security-setup-optional) to set up the CA server and log in registered users before sending chaincode transactions. In this case peers started using Docker images need to point to the correct CA address (default is localhost). CA addresses have to be specified in `peer/core.yaml` variables paddr of eca, tca and tlsca. Furthermore, if you are enabling security and privacy on the peer process with environment variables, it is important to include these environment variables in the command when executing all subsequent peer operations (e.g. deploy, invoke, or query).

### Setting up Docker image
To create a Docker image for the `hyperledger/fabric`, first clean out any active containers (hyperledger/fabric-peer and chaincode) using `docker ps -a` and `docker rm` commands. Second, remove any old images with `docker images` and `docker rmi` commands. **Careful:** Do not remove any other images (like busybox or hyperledger/fabric-baseimage) as they are needed for a correct execution.

Now we are ready to build a new docker image:

```
    cd $GOPATH/src/github.com/hyperledger/fabric
    make peer-image
```

Check the available images again with `docker images`, and you should see `hyperledger/fabric-peer` image.

### Starting up validating peers
From the Vagrant environment, find out which IP address your docker0 interface is on with `ip add` command. For example,

```
vagrant@vagrant-ubuntu-trusty-64:/opt/gopath/src/github.com/hyperledger/fabric$ ip add

<<< detail removed >>>

3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default
    link/ether 02:42:ad:be:70:cb brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.1/16 scope global docker0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:adff:febe:70cb/64 scope link
       valid_lft forever preferred_lft forever
```

Your output might contain something like `inet 172.17.0.1/16 scope global docker0`. That means docker0 interface is on IP address 172.17.0.1. Use that IP address for the `CORE_VM_ENDPOINT` option. For more information on the environment variables, see `core.yaml` configuration file in the `fabric` repository.

The ID value of `CORE_PEER_ID` must be lowercase since we use the ID as part of chaincode containers we build, and docker does not accept uppercase. The ID must also be unique for each validating peer.

By default, we are using a consensus plugin called `NOOPS`, which doesn't really do consensus. If you want to use some other consensus plugin, see Using Consensus Plugin section at the end of the document.

#### Start up the first validating peer:

```
docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:2375 -e CORE_PEER_ID=vp0 -e CORE_PEER_ADDRESSAUTODETECT=true hyperledger/fabric-peer peer node start
```

If starting the peer with security/privacy enabled, environment variables for security, CA address and peer's ID and password must be included:

```
docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:2375 -e CORE_PEER_ID=vp0 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_SECURITY_ENABLED=true -e CORE_SECURITY_PRIVACY=true -e CORE_PEER_PKI_ECA_PADDR=172.17.0.1:50051 -e CORE_PEER_PKI_TCA_PADDR=172.17.0.1:50051 -e CORE_PEER_PKI_TLSCA_PADDR=172.17.0.1:50051 -e CORE_SECURITY_ENROLLID=vp0 -e CORE_SECURITY_ENROLLSECRET=vp0_secret  hyperledger/fabric-peer peer node start
```

Additionally, the validating peer `enrollID` and `enrollSecret` (`vp0` and `vp0_secret`) has to be added to [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml).

#### Start up the second validating peer:
We need to get the IP address of the first validating peer, which will act as the root node that the new peer will connect to. The address is printed out on the terminal window of the first peer (e.g. 172.17.0.2) and should be passed in with the `CORE_PEER_DISCOVERY_ROOTNODE` environment variable. We'll use `vp1` as the ID for the second validating peer.

```
docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:2375 -e CORE_PEER_ID=vp1 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_SECURITY_ENABLED=true -e CORE_SECURITY_PRIVACY=true -e CORE_PEER_PKI_ECA_PADDR=172.17.0.1:50051 -e CORE_PEER_PKI_TCA_PADDR=172.17.0.1:50051 -e CORE_PEER_PKI_TLSCA_PADDR=172.17.0.1:50051 -e CORE_SECURITY_ENROLLID=vp1 -e CORE_SECURITY_ENROLLSECRET=vp1_secret -e CORE_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 hyperledger/fabric-peer peer node start
```

Again, the validating peer `enrollID` and `enrollSecret` (`vp1` and `vp1_secret`) has to be added to [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml).

You can start up a few more validating peers in a similar manner if you wish. Remember to change the peer ID and add the enrollID/enrollSecret to the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml).

### Enroll/Login a test user (if security is enabled):
If security is enabled, you must enroll a user with the certificate authority before sending requests. Choose a user that is already registered, i.e. added to the [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml). Then, execute the command below to log in the user on the target validating peer. `CORE_PEER_ADDRESS` specifies the target validating peer for which the user is to be logged in.

```
CORE_PEER_ADDRESS=172.17.0.2:30303 peer network login jim
```

**Note:** The certificate authority allows the enrollID and enrollSecret credentials to be used only *once*. Therefore, login by the same user from any other validating peer will result in an error. Currently, the application layer is responsible for duplicating the crypto material returned from the CA to other peer nodes. If you want to test secure transactions from more than one peer node without replicating the returned key and certificate, you can log in with a different user on other peer nodes.

### Deploy, Invoke, and Query a Chaincode
**Note:** When security is enabled, modify the CLI commands to deploy, invoke, or query a chaincode to pass the username of a logged in user. To log in a registered user through the CLI, execute the login command from the section above. On the CLI the username is passed with the -u parameter.

We can use the sample chaincode to test the network. You may find the chaincode here `$GOPATH/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02`.

Deploy the chaincode to the network. We can deploy to any validating peer by specifying `CORE_PEER_ADDRESS`:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 peer chaincode deploy -p github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
```

With security enabled, modify the command as follows:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode deploy -u jim -p github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'
```

You can watch for the message "Received build request for chaincode spec" on the output screen of all validating peers.

**Note:** If your GOPATH environment variable contains more than one element, the chaincode must be found in the first one or deployment will fail.

On successful completion, the above command will print the "name" assigned to the deployed chaincode. This "name" is used as the value of the "-n" parameter in invoke and query commands described below. For example the value of "name" could be

    bb540edfc1ee2ac0f5e2ec6000677f4cd1c6728046d5e32dede7fea11a42f86a6943b76a8f9154f4792032551ed320871ff7b7076047e4184292e01e3421889c

In a script the name can be captured for subsequent use. For example, run

    NAME=`CORE_PEER_ADDRESS=172.17.0.2:30303 CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode deploy ...`

and then replace `<name_value_returned_from_deploy_command>` in the examples below with `$NAME`.

We can run an invoke transaction to move 10 units from the value of `a` to the value of `b`:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 peer chaincode invoke -n <name_value_returned_from_deploy_command> -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'
```

With security enabled, modify the command as follows:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode invoke -u jim -n <name_value_returned_from_deploy_command> -c '{"Function": "invoke", "Args": ["a", "b", "10"]}'
```

We can also run a query to see the current value `a` has:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 peer chaincode query -l golang -n <name_value_returned_from_deploy_command> -c '{"Function": "query", "Args": ["a"]}'
```

With security enabled, modify the command as follows:

```
CORE_PEER_ADDRESS=172.17.0.2:30303 CORE_SECURITY_ENABLED=true CORE_SECURITY_PRIVACY=true peer chaincode query -u jim -l golang -n <name_value_returned_from_deploy_command> -c '{"Function": "query", "Args": ["a"]}'
```

### Using Consensus Plugin
A consensus plugin might require some specific configuration that you need to set up. For example, to use Byzantine consensus plugin provided as part of the fabric, perform the following configuration:

1. In `core.yaml`, set the `peer.validator.consensus` value to `pbft`
2. In `core.yaml`, make sure the `peer.id` is set sequentially as `vpX` where `X` is an integer that starts from `0` and goes to `N-1`. For example, with 4 validating peers, set the `peer.id` to`vp0`, `vp1`, `vp2`, `vp3`.
3. In `consensus/obcpbft/config.yaml`, set the `general.mode` value to either `classic`, `batch`, or `sieve`, and the `general.N` value to the number of validating peers on the network (if you do `batch`, also set `general.batchsize` to the number of transactions per batch)
4. In `consensus/obcpbft/config.yaml`, optionally set timer values for the batch period (`general.timeout.batch`), the acceptable delay between request and execution (`general.timeout.request`), and for view-change (`general.timeout.viewchange`)

See `core.yaml` and `consensus/obcpbft/config.yaml` for more detail.

All of these setting may be overriden via the command line environment variables, eg. `CORE_PEER_VALIDATOR_CONSENSUS_PLUGIN=pbft` or `CORE_PBFT_GENERAL_MODE=sieve`
