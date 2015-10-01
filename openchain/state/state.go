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

package state

import (
	"bytes"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

// State structure for maintaining world state. This is not thread safe
type State struct {
	chaincodeStateMap map[string]*chaincodeState
	statehash         *stateHash
}

var stateInstance *State

// GetState get handle to world state
func GetState() *State {
	if stateInstance == nil {
		state := new(State)
		state.chaincodeStateMap = make(map[string]*chaincodeState)
	}
	return stateInstance
}

// Get get state for chaincodeId and key. This first looks in memory and if missing, pulls from db
func (state *State) Get(chaincodeId string, key string) ([]byte, error) {
	updatedValue := state.chaincodeStateMap[chaincodeId].get(key)
	if updatedValue != nil {
		return updatedValue.value, nil
	}
	return state.fetchStateFromDB(chaincodeId, key)
}

// Set sets state to given value for chaincodeId and key. Does not immideatly writes to memory
func (state *State) Set(chaincodeId string, key string, value []byte) error {
	chaincodeState := getOrCreateChaincodeState(chaincodeId)
	chaincodeState.set(key, value)
	return nil
}

// Delete tracks the deletion of state for chaincodeId and key. Does not immideatly writes to memory
func (state *State) Delete(chaincodeId string, key string) error {
	chaincodeState := getOrCreateChaincodeState(chaincodeId)
	chaincodeState.remove(key)
	return nil
}

// GetUpdates get changes in state after most recent call to method ClearInMemoryChanges
func (state *State) GetUpdates() map[string]*chaincodeState {
	return state.chaincodeStateMap
}

// GetStateHash computes, if not already done, world state hash
func (state *State) GetStateHash() ([]byte, error) {
	if state.statehash == nil {
		hash, err := computeStateHash(state.chaincodeStateMap)
		if err != nil {
			return nil, err
		}
		state.statehash = hash
	}
	return state.statehash.globalHash, nil
}

// ClearInMemoryChanges remove from memory all the changes to state
func (state *State) ClearInMemoryChanges() {
	state.chaincodeStateMap = make(map[string]*chaincodeState)
	state.statehash = nil
}

func (state *State) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	openChainDB := db.GetDBHandle()
	for _, chaincodeState := range state.chaincodeStateMap {
		for key, updatedValue := range chaincodeState.updatedStateMap {
			dbKey := encodeStateDBKey(chaincodeState.chaincodeId, key)
			value := updatedValue.value
			if value != nil {
				writeBatch.PutCF(openChainDB.StateCF, dbKey, value)
			} else {
				writeBatch.DeleteCF(openChainDB.StateCF, dbKey)
			}
		}
	}
	state.statehash.addChangesForPersistence(writeBatch)
}

func getOrCreateChaincodeState(chaincodeId string) *chaincodeState {
	chaincodeState := stateInstance.chaincodeStateMap[chaincodeId]
	if chaincodeState == nil {
		chaincodeState = newChaincodeState(chaincodeId)
		stateInstance.chaincodeStateMap[chaincodeId] = chaincodeState
	}
	return chaincodeState
}

func (state *State) fetchStateFromDB(chaincodeId string, key string) ([]byte, error) {
	return db.GetDBHandle().GetFromStateCF(encodeStateDBKey(chaincodeId, key))
}

// functions for converting keys to byte[] for interacting with rocksdb

func encodeStateDBKey(chaincodeId string, key string) []byte {
	retKey := []byte(chaincodeId)
	retKey = append(retKey, stateKeyDelimiter...)
	keybytes := ([]byte(key))
	retKey = append(retKey, keybytes...)
	return retKey
}

func decodeStateDBKey(dbKey []byte) (string, string) {
	split := bytes.Split(dbKey, stateKeyDelimiter)
	return string(split[0]), string(split[1])
}

func buildLowestStateDBKey(chaincodeId string) []byte {
	retKey := []byte(chaincodeId)
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
	chaincodeId     string
	updatedStateMap map[string]*valueHolder
}

func newChaincodeState(chaincodeId string) *chaincodeState {
	return &chaincodeState{chaincodeId, make(map[string]*valueHolder)}
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
