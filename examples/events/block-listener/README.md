What
====
block-listener.go will connect to a peer and recieve blocks. For every transaction in the 
NonHashData in the block, it will test the ErrorCode field and print success/failure.

To Run
======

go build

./block-listener -events-address=<event address>

Example with PBFT
=================
1. Run 4 docker peers with PBFT
docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:4243 -e CORE_PEER_ID=vp0 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_PEER_VALIDATOR_CONSENSUS_PLUGIN=pbft hyperledger-peer peer peer

docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:4243 -e CORE_PEER_ID=vp1 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 -e CORE_PEER_VALIDATOR_CONSENSUS_PLUGIN=pbft hyperledger-peer peer peer

docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:4243 -e CORE_PEER_ID=vp2 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 -e CORE_PEER_VALIDATOR_CONSENSUS_PLUGIN=pbft hyperledger-peer peer peer

docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:4243 -e CORE_PEER_ID=vp3 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_PEER_DISCOVERY_ROOTNODE=172.17.0.2:30303 -e CORE_PEER_VALIDATOR_CONSENSUS_PLUGIN=pbft hyperledger-peer peer peer

2. Attach event client
./block-listener -event-address=172.17.0.2:31315

Event client should output "Event Address: 172.17.0.2:31315" and wait for events

3. Deploy
CORE_PEER_ADDRESS=172.17.0.2:30303 ./peer chaincode deploy -p github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'

Notice output in the events client

4. Invoke - good
CORE_PEER_ADDRESS=172.17.0.2:30303 ./peer chaincode invoke -n 5844bc142dcc9e788785e026e22c855957b2c754c912702c58d997dedbc9a042f05d152f6db0fbd7810d95c1b880c210566c9de3093aae0ab76ad2d90e9cfaa5 -c '{"Function":"invoke", "Args": ["a","b","10"]}'

Notice success transaction in events client

5. Invoke - bad
CORE_PEER_ADDRESS=172.17.0.2:30303 ./peer chaincode invoke -n 5844bc142dcc9e788785e026e22c855957b2c754c912702c58d997dedbc9a042f05d152f6db0fbd7810d95c1b880c210566c9de3093aae0ab76ad2d90e9cfaa5 -c '{"Function":"invoke", "Args": ["a","b"]}'

Notice error transaction in events client
