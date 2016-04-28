/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package ledger

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/db"
	"github.com/hyperledger/fabric/core/ledger/statemgmt"
	"github.com/hyperledger/fabric/core/ledger/statemgmt/state"
	"github.com/hyperledger/fabric/events/producer"
	"github.com/op/go-logging"
	"github.com/tecbot/gorocksdb"

	"github.com/hyperledger/fabric/protos"
	"golang.org/x/net/context"
)

var ledgerLogger = logging.MustGetLogger("ledger")

var (
	// ErrOutOfBounds is returned if a request is out of bounds
	ErrOutOfBounds = errors.New("ledger: out of bounds")

	// ErrResourceNotFound is returned if a resource is not found
	ErrResourceNotFound = errors.New("ledger: resource not found")
)

// Ledger - the struct for openchain ledger
type Ledger struct {
	blockchain *blockchain
	state      *state.State
	currentID  interface{}
}

var ledger *Ledger
var ledgerError error
var once sync.Once

// GetLedger - gives a reference to a 'singleton' ledger
func GetLedger() (*Ledger, error) {
	once.Do(func() {
		ledger, ledgerError = newLedger()
	})
	return ledger, ledgerError
}

func newLedger() (*Ledger, error) {
	blockchain, err := newBlockchain()
	if err != nil {
		return nil, err
	}

	state := state.NewState()
	return &Ledger{blockchain, state, nil}, nil
}

/////////////////// Transaction-batch related methods ///////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////

// BeginTxBatch - gets invoked when next round of transaction-batch execution begins
func (ledger *Ledger) BeginTxBatch(id interface{}) error {
	err := ledger.checkValidIDBegin()
	if err != nil {
		return err
	}
	ledger.currentID = id
	return nil
}

// GetTXBatchPreviewBlock returns a preview block that will have the same
// block.GetHash() result as the block commited to the database if
// ledger.CommitTxBatch is called with the same parameters. If the state is modified
// by a transaction between these two calls, the hash will be different. The
// preview block does not include non-hashed data such as the local timestamp.
func (ledger *Ledger) GetTXBatchPreviewBlock(id interface{},
	transactions []*protos.Transaction, metadata []byte) (*protos.Block, error) {
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return nil, err
	}
	stateHash, err := ledger.state.GetHash()
	if err != nil {
		return nil, err
	}
	return ledger.blockchain.buildBlock(protos.NewBlock(transactions, metadata), stateHash), nil
}

// CommitTxBatch - gets invoked when the current transaction-batch needs to be committed
// This function returns successfully iff the transactions details and state changes (that
// may have happened during execution of this transaction-batch) have been committed to permanent storage
func (ledger *Ledger) CommitTxBatch(id interface{}, transactions []*protos.Transaction, transactionResults []*protos.TransactionResult, metadata []byte) error {
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return err
	}

	stateHash, err := ledger.state.GetHash()
	if err != nil {
		ledger.resetForNextTxGroup(false)
		ledger.blockchain.blockPersistenceStatus(false)
		return err
	}

	writeBatch := gorocksdb.NewWriteBatch()
	defer writeBatch.Destroy()
	block := protos.NewBlock(transactions, metadata)
	block.NonHashData = &protos.NonHashData{TransactionResults: transactionResults}
	newBlockNumber, err := ledger.blockchain.addPersistenceChangesForNewBlock(context.TODO(), block, stateHash, writeBatch)
	if err != nil {
		ledger.resetForNextTxGroup(false)
		ledger.blockchain.blockPersistenceStatus(false)
		return err
	}
	ledger.state.AddChangesForPersistence(newBlockNumber, writeBatch)
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	dbErr := db.GetDBHandle().DB.Write(opt, writeBatch)
	if dbErr != nil {
		ledger.resetForNextTxGroup(false)
		ledger.blockchain.blockPersistenceStatus(false)
		return dbErr
	}

	ledger.resetForNextTxGroup(true)
	ledger.blockchain.blockPersistenceStatus(true)

	sendProducerBlockEvent(block)
	return nil
}

