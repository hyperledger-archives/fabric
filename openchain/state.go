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

// State is the state of all contracts after running the transactions
// for all blocks in the blockchain.
type State struct {
	db StateDB
}

// NewState creates a new empty state.
func NewState(statePath string, createIfMissing bool) (*State, error) {
	state := new(State)
	db, err := OpenStateDB(statePath, createIfMissing)
	if err != nil {
		return nil, err
	}
	state.db = *db
	return state, nil
}

// Put a key-value pair for the given contract ID
func (state *State) Put(contractID string, key, value []byte) error {
	err := state.db.Put(contractID, key, value)
	if err != nil {
		return err
	}
	return nil
}

// Get a value for the given contract ID and key
func (state *State) Get(contractID string, key []byte) ([]byte, error) {
	value, err := state.db.Get(contractID, key)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete a key-value pair for a given contract ID and key
func (state *State) Delete(contractID string, key []byte) error {
	err := state.db.Delete(contractID, key)
	if err != nil {
		return err
	}
	return nil
}

// GetHash returns the hash of the entire state. This can be used to compare
// the current state to value returned by block.GetStateHash().
func (state *State) GetHash() []byte {
	return state.db.GetHash()
}
