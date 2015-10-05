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
	chaincodeStateMap map[string]*chaincodeState
	statehash         *stateHash
	recomputeHash     bool
}

var stateLogger = logging.MustGetLogger("state")
var stateInstance *State

// GetState get handle to world state
func GetState() *State {
	if stateInstance == nil {
		stateInstance = &State{make(map[string]*chaincodeState), nil, true}
	}
	return stateInstance
}

// Get get state for chaincodeID and key. This first looks in memory and if missing, pulls from db
func (state *State) Get(chaincodeID string, key string) ([]byte, error) {
	chaincodeState := state.chaincodeStateMap[chaincodeID]
	if chaincodeState != nil {
		updatedValue := state.chaincodeStateMap[chaincodeID].get(key)
		if updatedValue != nil {
			return updatedValue.value, nil
		}
	}
	return state.fetchStateFromDB(chaincodeID, key)
}

// Set sets state to given value for chaincodeID and key. Does not immideatly writes to memory
func (state *State) Set(chaincodeID string, key string, value []byte) error {
	chaincodeState := getOrCreateChaincodeState(chaincodeID)
	chaincodeState.set(key, value)
	state.recomputeHash = true
	return nil
}

// Delete tracks the deletion of state for chaincodeID and key. Does not immideatly writes to memory
func (state *State) Delete(chaincodeID string, key string) error {
	chaincodeState := getOrCreateChaincodeState(chaincodeID)
	chaincodeState.remove(key)
	state.recomputeHash = true
	return nil
}

// GetHash computes state hash, if called for first time or state has changed after most recent call to this function
func (state *State) GetHash() ([]byte, error) {
	if state.recomputeHash {
		stateLogger.Debug("Recomputing state hash...")
		hash, err := computeStateHash(state.chaincodeStateMap)
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
	state.chaincodeStateMap = make(map[string]*chaincodeState)
}

// getUpdates get changes in state after most recent call to method ClearInMemoryChanges
func (state *State) getUpdates() map[string]*chaincodeState {
	return state.chaincodeStateMap
}

func (state *State) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	stateLogger.Debug("addChangesForPersistence()...start")
	for _, chaincodeState := range state.chaincodeStateMap {
		addChaincodeStateForPersistence(writeBatch, chaincodeState)
	}
	state.statehash.addChangesForPersistence(writeBatch)
	stateLogger.Debug("addChangesForPersistence()...finished")
}

func addChaincodeStateForPersistence(writeBatch *gorocksdb.WriteBatch, chaincodeState *chaincodeState) {
	stateLogger.Debug("addChaincodeStateForPersistence() for codechainId = [%s]", chaincodeState.chaincodeID)
	openChainDB := db.GetDBHandle()
	for key, updatedValue := range chaincodeState.updatedStateMap {
		dbKey := encodeStateDBKey(chaincodeState.chaincodeID, key)
		value := updatedValue.value
		if value != nil {
			writeBatch.PutCF(openChainDB.StateCF, dbKey, value)
		} else {
			writeBatch.DeleteCF(openChainDB.StateCF, dbKey)
		}
	}
	stateLogger.Debug("addChaincodeStateForPersistence() for codechainId = [%s]", chaincodeState.chaincodeID)
}

func getOrCreateChaincodeState(chaincodeID string) *chaincodeState {
	chaincodeState := stateInstance.chaincodeStateMap[chaincodeID]
	if chaincodeState == nil {
		chaincodeState = newChaincodeState(chaincodeID)
		stateInstance.chaincodeStateMap[chaincodeID] = chaincodeState
	}
	return chaincodeState
}

func (state *State) fetchStateFromDB(chaincodeID string, key string) ([]byte, error) {
	return db.GetDBHandle().GetFromStateCF(encodeStateDBKey(chaincodeID, key))
}

// functions for converting keys to byte[] for interacting with rocksdb

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

var stateKeyDelimiter = []byte{0x00}

// Code below is for maintaining state for a chaincode

type valueHolder struct {
	value []byte
}

func (valueHolder *valueHolder) isDelete() bool {
	return valueHolder.value == nil
}

type chaincodeState struct {
	chaincodeID     string
	updatedStateMap map[string]*valueHolder
}

func newChaincodeState(chaincodeID string) *chaincodeState {
	return &chaincodeState{chaincodeID, make(map[string]*valueHolder)}
}

func (chaincodeState *chaincodeState) get(key string) *valueHolder {
	return chaincodeState.updatedStateMap[key]
}

func (chaincodeState *chaincodeState) set(key string, value []byte) {
	chaincodeState.updatedStateMap[key] = &valueHolder{value}
}

func (chaincodeState *chaincodeState) remove(key string) {
	chaincodeState.updatedStateMap[key] = &valueHolder{nil}
}

func (chaincodeState *chaincodeState) changed() bool {
	return len(chaincodeState.updatedStateMap) > 0
}
