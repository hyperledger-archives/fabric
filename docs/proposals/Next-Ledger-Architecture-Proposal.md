**Draft** / **Work in Progress**

[Proposal Discussion](https://github.com/hyperledger/fabric/issues/1666)

This page documents a proposal for a future ledger architecture based on community feedback. All input is welcome as the goal is to make this a community effort.

##### Table of Contents  
[Motivation](#motivation)  
[API](#api)  
[Point-in-Time Queries](#pointintime)  
[Query Language](#querylanguage)

<a name="motivation"/>
## Motivation
The motivation for exploring a new ledger architecture is based on community feedback. While the existing ledger is able to support some (but not all) of the below requirements, we wanted to explore what a new ledger would look like given all that has been learned. Based on many discussions in the community over Slack, GitHub, and the face to face hackathons, it is clear that there is a strong desire to support the following requirements:

1. Point in time queries - The ability to query chaincode state at previous blocks and easily trace lineage **without** replaying transactions
2. SQL like query language
3. Privacy - The complete ledger may not reside on all committers
4. Cryptographically secure ledger - Data integrity without consulting other nodes
5. Support for consensus algorithms that provides immediate finality like PBFT
6. Support consensus algorithms that require stochastic convergence like PoW, PoET
7. Pruning - Ability to remove old transaction data as needed.
8. Support separation of endorsement from consensus as described in the [Next Consensus Architecture Proposal](https://github.com/hyperledger/fabric/wiki/Next-Consensus-Architecture-Proposal). This implies that some peers may apply endorsed results to their ledger **without** executing transactions or viewing chaincode logic.
9. API / Enginer separation. The ability to plug in different storage engines as needed.

<a name="api"/>
## API

Proposed API in Go pseudocode

```
type TxSimulationRead struct {
	ChaincodeID string
	Key         string
	Version     string // auto-incrementing number or previous TxId
}

type TxSimulationWrite struct {
	Read     TxSimulationRead
	NewValue []byte
}

type TxSimulationReadWriteSet struct {
	ReadSet  []TxSimulationRead
	WriteSet []TxSimulationWrite
}

type SimulatedTransaction struct {
	// add TxSimulationReadWriteSet into payload
	tx protos.Transaction
}

type RawBlock struct {
	// blk contains
	blk protos.Block
}

type BlockHeader struct {
}

type RangeScanIterator interface {

	// Next moves to next key-value. Returns true if next key-value exists
	Next() bool

	// GetKeyValue returns next key-value
	GetKeyValue() (string, []byte)

	// Close releases resources occupied by the iterator
	Close()
}

type TxSimulator interface {

	// These return the most recent value for a given key
	GetState(chaincodeID string, key string) ([]byte, error)
	SetState(chaincodeID string, key string, value []byte)
	GetStateRangeScanIterator(chaincodeID string, startKey string, endKey string) RangeScanIterator
	DeleteState(chaincodeID string, key string)
	GetStateMultipleKeys(chaincodeID string, keys []string) ([][]byte, error)
	SetStateMultipleKeys(chaincodeID string, kvs map[string][]byte)
	// This can be a large payload
	CopyState(sourceChaincodeID string, destChaincodeID string) error
	GetReadWriteSet() *TxSimulationReadWriteSet
	HasConflicts() bool
	Clear()

	// Return transaction or value? Possibly need both?
	ExecuteQuery(query string) []*protos.Transaction

	// Returns latest transactions and previous transactions in order
	GetTransactionsForKey(chaincodeID string, key string)  (<-chan *protos.Transaction, <-chan error, error)
}

type Ledger interface {

	//NewTxSimulator is expected to be used by a transaction simulation module.
	NewTxSimulator() (TxSimulator, error)

	// Blockchain related functions
	GetTopBlockHashes() []string // For consensus algorithms which support forking
	GetBlockHeaders(startingBlockHash string, endingBlockHash string) []*BlockHeader // For consensus algorithms which support forking
	GetBlocks(startingBlockHash string, endingBlockHash string) []*protos.Block // For consensus algorithms which support forking
	SwitchTo(blockHash string) // For consensus algorithms which support forking
	GetBlockByNumber(uint64) []*protos.Block

	// State read functions to be used for read queries
	// Returns the most recent value for a given key
	GetState(chaincodeID string, key string) ([]byte, error)
	GetStateRangeScanIterator(chaincodeID string, startKey string, endKey string) RangeScanIterator
	GetStateMultipleKeys(chaincodeID string, keys []string) ([][]byte, error)

	// Return transaction or value? Possibly need both?
	ExecuteQuery(query string) []*protos.Transaction

	// CommitRawBlock accepts a raw block from consensus
	CommitRawBlock(rawBlock *RawBlock) error

	// Functions used during state transfer
	CommitBlock(block *protos.Block) error

	// General functions related blockchain
	GetBlockchainInfo() (*protos.BlockchainInfo, error)
	GetBlockchainSize() uint64
	VerifyChain(highBlock, lowBlock uint64) (uint64, error)

	GetTransactionByID(txID string) (*protos.Transaction, error)

	// Returns latest transactions and previous transactions in order
	GetTransactionsForKey(chaincodeID string, key string)  (<-chan *protos.Transaction, <-chan error, error)

	// This takes an interface to be defined by the engine
	Prune({}interface) error


}
```

# Engine specific thoughts

<a name="pointintime"/>
### Point-in-Time Queries
In abstract temporal terms, there are three varieties of query important to chaincode and application developers:

1. Retrieve the most recent value of a key. (type: current; ex. How much money is in Alice's account?)
2. Retrieve the value of a key at a specific time. (type: historical; ex. What was Alice's account balance at the end of last month?)
3. Retrieve all values of a key over time. (type: lineage; ex. Produce a statement listing all of Alice's transactions.)

When formulating a query, a developer will benefit from the ability to filter, project, and relate transactions to one-another. Consider the following examples:

1. Simple Filtering: Find all accounts that fell below a balance of $100 in the last month.
2. Complex Filtering: Find all of Trudy's transactions that occurred in Iraq or Syria where the amount is above a threshold and the other party has a name that matches a regular expression.
3. Relating: Determine if Alice has ever bought from the same gas station more than once in the same day. Feed this information into a fraud detection model.
4. Projection: Retrieve the city, state, country, and amount of Alice's last ten transactions. This information will be fed into a risk/fraud detection model.

<a name="querylanguage"/>
### Query Language
Developing a query language to support such a diverse range of queries will not be simple. The challenges are:

1. Scaling the query language with developers as their needs grow. To date, the requests from developers have been modest. As the Hyperledger project's user base grows, so will the query complexity.
2. There are two nearly disjoint classes of query:
    1. Find a single value matching a set of constraints. Amenable to existing SQL and NoSQL grammars.
    2. Find a chain or chains of transactions satisfying a set of constraints. Amenable to graph query languages, such as Neo4J's Cypher or SPARQL.