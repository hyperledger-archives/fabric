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
	"fmt"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

// State structure for maintaining world state. This is not thread safe
type state struct {
	stateDelta          *stateDelta
	currentTxStateDelta *stateDelta
	currentTxUuid       string
	txStateDeltaHash    map[string][]byte
	statehash           *stateHash
	recomputeHash       bool
}

var stateLogger = logging.MustGetLogger("state")

// should this be configurable in yaml?
var historyStateDeltaSize = uint64(500)
var stateInstance *state

// getState get handle to world state
func getState() *state {
	if stateInstance == nil {
		stateInstance = &state{newStateDelta(), newStateDelta(), "", make(map[string][]byte), nil, true}
	}
	return stateInstance
}

func (state *state) txBegin(txUuid string) {
	stateLogger.Debug("txBegin() for txUuid [%s]", txUuid)
	if state.txInProgress() {
		panic(fmt.Errorf("A tx [%s] is already in progress. Received call for begin of another tx [%s]", state.currentTxUuid, txUuid))
	}
	state.currentTxUuid = txUuid
}

func (state *state) txFinish(txUuid string, txSuccessful bool) {
	stateLogger.Debug("txFinish() for txUuid [%s], txSuccessful=[%t]", txUuid, txSuccessful)
	if state.currentTxUuid != txUuid {
		panic(fmt.Errorf("Different Uuid in tx-begin [%s] and tx-finish [%s]", state.currentTxUuid, txUuid))
	}
	if txSuccessful {
		if !state.currentTxStateDelta.isEmpty() {
			stateLogger.Debug("txFinish() for txUuid [%s] merging state changes", txUuid)
			state.stateDelta.applyChanges(state.currentTxStateDelta)
			state.recomputeHash = true
			state.txStateDeltaHash[txUuid] = state.currentTxStateDelta.computeCryptoHash()
		} else {
			state.txStateDeltaHash[txUuid] = nil
		}
	}
	state.currentTxStateDelta = newStateDelta()
	state.currentTxUuid = ""
}

func (state *state) txInProgress() bool {
	return state.currentTxUuid != ""
}

// get - get state for chaincodeID and key. If committed is false, this first looks in memory and if missing,
// pulls from db. If committed is true, this pulls from the db only.
func (state *state) get(chaincodeID string, key string, committed bool) ([]byte, error) {
	if !committed {
		valueHolder := state.currentTxStateDelta.get(chaincodeID, key)
		if valueHolder != nil {
			return valueHolder.value, nil
		}
		valueHolder = state.stateDelta.get(chaincodeID, key)
		if valueHolder != nil {
			return valueHolder.value, nil
		}
	}
	return state.fetchStateFromDB(chaincodeID, key)
}

// set - sets state to given value for chaincodeID and key. Does not immideatly writes to memory
func (state *state) set(chaincodeID string, key string, value []byte) error {
	stateLogger.Debug("set() chaincodeID=[%s], key=[%s], value=[%#v]", chaincodeID, key, value)
	if !state.txInProgress() {
		panic("State can be changed only in context of a tx.")
	}
	state.currentTxStateDelta.set(chaincodeID, key, value)
	return nil
}

// delete tracks the deletion of state for chaincodeID and key. Does not immideatly writes to memory
func (state *state) delete(chaincodeID string, key string) error {
	stateLogger.Debug("delete() chaincodeID=[%s], key=[%s]", chaincodeID, key)
	if !state.txInProgress() {
		panic("State can be changed only in context of a tx.")
	}
	state.currentTxStateDelta.delete(chaincodeID, key)
	return nil
}

// getHash computes new state hash if the stateDelta is to be applied.
// Recomputes only if stateDelta has changed after most recent call to this function
func (state *state) getHash() ([]byte, error) {
	if state.recomputeHash {
		stateLogger.Debug("Recomputing state hash...")
		hash, err := computeStateHash(state.stateDelta)
		if err != nil {
			return nil, err
		}
		state.statehash = hash
		state.recomputeHash = false
	}
	return state.statehash.globalHash, nil
}

