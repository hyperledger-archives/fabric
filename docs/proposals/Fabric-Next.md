***Status: Draft (not ready for review; not yet prioritized)***

This page contains a list of high-level items proposed for development of the next generation Fabric. It includes both existing feature enhancements and new features.

#### 1. Introduction
The key motivation for the next generation is modularization to achieve plug-ability and concurrency. As outlined in the [next consensus architecture](Next-Consensus-Architecture-Proposal) and the [next ledger structure](Next-Ledger-Architecture-Proposal), we want to enable different implementations to plug in at different points, including consensus, storage, and membership services. We can achieve concurrency at endorsers and consenters based on the transaction endorsement policy, which allows driving multiple channels of consensus as a service.

##### 1.2. Deployment Scenarios
The new architecture offers great flexibility in deployments. As the current generation, the simple 1 peer development environment, where a developer may quickly iterate over edit-compile-debug of the fabric code or a chaincode, is still intact. What change are the varieties of network deployment options, from a simple network of a few peers to a complex hundred of peers with different configurations possible.

Conceptually a peer may perform all the functions (submitting, endorsing, ordering, and committing transactions); this is called full peer, or peer. A partial or light peer may perform a specific task like consensus (ordering transactions) or a combination of submitting, endorsing, and committing transactions. A member of a network generally owes a peer and/or some light peers. For high availability, a member may deploy multiple peers but 1 endorsement vote. Currently we have 2 types of peers: Validating peer (VP) and none-validating peer (NVP). We may roughly think of VP as a full peer and NVP as a light peer with submitting and committing tasks.

##### 1.3. Network Deployment
A simple network could be made up of several full peers, where each peer owed by a member with unique enrollment identity in a permissioned network. Building on this, a member may add more peers to increase availability and to support fail-over; however, these single-member peers must operate as a cluster with 1 vote on endorsement. A cluster of peers could be achieved by a shared ledger and a proxy to dispatch messages to the member peers, or a leader-followers pattern.

##### 1.4. Consensus Service
Consenters may independently operate to form a consensus network, isolating from the other peer functions (submitting, endorsing and committing transactions). This capability allows the members to set up a consensus network such that it can serve multiple blockchains, which only need to deploy light peers (without consenter enabled). The consensus network can be agnostic to the transactions enabling data isolation such that submitters may replace the transaction payload with its hash.

#### 2. Detail Development Items
The following items either come from the proposed next architecture or known enhancements to the existing functions that have been circulated as **issues**. Each item will be associated with 1 or more issues, but they are all here for the big picture.

##### 2.1. Consensus
1. Decouple consensus from peer: We've been always wanting to separate the consensus module into its own gRPC process with a simple peer interface to enable other implementation to easily plugin.

1. Define consensus protocols
1. Extracting PBFT
1. Consensus channels: A blockchain network has 1 consensus channel by default that every peer listens on; this is called public channel. We can establish a separate channel per confidential domain where only permitted peers may subscribe to. Transactions sent to a channel will be ordered respective to each other within the channel, so a batch only contains transactions from the channel, not from any other channels.

1. Dynamic membership: Allowing peers to come and go, and the blockchain network should continue to operate according to the consensus algorithm.

1. Bound the number of incoming transactions
  
##### 2.2. Ledger
1. Transaction rw-set: A submitter composes a transaction consisting of [header, payload, rw-set] where rw-set is the set containing the state variables that the transaction reads from (read-set) and the state variables that the transaction writes to (write-set). The rw-set is created via transaction execution simulation (not writing to database). This simulation is also done by the endorsers of the transaction to fulfill the endorsement policy of the transaction.

1. Enhance API to enable pluggable datastore: Decouple the current API implementation from RocksDB to enable plugging in different datastore.

1. File-based datastore: This is the default (reference) implementation, where blocks (transaction log) are stored in structured files as marshaled objects such that data replication is a matter of copying files.

1. World state cache: For better performance, we need to cache the state to help submitters quickly build the rw-set.