// RollbackTxBatch - Descards all the state changes that may have taken place during the execution of
// current transaction-batch
func (ledger *Ledger) RollbackTxBatch(id interface{}) error {
	ledgerLogger.Debug("RollbackTxBatch for id = [%s]", id)
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return err
	}
	ledger.resetForNextTxGroup(false)
	return nil
}

// TxBegin - Marks the begin of a new transaction in the ongoing batch
func (ledger *Ledger) TxBegin(txUUID string) {
	ledger.state.TxBegin(txUUID)
}

// TxFinished - Marks the finish of the on-going transaction.
// If txSuccessful is false, the state changes made by the transaction are discarded
func (ledger *Ledger) TxFinished(txUUID string, txSuccessful bool) {
	ledger.state.TxFinish(txUUID, txSuccessful)
}

/////////////////// world-state related methods /////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////

// GetTempStateHash - Computes state hash by taking into account the state changes that may have taken
// place during the execution of current transaction-batch
func (ledger *Ledger) GetTempStateHash() ([]byte, error) {
	return ledger.state.GetHash()
}

// GetTempStateHashWithTxDeltaStateHashes - In addition to the state hash (as defined in method GetTempStateHash),
// this method returns a map [txUuid of Tx --> cryptoHash(stateChangesMadeByTx)]
// Only successful txs appear in this map
func (ledger *Ledger) GetTempStateHashWithTxDeltaStateHashes() ([]byte, map[string][]byte, error) {
	stateHash, err := ledger.state.GetHash()
	return stateHash, ledger.state.GetTxStateDeltaHash(), err
}

// GetState get state for chaincodeID and key. If committed is false, this first looks in memory
// and if missing, pulls from db.  If committed is true, this pulls from the db only.
func (ledger *Ledger) GetState(chaincodeID string, key string, committed bool) ([]byte, error) {
	return ledger.state.Get(chaincodeID, key, committed)
}

// GetStateRangeScanIterator returns an iterator to get all the keys (and values) between startKey and endKey
// (assuming lexical order of the keys) for a chaincodeID.
// If committed is true, the key-values are retrived only from the db. If committed is false, the results from db
// are mergerd with the results in memory (giving preference to in-memory data)
// The key-values in the returned iterator are not guaranteed to be in any specific order
func (ledger *Ledger) GetStateRangeScanIterator(chaincodeID string, startKey string, endKey string, committed bool) (statemgmt.RangeScanIterator, error) {
	return ledger.state.GetRangeScanIterator(chaincodeID, startKey, endKey, committed)
}

// SetState sets state to given value for chaincodeID and key. Does not immideatly writes to DB
func (ledger *Ledger) SetState(chaincodeID string, key string, value []byte) error {
	return ledger.state.Set(chaincodeID, key, value)
}

// DeleteState tracks the deletion of state for chaincodeID and key. Does not immideatly writes to DB
func (ledger *Ledger) DeleteState(chaincodeID string, key string) error {
	return ledger.state.Delete(chaincodeID, key)
}

// CopyState copies all the key-values from sourceChaincodeID to destChaincodeID
func (ledger *Ledger) CopyState(sourceChaincodeID string, destChaincodeID string) error {
	return ledger.state.CopyState(sourceChaincodeID, destChaincodeID)
}

// GetStateMultipleKeys returns the values for the multiple keys.
// This method is mainly to amortize the cost of grpc communication between chaincode shim peer
func (ledger *Ledger) GetStateMultipleKeys(chaincodeID string, keys []string, committed bool) ([][]byte, error) {
	return ledger.state.GetMultipleKeys(chaincodeID, keys, committed)
}

// SetStateMultipleKeys sets the values for the multiple keys.
// This method is mainly to amortize the cost of grpc communication between chaincode shim peer
func (ledger *Ledger) SetStateMultipleKeys(chaincodeID string, kvs map[string][]byte) error {
	return ledger.state.SetMultipleKeys(chaincodeID, kvs)
}

