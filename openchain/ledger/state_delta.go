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
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/tecbot/gorocksdb"
)

type stateDelta struct {
	chaincodeStateDeltas map[string]*chaincodeStateDelta
}

func newStateDelta() *stateDelta {
	return &stateDelta{make(map[string]*chaincodeStateDelta)}
}

func (stateDelta *stateDelta) get(chaincodeID string, key string) *valueHolder {
	chaincodeStateDelta, ok := stateDelta.chaincodeStateDeltas[chaincodeID]
	if ok {
		updatedValue := chaincodeStateDelta.get(key)
		if updatedValue != nil {
			return updatedValue
		}
	}
	return nil
}

func (stateDelta *stateDelta) set(chaincodeID string, key string, value []byte) {
	chaincodeStateDelta := stateDelta.getOrCreateChaincodeStateDelta(chaincodeID)
	chaincodeStateDelta.set(key, value)
	return
}

func (stateDelta *stateDelta) delete(chaincodeID string, key string) {
	chaincodeStateDelta := stateDelta.getOrCreateChaincodeStateDelta(chaincodeID)
	chaincodeStateDelta.remove(key)
	return
}

func (stateDelta *stateDelta) applyChanges(anotherStateDelta *stateDelta) {
	for chaincodeID, chaincodeStateDelta := range anotherStateDelta.chaincodeStateDeltas {
		for key, valueHolder := range chaincodeStateDelta.updatedKVs {
			if valueHolder.isDelete() {
				stateDelta.delete(chaincodeID, key)
			} else {
				stateDelta.set(chaincodeID, key, valueHolder.value)
			}
		}
	}
}

func (stateDelta *stateDelta) isEmpty() bool {
	return len(stateDelta.chaincodeStateDeltas) == 0
}

func (stateDelta *stateDelta) getOrCreateChaincodeStateDelta(chaincodeID string) *chaincodeStateDelta {
	chaincodeStateDelta, ok := stateDelta.chaincodeStateDeltas[chaincodeID]
	if !ok {
		chaincodeStateDelta = newChaincodeStateDelta(chaincodeID)
		stateDelta.chaincodeStateDeltas[chaincodeID] = chaincodeStateDelta
	}
	return chaincodeStateDelta
}

func (stateDelta *stateDelta) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	stateLogger.Debug("stateDelta.addChangesForPersistence()...start")
	for _, chaincodeStateDelta := range stateDelta.chaincodeStateDeltas {
		chaincodeStateDelta.addChangesForPersistence(writeBatch)
	}
	stateLogger.Debug("stateDelta.addChangesForPersistence()...finished")
}

func (stateDelta *stateDelta) computeCryptoHash() []byte {
	var buffer bytes.Buffer
	//sort chaincodeIDs
	sortedChaincodeIds := stateDelta.getSortedChaincodeIDs()
	for _, chaincodeID := range sortedChaincodeIds {
		buffer.WriteString(chaincodeID)
		chaincodeStateDelta := stateDelta.chaincodeStateDeltas[chaincodeID]
		sortedKeys := chaincodeStateDelta.getSortedKeys()
		for _, key := range sortedKeys {
			buffer.WriteString(key)
			updatedValue := chaincodeStateDelta.get(key)
			if !updatedValue.isDelete() {
				buffer.Write(updatedValue.value)
			}
		}
	}
	hashingContent := buffer.Bytes()
	stateLogger.Debug("computing hash on %#v", hashingContent)
	return util.ComputeCryptoHash(hashingContent)
}

func (stateDelta *stateDelta) getSortedChaincodeIDs() []string {
	chaincodeIDs := []string{}
	for k, _ := range stateDelta.chaincodeStateDeltas {
		chaincodeIDs = append(chaincodeIDs, k)
	}
	sort.Strings(chaincodeIDs)
	stateLogger.Debug("sorted chaincodeIDs = %#v", chaincodeIDs)
	return chaincodeIDs
}

// Code below is for maintaining state for a chaincode
type valueHolder struct {
	value []byte
}

func (valueHolder *valueHolder) isDelete() bool {
	return valueHolder.value == nil
}

type chaincodeStateDelta struct {
	chaincodeID string
	updatedKVs  map[string]*valueHolder
}

func newChaincodeStateDelta(chaincodeID string) *chaincodeStateDelta {
	return &chaincodeStateDelta{chaincodeID, make(map[string]*valueHolder)}
}

func (chaincodeStateDelta *chaincodeStateDelta) get(key string) *valueHolder {
	return chaincodeStateDelta.updatedKVs[key]
}

func (chaincodeStateDelta *chaincodeStateDelta) set(key string, value []byte) {
	chaincodeStateDelta.updatedKVs[key] = &valueHolder{value}
}

func (chaincodeStateDelta *chaincodeStateDelta) remove(key string) {
	chaincodeStateDelta.updatedKVs[key] = &valueHolder{nil}
}

func (chaincodeStateDelta *chaincodeStateDelta) hasChanges() bool {
	return len(chaincodeStateDelta.updatedKVs) > 0
}

func (chaincodeStateDelta *chaincodeStateDelta) getSortedKeys() []string {
	updatedKeys := []string{}
	for k, _ := range chaincodeStateDelta.updatedKVs {
		updatedKeys = append(updatedKeys, k)
	}
	sort.Strings(updatedKeys)
	stateLogger.Debug("Sorted keys = %#v", updatedKeys)
	return updatedKeys
}

func (chaincodeStateDelta *chaincodeStateDelta) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	stateLogger.Debug("stating chaincodeStateDelta.addChangesForPersistence() for codechainId = [%s]", chaincodeStateDelta.chaincodeID)
	openChainDB := db.GetDBHandle()
	for key, updatedValue := range chaincodeStateDelta.updatedKVs {
		dbKey := encodeStateDBKey(chaincodeStateDelta.chaincodeID, key)
		if !updatedValue.isDelete() {
			writeBatch.PutCF(openChainDB.StateCF, dbKey, updatedValue.value)
		} else {
			writeBatch.DeleteCF(openChainDB.StateCF, dbKey)
		}
	}
	stateLogger.Debug("finished chaincodeStateDelta.addChangesForPersistence() for codechainId = [%s]", chaincodeStateDelta.chaincodeID)
}

// marshalling / Unmarshalling code
// We need to revisit the following when we define proto messages
// for state related structures for transporting. May be we can
// completely get rid of custom marshalling / Unmarshalling of a state delta
func (stateDelta *stateDelta) marshal() (b []byte) {
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

func (stateDelta *stateDelta) unmarshal(bytes []byte) {
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
	chaincodeStateDelta.updatedKVs = make(map[string]*valueHolder, size)
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
		chaincodeStateDelta.updatedKVs[key] = &valueHolder{value}
	}
	return nil
}
