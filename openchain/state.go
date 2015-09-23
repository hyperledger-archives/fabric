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
	"sort"

	"golang.org/x/crypto/sha3"
)

// State is the state of all contracts after running the transactions
// for all blocks in the blockchain.
type State struct {
	// This is a map of contract ID to the state.
	contractIDToState map[string]string
}

// NewState creates a new empty state.
func NewState() *State {
	state := new(State)
	state.contractIDToState = make(map[string]string)
	return state
}

// Update the state of a contract.
func (state *State) Update(contractID string, contractState string) {
	state.contractIDToState[contractID] = contractState
}

// Get the state of a contract.
func (state *State) Get(contractID string) string {
	return state.contractIDToState[contractID]
}

// Delete the state of a contract.
func (state *State) Delete(contractID string) {
	delete(state.contractIDToState, contractID)
}

// GetHash returns the hash of the entire state. This can be used to compare
// the current state to value returned by block.GetStateHash().
func (state *State) GetHash() []byte {
	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, state.Bytes())
	return hash
}

// Bytes returns the state as an array of bytes
func (state *State) Bytes() []byte {
	var buffer bytes.Buffer
	var keyArray []string
	for k := range state.contractIDToState {
		keyArray = append(keyArray, k)
	}
	sort.Strings(keyArray)
	for _, k := range keyArray {
		buffer.WriteString(k)
		buffer.WriteString(state.contractIDToState[k])
	}
	return buffer.Bytes()
}

func (state *State) String() string {
	var buffer bytes.Buffer
	for k, v := range state.contractIDToState {
		buffer.WriteString("\nKey: ")
		buffer.WriteString(k)
		buffer.WriteString("Value: ")
		buffer.WriteString(v)
	}
	return buffer.String()
}
