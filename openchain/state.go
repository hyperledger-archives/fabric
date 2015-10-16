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

package openchain

import (
	"bytes"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

// State structure for maintaining world state. This is not thread safe
type State struct {
	stateDelta    *stateDelta
	statehash     *stateHash
	recomputeHash bool
}

var stateLogger = logging.MustGetLogger("state")

// should this be configurable in yaml?
var historyStateDeltaSize = uint64(500)
var stateInstance *State

// GetState get handle to world state
func GetState() *State {
	if stateInstance == nil {
		stateInstance = &State{newStateDelta(), nil, true}
	}
	return stateInstance
}

// Get get state for chaincodeID and key. This first looks in memory and if missing, pulls from db
func (state *State) Get(chaincodeID string, key string) ([]byte, error) {
	valueHolder := state.stateDelta.get(chaincodeID, key)
	if valueHolder != nil {
		return valueHolder.value, nil
	}
	return state.fetchStateFromDB(chaincodeID, key)
}

// Set sets state to given value for chaincodeID and key. Does not immideatly writes to memory
func (state *State) Set(chaincodeID string, key string, value []byte) error {
	state.stateDelta.set(chaincodeID, key, value)
	state.recomputeHash = true
	return nil
}

// Delete tracks the deletion of state for chaincodeID and key. Does not immideatly writes to memory
func (state *State) Delete(chaincodeID string, key string) error {
	state.stateDelta.delete(chaincodeID, key)
	state.recomputeHash = true
	return nil
}

// GetHash computes state hash, if called for first time or state has changed after most recent call to this function
func (state *State) GetHash() ([]byte, error) {
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

// ClearInMemoryChanges remove from memory all the changes to state
func (state *State) ClearInMemoryChanges() {
	state.stateDelta = newStateDelta()
}

// getUpdates get changes in state after most recent call to method ClearInMemoryChanges
func (state *State) getStateDelta() *stateDelta {
	return state.stateDelta
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
	stateDelta.unmarshal(stateDeltaBytes)
	return stateDelta, nil
}

func (state *State) addChangesForPersistence(blockNumber uint64, writeBatch *gorocksdb.WriteBatch) error {
	stateLogger.Debug("state.addChangesForPersistence()...start")
	state.stateDelta.addChangesForPersistence(writeBatch)

	serializedStateDelta, err := state.stateDelta.marshal()
	if err != nil {
		return err
	}
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
	return nil
}

func (state *State) fetchStateFromDB(chaincodeID string, key string) ([]byte, error) {
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
