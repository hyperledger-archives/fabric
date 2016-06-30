### Release [v0.5-developer-preview](https://github.com/hyperledger/fabric/tree/v0.5-developer-preview)
June 17, 2016 This is a developer-preview release of the Hyperledger Fabric intended to exercise the release logistics and stabilize a set of capabilities for developers to try out. 

##### Key Features
1. Permissioned blockchain with immediate finality
1. Chaincode (aka smart contract) execution environments
   1. Docker container (user chaincode)
   1. In-process with peer (system chaincode)
1. Pluggable consensus with PBFT, NOOPS (development mode), SIEVE (prototype)
1. Event framework supports pre-defined and custom events
1. Client SDK (Node.js), basic REST APIs and CLIs

##### Known Key Bugs and work in progress
1. [#1895](https://github.com/hyperledger/fabric/issues/1895) Client SDK interfaces may crash if wrong parameter specified
1. [#1901](https://github.com/hyperledger/fabric/issues/1901) Slow response after a few hours of stress testing
1. [#1911](https://github.com/hyperledger/fabric/issues/1911) Missing peer event listener on the client SDK
1. [889](https://github.com/hyperledger/fabric/issues/889)The attributes in the TCert are not encrypted. This work is still on-going


