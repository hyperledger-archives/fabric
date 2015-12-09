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

package statemgmt

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/util"
)

// StateDelta holds the changes to existing state. This struct is used for holding the uncommited changes during execution of a tx-batch
// Also, to be used for transferring the state to another peer in chunks
type StateDelta struct {
	chaincodeStateDeltas map[string]*chaincodeStateDelta
}

// NewStateDelta constructs an empty StateDelta struct
func NewStateDelta() *StateDelta {
	return &StateDelta{make(map[string]*chaincodeStateDelta)}
}

// Get get the state from delta if exists
func (stateDelta *StateDelta) Get(chaincodeID string, key string) *UpdatedValue {
	chaincodeStateDelta, ok := stateDelta.chaincodeStateDeltas[chaincodeID]
	if ok {
		updatedValue := chaincodeStateDelta.get(key)
		if updatedValue != nil {
			return updatedValue
		}
	}
	return nil
}

// Set sets state value for a key
func (stateDelta *StateDelta) Set(chaincodeID string, key string, value []byte) {
	chaincodeStateDelta := stateDelta.getOrCreateChaincodeStateDelta(chaincodeID)
	chaincodeStateDelta.set(key, value)
	return
}

// Delete deletes a key from the state
func (stateDelta *StateDelta) Delete(chaincodeID string, key string) {
	chaincodeStateDelta := stateDelta.getOrCreateChaincodeStateDelta(chaincodeID)
	chaincodeStateDelta.remove(key)
	return
}

// ApplyChanges merges another delta - if a key is present in both, the value of the existing key is overwritten
func (stateDelta *StateDelta) ApplyChanges(anotherStateDelta *StateDelta) {
	for chaincodeID, chaincodeStateDelta := range anotherStateDelta.chaincodeStateDeltas {
		for key, valueHolder := range chaincodeStateDelta.updatedKVs {
			if valueHolder.IsDelete() {
				stateDelta.Delete(chaincodeID, key)
			} else {
				stateDelta.Set(chaincodeID, key, valueHolder.value)
			}
		}
	}
}

// IsEmpty checks whether StateDelta contains any data
func (stateDelta *StateDelta) IsEmpty() bool {
	return len(stateDelta.chaincodeStateDeltas) == 0
}

// GetUpdatedChaincodeIds return the chaincodeIDs that are prepsent in the delta
// If sorted is true, the method return chaincodeIDs in lexicographical sorted order
func (stateDelta *StateDelta) GetUpdatedChaincodeIds(sorted bool) []string {
	updatedChaincodeIds := make([]string, len(stateDelta.chaincodeStateDeltas))
	i := 0
	for k := range stateDelta.chaincodeStateDeltas {
		updatedChaincodeIds[i] = k
		i++
	}
	if sorted {
		sort.Strings(updatedChaincodeIds)
	}
	return updatedChaincodeIds
}

// GetUpdates returns changes associated with given chaincodeId
func (stateDelta *StateDelta) GetUpdates(chaincodeID string) map[string]*UpdatedValue {
	chaincodeStateDelta := stateDelta.chaincodeStateDeltas[chaincodeID]
	if chaincodeStateDelta == nil {
		return nil
	}
	return chaincodeStateDelta.updatedKVs
}

func (stateDelta *StateDelta) getOrCreateChaincodeStateDelta(chaincodeID string) *chaincodeStateDelta {
	chaincodeStateDelta, ok := stateDelta.chaincodeStateDeltas[chaincodeID]
	if !ok {
		chaincodeStateDelta = newChaincodeStateDelta(chaincodeID)
		stateDelta.chaincodeStateDeltas[chaincodeID] = chaincodeStateDelta
	}
	return chaincodeStateDelta
}

// ComputeCryptoHash computes crypto-hash for the data held
// returns nil if no data is present
func (stateDelta *StateDelta) ComputeCryptoHash() []byte {
	if stateDelta.IsEmpty() {
		return nil
	}
	var buffer bytes.Buffer
	sortedChaincodeIds := stateDelta.GetUpdatedChaincodeIds(true)
	for _, chaincodeID := range sortedChaincodeIds {
		buffer.WriteString(chaincodeID)
		chaincodeStateDelta := stateDelta.chaincodeStateDeltas[chaincodeID]
		sortedKeys := chaincodeStateDelta.getSortedKeys()
		for _, key := range sortedKeys {
			buffer.WriteString(key)
			updatedValue := chaincodeStateDelta.get(key)
			if !updatedValue.IsDelete() {
				buffer.Write(updatedValue.value)
			}
		}
	}
	hashingContent := buffer.Bytes()
	logger.Debug("computing hash on %#v", hashingContent)
	return util.ComputeCryptoHash(hashingContent)
}

// Code below is for maintaining state for a chaincode
type chaincodeStateDelta struct {
	chaincodeID string
	updatedKVs  map[string]*UpdatedValue
}

func newChaincodeStateDelta(chaincodeID string) *chaincodeStateDelta {
	return &chaincodeStateDelta{chaincodeID, make(map[string]*UpdatedValue)}
}

