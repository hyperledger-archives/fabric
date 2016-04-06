## Hyperledger Fabric Chaincode (Smart Contract and Digital Assets)

&nbsp;
##### Does the Hyperledger fabric support smart contract?
Yes. Chaincode is the Hyperledger fabric’s interpretation of the smart contract method/algorithm, with additional features.

Chaincodes are codes deployed on the Hyperledger fabric network, where they are executed and validated by chain validators together during the consensus process. Developers can use chaincodes to develop business contracts, asset definitions, and collectively-managed decentralized applications.

&nbsp;
##### How do I create a business contract on the Hyperledger fabric?
There are generally two ways to develop business contracts: the first way is to code individual contracts into standalone instances of chaincode; the second way, and probably the more efficient way, is to use chaincode to create decentralized applications that manage the life cycle of one or multiple types of business contracts, and let end users instantiate instances of contracts within these applications. 

&nbsp;
##### How do I create assets on the Hyperledger fabric?
Hyperledger fabricusers can use chaincode (for business rules) and membership service (for digital tokens) to design assets, as well as the logics that manage them.

There are two popular approaches to defining assets in most blockchain solutions: the stateless UTXO model, where account balances are encoded into past transaction records; and the account model, where account balances are kept in state storage space on the ledger.

Each approach carries its own benefits and drawbacks. The Hyperledger fabricdoes not advocate either one over the other. Instead, one of our first requirements was to ensure that both approaches can be easily implemented with tools available in the Hyperledger fabric.

&nbsp;
##### Which languages are supported for writing chaincode?
Chaincode can be written in any programming language and executed in containers inside the the Hyperledger fabric context layer. We are also looking into developing a templating language (such as Apache Velocity) that can either get compiled into chaincode or have its interpreter embedded into a chaincode container.

The first Hyperledger fabric-supported chaincode language is Golang, and support for JavaScript and Java is planned for 2016. Support for additional languages and the development of a Hyperledger fabric-specific templating language have been discussed, and more details will be released in the near future.

&nbsp;
##### Does the Hyperledger fabric have native currency?
No. However, if you really need a native currency for your chain network, you can develop your own native currency with chaincode. One common attribute of native currency is that some amount will get transacted (the chaincode defining that currency will get called) every time a transaction is processed on its chain.