// GetStateSnapshot returns a point-in-time view of the global state for the current block. This
// should be used when transfering the state from one peer to another peer. You must call
// stateSnapshot.Release() once you are done with the snapsnot to free up resources.
func (ledger *Ledger) GetStateSnapshot() (*state.StateSnapshot, error) {
	dbSnapshot := db.GetDBHandle().GetSnapshot()
	blockHeight, err := fetchBlockchainSizeFromSnapshot(dbSnapshot)
	if err != nil {
		dbSnapshot.Release()
		return nil, err
	}
	if 0 == blockHeight {
		dbSnapshot.Release()
		return nil, fmt.Errorf("Blockchain has no blocks, cannot determine block number")
	}
	return ledger.state.GetSnapshot(blockHeight-1, dbSnapshot)
}

// GetStateDelta will return the state delta for the specified block if
// available.  If not available because it has been discarded, returns nil,nil.
func (ledger *Ledger) GetStateDelta(blockNumber uint64) (*statemgmt.StateDelta, error) {
	if blockNumber >= ledger.GetBlockchainSize() {
		return nil, ErrOutOfBounds
	}
	return ledger.state.FetchStateDeltaFromDB(blockNumber)
}

// ApplyStateDelta applies a state delta to the current state. This is an
// in memory change only. You must call ledger.CommitStateDelta to persist
// the change to the DB.
// This should only be used as part of state synchronization. State deltas
// can be retrieved from another peer though the Ledger.GetStateDelta function
// or by creating state deltas with keys retrieved from
// Ledger.GetStateSnapshot(). For an example, see TestSetRawState in
// ledger_test.go
// Note that there is no order checking in this function and it is up to
// the caller to ensure that deltas are applied in the correct order.
// For example, if you are currently at block 8 and call this function
// with a delta retrieved from Ledger.GetStateDelta(10), you would now
// be in a bad state because you did not apply the delta for block 9.
// It's possible to roll the state forwards or backwards using
// stateDelta.RollBackwards. By default, a delta retrieved for block 3 can
// be used to roll forwards from state at block 2 to state at block 3. If
// stateDelta.RollBackwards=false, the delta retrived for block 3 can be
// used to roll backwards from the state at block 3 to the state at block 2.
func (ledger *Ledger) ApplyStateDelta(id interface{}, delta *statemgmt.StateDelta) error {
	err := ledger.checkValidIDBegin()
	if err != nil {
		return err
	}
	ledger.currentID = id
	ledger.state.ApplyStateDelta(delta)
	return nil
}

// CommitStateDelta will commit the state delta passed to ledger.ApplyStateDelta
// to the DB
func (ledger *Ledger) CommitStateDelta(id interface{}) error {
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return err
	}
	defer ledger.resetForNextTxGroup(true)
	return ledger.state.CommitStateDelta()
}

// RollbackStateDelta will discard the state delta passed
// to ledger.ApplyStateDelta
func (ledger *Ledger) RollbackStateDelta(id interface{}) error {
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return err
	}
	ledger.resetForNextTxGroup(false)
	return nil
}

// DeleteALLStateKeysAndValues deletes all keys and values from the state.
// This is generally only used during state synchronization when creating a
// new state from a snapshot.
func (ledger *Ledger) DeleteALLStateKeysAndValues() error {
	return ledger.state.DeleteState()
}

/////////////////// blockchain related methods /////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////

// GetBlockchainInfo returns information about the blockchain ledger such as
// height, current block hash, and previous block hash.
func (ledger *Ledger) GetBlockchainInfo() (*protos.BlockchainInfo, error) {
	return ledger.blockchain.getBlockchainInfo()
}

// GetBlockByNumber return block given the number of the block on blockchain.
// Lowest block on chain is block number zero
func (ledger *Ledger) GetBlockByNumber(blockNumber uint64) (*protos.Block, error) {
	if blockNumber >= ledger.GetBlockchainSize() {
		return nil, ErrOutOfBounds
	}
	return ledger.blockchain.getBlock(blockNumber)
}

// GetBlockchainSize returns number of blocks in blockchain
func (ledger *Ledger) GetBlockchainSize() uint64 {
	return ledger.blockchain.getSize()
}

// GetTransactionByUUID return transaction by it's uuid
func (ledger *Ledger) GetTransactionByUUID(txUUID string) (*protos.Transaction, error) {
	return ledger.blockchain.getTransactionByUUID(txUUID)
}

