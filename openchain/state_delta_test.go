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
	"reflect"
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

func TestStateDeltaMarshalling(t *testing.T) {
	stateDelta := createTestStateDelta()
	by := marshalTestStateDelta(t, stateDelta)
	t.Logf("length of marshalled bytes = [%d]", len(by))
	stateDelta1 := unmarshalTestStateData(t, by)

	if !reflect.DeepEqual(stateDelta, stateDelta1) {
		t.Fatalf("Delta state not same. Found=[%s], Expected=[%s]", stateDelta1, stateDelta)
	}
}

func TestStateDeltaPersistence(t *testing.T) {
	initTestDB(t)
	historyStateDeltaSize = 2
	state := GetState()

	state.ClearInMemoryChanges()
	state.Set("chaincode1", "key1", []byte("value1"))
	state.Set("chaincode2", "key2", []byte("value2"))
	commitTestState(t, 0)

	state.ClearInMemoryChanges()
	state.Set("chaincode1", "key3", []byte("value3"))
	state.Set("chaincode2", "key4", []byte("value4"))
	commitTestState(t, 1)

	state.ClearInMemoryChanges()
	state.Set("chaincode1", "key5", []byte("value5"))
	state.Set("chaincode2", "key6", []byte("value6"))
	commitTestState(t, 2)

	state.ClearInMemoryChanges()
	state.Set("chaincode1", "key7", []byte("value7"))
	state.Set("chaincode2", "key8", []byte("value8"))
	commitTestState(t, 3)

	// state delta for block# 3
	stateDelta := fetchTestStateDeltaFromDB(t, 3)
	if bytes.Compare(stateDelta.get("chaincode1", "key7").value, []byte("value7")) != 0 {
		t.Fatalf("wrong value found in state delta = [%s]", string(stateDelta.get("chaincode1", "key7").value))
	}

	if stateDelta.get("chaincode1", "key5") != nil {
		t.Fatalf("wrong value found in state delta = [%s]", string(stateDelta.get("chaincode1", "key5").value))
	}

	// state delta for block# 2
	stateDelta = fetchTestStateDeltaFromDB(t, 2)
	if bytes.Compare(stateDelta.get("chaincode1", "key5").value, []byte("value5")) != 0 {
		t.Fatalf("wrong value found in state delta = [%s]", string(stateDelta.get("chaincode1", "key5").value))
	}

	// state delta for block# 1
	stateDelta = fetchTestStateDeltaFromDB(t, 1)
	if stateDelta != nil {
		t.Fatalf("state delta should be nil")
	}

	// state delta for block# 0
	stateDelta = fetchTestStateDeltaFromDB(t, 0)
	if stateDelta != nil {
		t.Fatalf("state delta should be nil")
	}
}

func commitTestState(t *testing.T, blockNumber uint64) {
	writeBatch := gorocksdb.NewWriteBatch()
	opts := gorocksdb.NewDefaultWriteOptions()

	_, err := GetState().GetHash()
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	GetState().addChangesForPersistence(blockNumber, writeBatch)

	err = db.GetDBHandle().DB.Write(opts, writeBatch)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func fetchTestStateDeltaFromDB(t *testing.T, blockNumber uint64) *stateDelta {
	stateDelta, err := fetchStateDeltaFromDB(blockNumber)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	return stateDelta
}

func createTestStateDelta() *stateDelta {
	stateDelta := newStateDelta()
	stateDelta.set("chaincode1", "key1", []byte("value1"))
	stateDelta.set("chaincode2", "key2", []byte("value2"))
	stateDelta.delete("chaincode3", "key3")
	return stateDelta
}

func marshalTestStateDelta(t *testing.T, stateDelta *stateDelta) []byte {
	bytes, err := stateDelta.marshal()
	if err != nil {
		t.Fatalf("Error during marshal. Error: %s", err)
	}
	return bytes
}

func unmarshalTestStateData(t *testing.T, bytes []byte) *stateDelta {
	stateDelta := newStateDelta()
	err := stateDelta.unmarshal(bytes)
	if err != nil {
		t.Fatalf("Error during unmarshalling. Error: %s", err)
	}
	return stateDelta
}
