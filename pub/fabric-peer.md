## Hyperledger fabric-peer

The Hyperledger [fabric](https://github.com/hyperledger/fabric) is an implementation of blockchain technology, that has been collaboratively developed under the Linux Foundation's [Hyperledger Project](http://hyperledger.org). It leverages familiar and proven technologies, and offers a modular architecture that allows pluggable implementations of various function including membership services, consensus, and smart contracts (chaincode) execution. It features powerful container technology to host any mainstream language for smart contracts development.

## Usage

### Vagrant

If Docker running within Vagrant, identify the IP of the Docker daemon host:

```
$ ip add
...
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default
    link/ether 02:42:9c:9a:d9:22 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.1/16 scope global docker0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:9cff:fe9a:d922/64 scope link
       valid_lft forever preferred_lft forever
```

in the example output above, that would be ```172.17.0.1```. We'll use this IP as the value of the ```CORE_VM_ENDPOINT``` variable.

To start a single peer instance, without membership services, run the following docker command:

```
docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:2375 -e CORE_PEER_ID=vp0 -e CORE_PEER_ADDRESSAUTODETECT=true hyperledger/fabric-peer peer node start
```

### Natively on Mac or Windows

If running Docker natively on Mac or Windows, the value of ```CORE_VM_ENDPOINT``` should be set to ```127.0.0.1``` (localhost).

### Using Docker Compose

When running in Vagrant, you can use the docker-compose yaml files in the ```fabric/bddtests``` [directory](https://github.com/hyperledger/fabric/tree/master/bddtests) to start up a network of peers, with or without the membership services.

When running natively on Mac or Windows, the following docker-compose.yml can be used to start a peer with, or without, the membersrvc service:

```
vp:
  image: hyperledger/fabric-peer
  ports:
  - "5000:5000"
  environment:
  - CORE_PEER_ADDRESSAUTODETECT=true
  - CORE_VM_ENDPOINT=http://127.0.0.1:2375
  - CORE_LOGGING_LEVEL=DEBUG
  command: peer node start
membersrvc:
  image: hyperledger/fabric-membersrvc
  command: membersrvc
```

### Advanced Configuration

Please visit the Hyperledger Fabric [documentation](http://hyperledger-fabric.readthedocs.io/en/latest/dev-setup/devnet-setup/) for more advanced configurations.