// PutRawBlock puts a raw block on the chain. This function should only be
// used for synchronization between peers.
func (ledger *Ledger) PutRawBlock(block *protos.Block, blockNumber uint64) error {
	err := ledger.blockchain.persistRawBlock(block, blockNumber)
	if err != nil {
		return err
	}
	sendProducerBlockEvent(block)
	return nil
}

// VerifyChain will verify the integrety of the blockchain. This is accomplished
// by ensuring that the previous block hash stored in each block matches
// the actual hash of the previous block in the chain. The return value is the
// block number of the block that contains the non-matching previous block hash.
// For example, if VerifyChain(0, 99) is called and prevous hash values stored
// in blocks 8, 32, and 42 do not match the actual hashes of respective previous
// block 42 would be the return value from this function.
// highBlock is the high block in the chain to include in verofication. If you
// wish to verify the entire chain, use ledger.GetBlockchainSize() - 1.
// lowBlock is the low block in the chain to include in verification. If
// you wish to verify the entire chain, use 0 for the genesis block.
func (ledger *Ledger) VerifyChain(highBlock, lowBlock uint64) (uint64, error) {
	if highBlock >= ledger.GetBlockchainSize() {
		return highBlock, ErrOutOfBounds
	}
	if highBlock <= lowBlock {
		return lowBlock, ErrOutOfBounds
	}

	for i := highBlock; i > lowBlock; i-- {
		currentBlock, err := ledger.GetBlockByNumber(i)
		if err != nil {
			return i, fmt.Errorf("Error fetching block %d.", i)
		}
		if currentBlock == nil {
			return i, fmt.Errorf("Block %d is nil.", i)
		}
		previousBlock, err := ledger.GetBlockByNumber(i - 1)
		if err != nil {
			return i - 1, fmt.Errorf("Error fetching block %d.", i)
		}
		if previousBlock == nil {
			return i - 1, fmt.Errorf("Block %d is nil.", i-1)
		}

		previousBlockHash, err := previousBlock.GetHash()
		if err != nil {
			return i - 1, fmt.Errorf("Error calculating block hash for block %d.", i-1)
		}
		if bytes.Compare(previousBlockHash, currentBlock.PreviousBlockHash) != 0 {
			return i, nil
		}
	}

	return 0, nil
}

func (ledger *Ledger) checkValidIDBegin() error {
	if ledger.currentID != nil {
		return fmt.Errorf("Another TxGroup [%s] already in-progress", ledger.currentID)
	}
	return nil
}

func (ledger *Ledger) checkValidIDCommitORRollback(id interface{}) error {
	if !reflect.DeepEqual(ledger.currentID, id) {
		return fmt.Errorf("Another TxGroup [%s] already in-progress", ledger.currentID)
	}
	return nil
}

func (ledger *Ledger) resetForNextTxGroup(txCommited bool) {
	ledgerLogger.Debug("resetting ledger state for next transaction batch")
	ledger.currentID = nil
	ledger.state.ClearInMemoryChanges(txCommited)
}

func sendProducerBlockEvent(block *protos.Block) {

	// Remove payload from deploy transactions. This is done to make block
	// events more lightweight as the payload for these types of transactions
	// can be very large.
	blockTransactions := block.GetTransactions()
	for _, transaction := range blockTransactions {
		if transaction.Type == protos.Transaction_CHAINCODE_DEPLOY {
			deploymentSpec := &protos.ChaincodeDeploymentSpec{}
			err := proto.Unmarshal(transaction.Payload, deploymentSpec)
			if err != nil {
				ledgerLogger.Error(fmt.Sprintf("Error unmarshalling deployment transaction for block event: %s", err))
				continue
			}
			deploymentSpec.CodePackage = nil
			deploymentSpecBytes, err := proto.Marshal(deploymentSpec)
			if err != nil {
				ledgerLogger.Error(fmt.Sprintf("Error marshalling deployment transaction for block event: %s", err))
				continue
			}
			transaction.Payload = deploymentSpecBytes
		}
	}

	producer.Send(producer.CreateBlockEvent(block))
}