1. Ability to archive/prune "spent" transactions: To reduce the size of the ledger, peers may removed the transactions that are no longer needed (of course that transaction must have been archived) for operation. A transaction is not needed to be in the ledger when its write-set has been consumed. We don't think that it is necessary to remove the block structure since it is small and fixed, but we can retain the transaction ID in the block.

1. SQL-like queries (point in time, filter): Many requests from developers to provide ability to query states or transactions with some filters.

##### 2.3. Chaincode
Currently we have 2 types of chaincodes: System and user chaincodes. System chaincode is built with the peer code and initialized during peer startup, whereas, user chaincode is built during deploy transaction and runs in a sandbox (default to Docker container).

System chaincode concept allows us to reorganize the peer chaincode services into system chaincodes. For example, chaincode life-cycle (deploy, upgrade, terminate), chaincode naming, key management, etc can be implemented as system chaincodes since these are "trusted" operations which require access to the peer. 

1. Life-cycle management [issue 1127](https://github.com/hyperledger/fabric/issues/1127): A chaincode begins life at deployment and ends at termination (out of service). Along this timeline, there might be several upgrades either to fix bugs or to amend functions. We have implemented deployment (via `Deploy` transaction) but have not done (completed) upgrade and terminate. Upgrading of a chaincode is not only involved code changes but also data (and potentially security) migration. We can implement chaincode life-cycle in a system chaincode, named "uber-chaincode", such that, to deploy a chaincode, we would send an `Invoke` transaction to the uber chaincode; similarly for upgrade and terminate. This means that, our transaction types now would only consist of `Invoke` and `Query`. And `Deploy` transaction is no longer needed.

1. Naming service [issue 1077](https://github.com/hyperledger/fabric/issues/1077): Naming service is a system chaincode to map chaincode ID to a user-friendly name, allowing applications to use the name instead of the long ID to address a chaincode.
 
1. Calling another chaincode with security: We currently have chaincode calling chaincode locally but without security. Security means both access control and transaction confidentiality, which means whether the callee is visible to the caller. Multiple confidential domains complicate the picture, so the first implementation should focus on chaincode calling chaincode within the same confidential domain. Calling a chaincode in the "public" domain should allow only queries since changes to a chaincode state requires a transaction visible to that chaincode's endorsers.

1. Access control: The [TCert attributes](https://github.com/hyperledger/fabric/blob/master/docs/tech/attributes.md) provide a mechanism for the chaincode code to control whom may perform the function by matching the attributes with the intended permissions. For example, uber chaincode may control who can deploy chaincodes by adding access control logic to the `Deploy` function.

##### 2.4. Membership Services
1. TCert attributes: We continue the development of attributes as part of TCert, including backend API for external systems to inject attributes and encryption of the attribute values.
1. Decentralization: Members of a network want ability to enroll their users.
1. Key rotation: Ability to replace peer key as a system chaincode transaction. 
1. HSM support: Ability to work with HSM to protect keys.
1. Auditability: Provide auditing APIs

##### 2.5. Protocol
1. Enhance message structures: Remove unused or duplicated fields; add version number.
1. Status codes and messages: Standardize the response message status codes and their messages.
1. Error handling: Standardize the error messages and their handling such as when to bow out vs log and move on vs propagate up the chain.
1. Extensions: How to extend the message protocol to allow the application to add more fields.

##### 2.6. SDK
1. Nodejs: Event handling, chaincode deployment API
1. Java: Similar to Node.js SDK
1. Go ? Any need for Golang SDK?

##### 2.7. Transaction confidentiality
1. Endorsers
1. Users
1. Event security

##### 2.8. Upgrade Fabric
We divide the fabric upgrade into 3 phases to gradually build up the capabilities.

1. Bug fixes: This is applying patches to the peer code such that no backward compatibility needed. The thought here is that one may just replace the peer code with the new build, and it should resume.
1. Protocol changes: This kind of updates requires backward compatibility.
1. Ledger migration? We don't anticipate this kind of updates any time soon.

##### 2.9. Integration (SoR)
