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
	"fmt"
	"reflect"
	"sync"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

var ledgerLogger = logging.MustGetLogger("ledger")

// Ledger - the struct for openchain ledger
type Ledger struct {
	blockchain *blockchain
	state      *state
	currentID  interface{}
}

var ledger *Ledger
var mutex sync.Mutex

// GetLedger - gives a reference to a singleton ledger
func GetLedger() (*Ledger, error) {
	if ledger == nil {
		mutex.Lock()
		defer mutex.Unlock()
		if ledger == nil {
			blockchain, err := getBlockchain()
			if err != nil {
				return nil, err
			}
			state := getState()
			ledger = &Ledger{blockchain, state, nil}
		}
	}
	return ledger, nil
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

// CommitTxBatch - gets invoked when the current transaction-batch needs to be committed
// This function returns successfully iff the transactions details and state changes (that
// may have happened during execution of this transaction-batch) have been committed to permanent storage
func (ledger *Ledger) CommitTxBatch(id interface{}, transactions []*protos.Transaction, proof []byte) error {
	err := ledger.checkValidIDCommitORRollback(id)
	if err != nil {
		return err
	}
	block := protos.NewBlock("proposerID string", transactions)
	ledger.blockchain.addBlock(context.TODO(), block)
	ledger.resetForNextTxGroup()
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
	ledger.resetForNextTxGroup()
	return nil
}

/////////////////// world-state related methods /////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////

// GetTempStateHash - Computes state hash by taking into account the state changes that may have taken
// place during the execution of current transaction-batch
func (ledger *Ledger) GetTempStateHash() ([]byte, error) {
	return ledger.state.getHash()
}

// GetState get state for chaincodeID and key. This first looks in memory and if missing, pulls from db
func (ledger *Ledger) GetState(chaincodeID string, key string) ([]byte, error) {
	return ledger.state.get(chaincodeID, key)
}

// SetState sets state to given value for chaincodeID and key. Does not immideatly writes to memory
func (ledger *Ledger) SetState(chaincodeID string, key string, value []byte) error {
	return ledger.state.set(chaincodeID, key, value)
}

// DeleteState tracks the deletion of state for chaincodeID and key. Does not immideatly writes to memory
func (ledger *Ledger) DeleteState(chaincodeID string, key string) error {
	return ledger.state.delete(chaincodeID, key)
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
	return ledger.blockchain.getBlock(blockNumber)
}

// GetBlockchainSize returns number of blocks in blockchain
func (ledger *Ledger) GetBlockchainSize() uint64 {
	return ledger.blockchain.getSize()
}

//GetTransactionByUUID return transaction by it's uuid
func (ledger *Ledger) GetTransactionByUUID(txUUID string) (*protos.Transaction, error) {
	return ledger.blockchain.getTransactionByUUID(txUUID)
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

func (ledger *Ledger) resetForNextTxGroup() {
	ledgerLogger.Debug("resetting ledger state for next transaction batch")
	ledger.currentID = nil
	ledger.state.clearInMemoryChanges()
}