// clearInMemoryChanges remove from memory all the changes to state
func (state *state) clearInMemoryChanges() {
	state.stateDelta = newStateDelta()
	state.txStateDeltaHash = make(map[string][]byte)
}

// getStateDelta get changes in state after most recent call to method clearInMemoryChanges
func (state *state) getStateDelta() *stateDelta {
	return state.stateDelta
}

// getSnapshot returns a snapshot of the global state for the current block. stateSnapshot.Release()
// must be called once you are done.
func (state *state) getSnapshot() (*stateSnapshot, error) {
	return newStateSnapshot()
}

func fetchStateDeltaFromDB(blockNumber uint64) (*stateDelta, error) {
	stateDeltaBytes, err := db.GetDBHandle().GetFromStateCF(encodeStateDeltaKey(blockNumber))
	if err != nil {
		return nil, err
	}
	if stateDeltaBytes == nil {
		return nil, nil
	}
	stateDelta := newStateDelta()
	stateDelta.Unmarshal(stateDeltaBytes)
	return stateDelta, nil
}

func (state *state) addChangesForPersistence(blockNumber uint64, writeBatch *gorocksdb.WriteBatch) {
	stateLogger.Debug("state.addChangesForPersistence()...start")
	state.stateDelta.addChangesForPersistence(writeBatch)

	serializedStateDelta := state.stateDelta.Marshal()
	cf := db.GetDBHandle().StateCF

	stateLogger.Debug("Adding state-delta corresponding to block number[%d]", blockNumber)
	writeBatch.PutCF(cf, encodeStateDeltaKey(blockNumber), serializedStateDelta)

	if blockNumber >= historyStateDeltaSize {
		blockNumberToDelete := blockNumber - historyStateDeltaSize
		stateLogger.Debug("Deleting state-delta corresponding to block number[%d]", blockNumberToDelete)
		writeBatch.DeleteCF(cf, encodeStateDeltaKey(blockNumberToDelete))
	} else {
		stateLogger.Debug("Not deleting previous state-delta. Block number [%d] is smaller than historyStateDeltaSize [%d]",
			blockNumber, historyStateDeltaSize)
	}

	state.statehash.addChangesForPersistence(writeBatch)
	stateLogger.Debug("state.addChangesForPersistence()...finished")
}

func (state *state) applyStateDelta(delta *stateDelta) error {

	state.stateDelta = delta
	state.recomputeHash = true
	writeBatch := gorocksdb.NewWriteBatch()
	delta.addChangesForPersistence(writeBatch)
	_, err := state.getHash()
	if err != nil {
		return err
	}
	state.statehash.addChangesForPersistence(writeBatch)

	opt := gorocksdb.NewDefaultWriteOptions()
	err = db.GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		return err
	}

	return nil
}

func (state *state) fetchStateFromDB(chaincodeID string, key string) ([]byte, error) {
	return db.GetDBHandle().GetFromStateCF(encodeStateDBKey(chaincodeID, key))
}

// functions for converting keys to byte[] for interacting with rocksdb

var stateKeyDelimiter = []byte{0x00}

func encodeStateDBKey(chaincodeID string, key string) []byte {
	retKey := []byte(chaincodeID)
	retKey = append(retKey, stateKeyDelimiter...)
	keybytes := ([]byte(key))
	retKey = append(retKey, keybytes...)
	return retKey
}

func decodeStateDBKey(dbKey []byte) (string, string) {
	split := bytes.Split(dbKey, stateKeyDelimiter)
	return string(split[0]), string(split[1])
}

func buildLowestStateDBKey(chaincodeID string) []byte {
	retKey := []byte(chaincodeID)
	retKey = append(retKey, stateKeyDelimiter...)
	return retKey
}

var stateDeltaKeyPrefix = byte(0)

func encodeStateDeltaKey(blockNumber uint64) []byte {
	return prependKeyPrefix(stateDeltaKeyPrefix, encodeBlockNumberDBKey(blockNumber))
}

func decodeStateDeltaKey(dbkey []byte) uint64 {
	return decodeBlockNumberDBKey(dbkey[1:])
}