func (chaincodeStateDelta *chaincodeStateDelta) get(key string) *UpdatedValue {
	return chaincodeStateDelta.updatedKVs[key]
}

func (chaincodeStateDelta *chaincodeStateDelta) set(key string, value []byte) {
	chaincodeStateDelta.updatedKVs[key] = &UpdatedValue{value}
}

func (chaincodeStateDelta *chaincodeStateDelta) remove(key string) {
	chaincodeStateDelta.updatedKVs[key] = &UpdatedValue{nil}
}

func (chaincodeStateDelta *chaincodeStateDelta) hasChanges() bool {
	return len(chaincodeStateDelta.updatedKVs) > 0
}

func (chaincodeStateDelta *chaincodeStateDelta) getSortedKeys() []string {
	updatedKeys := []string{}
	for k := range chaincodeStateDelta.updatedKVs {
		updatedKeys = append(updatedKeys, k)
	}
	sort.Strings(updatedKeys)
	logger.Debug("Sorted keys = %#v", updatedKeys)
	return updatedKeys
}

// UpdatedValue hods the value for a key
type UpdatedValue struct {
	value []byte
}

// IsDelete checks whether the key was deleted
func (updatedValue *UpdatedValue) IsDelete() bool {
	return updatedValue.value == nil
}

// GetValue returns the value
func (updatedValue *UpdatedValue) GetValue() []byte {
	return updatedValue.value
}

// marshalling / Unmarshalling code
// We need to revisit the following when we define proto messages
// for state related structures for transporting. May be we can
// completely get rid of custom marshalling / Unmarshalling of a state delta

// Marshal serializes the StateDelta
func (stateDelta *StateDelta) Marshal() (b []byte) {
	buffer := proto.NewBuffer([]byte{})
	err := buffer.EncodeVarint(uint64(len(stateDelta.chaincodeStateDeltas)))
	if err != nil {
		// in protobuf code the error return is always nil
		panic(fmt.Errorf("This error should not occure: %s", err))
	}
	for chaincodeID, chaincodeStateDelta := range stateDelta.chaincodeStateDeltas {
		buffer.EncodeStringBytes(chaincodeID)
		chaincodeStateDelta.marshal(buffer)
	}
	b = buffer.Bytes()
	return
}

func (chaincodeStateDelta *chaincodeStateDelta) marshal(buffer *proto.Buffer) {
	err := buffer.EncodeVarint(uint64(len(chaincodeStateDelta.updatedKVs)))
	if err != nil {
		// in protobuf code the error return is always nil
		panic(fmt.Errorf("This error should not occure: %s", err))
	}
	for key, valueHolder := range chaincodeStateDelta.updatedKVs {
		err = buffer.EncodeStringBytes(key)
		if err != nil {
			return
		}
		err = buffer.EncodeRawBytes(valueHolder.value)
		if err != nil {
			// in protobuf code the error return is always nil
			panic(fmt.Errorf("This error should not occure: %s", err))
		}
	}
	return
}

// Unmarshal deserializes StateDelta
func (stateDelta *StateDelta) Unmarshal(bytes []byte) {
	buffer := proto.NewBuffer(bytes)
	size, err := buffer.DecodeVarint()
	if err != nil {
		panic(fmt.Errorf("This error should not occure: %s", err))
	}
	stateDelta.chaincodeStateDeltas = make(map[string]*chaincodeStateDelta, size)
	for i := uint64(0); i < size; i++ {
		chaincodeID, err := buffer.DecodeStringBytes()
		if err != nil {
			panic(fmt.Errorf("This error should not occure: %s", err))
		}
		chaincodeStateDelta := newChaincodeStateDelta(chaincodeID)
		err = chaincodeStateDelta.unmarshal(buffer)
		if err != nil {
			panic(fmt.Errorf("This error should not occure: %s", err))
		}
		stateDelta.chaincodeStateDeltas[chaincodeID] = chaincodeStateDelta
	}
}

func (chaincodeStateDelta *chaincodeStateDelta) unmarshal(buffer *proto.Buffer) error {
	size, err := buffer.DecodeVarint()
	if err != nil {
		panic(fmt.Errorf("This error should not occure: %s", err))
	}
	chaincodeStateDelta.updatedKVs = make(map[string]*UpdatedValue, size)
	for i := uint64(0); i < size; i++ {
		key, err := buffer.DecodeStringBytes()
		if err != nil {
			panic(fmt.Errorf("This error should not occure: %s", err))
		}
		value, err := buffer.DecodeRawBytes(false)
		if err != nil {
			panic(fmt.Errorf("This error should not occure: %s", err))
		}

		// protobuff does not differentiate between an empty []byte or a nil
		// For now we assume user does not have a motivation to store []byte array
		// as a value for a key and we treat an empty []byte represent that the value was nil
		// during marshalling (i.e., the entry represent a delete of a key)
		// If we need to differentiate, we need to write a flag during marshalling
		//(which would require one bool per keyvalue entry)
		if len(value) == 0 {
			value = nil
		}
		chaincodeStateDelta.updatedKVs[key] = &UpdatedValue{value}
	}
	return nil
}
