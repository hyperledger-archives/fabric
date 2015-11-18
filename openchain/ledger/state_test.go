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
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

func TestStateChanges(t *testing.T) {
	initTestDB(t)
	state := getState()
	saveTestStateDataInDB(t)

	// add keys
	state.txBegin("txUuid")
	state.set("chaincode1", "key1", []byte("value1"))
	state.set("chaincode1", "key2", []byte("value2"))
	state.txFinish("txUuid", true)
	//chehck in-memory
	checkStateViaInterface(t, "chaincode1", "key1", "value1")

	// save to db
	saveTestStateDataInDB(t)

	// check from db
	checkStateInDB(t, "chaincode1", "key1", "value1")

	inMemoryState := state.stateDelta.chaincodeStateDeltas["chaincode1"]
	if inMemoryState != nil {
		t.Fatalf("In-memory state should be empty here")
	}

	state.txBegin("txUuid")
	// make changes when data is already in db
	state.set("chaincode1", "key1", []byte("new_value1"))
	state.txFinish("txUuid", true)
	checkStateViaInterface(t, "chaincode1", "key1", "new_value1")

	state.txBegin("txUuid")
	state.delete("chaincode1", "key2")
	state.txFinish("txUuid", true)
	checkStateViaInterface(t, "chaincode1", "key2", "")

	state.txBegin("txUuid")
	state.set("chaincode2", "key3", []byte("value3"))
	state.set("chaincode2", "key4", []byte("value4"))
	state.txFinish("txUuid", true)

	saveTestStateDataInDB(t)
	checkStateInDB(t, "chaincode1", "key1", "new_value1")
	checkStateInDB(t, "chaincode1", "key2", "")
	checkStateInDB(t, "chaincode2", "key3", "value3")
}

func TestStateTxBehavior(t *testing.T) {
	initTestDB(t)
	state := getState()
	if state.txInProgress() {
		t.Fatalf("No tx should be reported to be in progress")
	}

	// set state in a successful tx
	state.txBegin("txUuid")
	state.set("chaincode1", "key1", []byte("value1"))
	state.set("chaincode2", "key2", []byte("value2"))
	checkStateViaInterface(t, "chaincode1", "key1", "value1")
	state.txFinish("txUuid", true)
	checkStateViaInterface(t, "chaincode1", "key1", "value1")

	// set state in a failed tx
	state.txBegin("txUuid1")
	state.set("chaincode1", "key1", []byte("value1_1"))
	state.set("chaincode2", "key2", []byte("value2_1"))
	checkStateViaInterface(t, "chaincode1", "key1", "value1_1")
	state.txFinish("txUuid1", false)
	//older state should be available
	checkStateViaInterface(t, "chaincode1", "key1", "value1")

	// delete state in a successful tx
	state.txBegin("txUuid2")
	state.delete("chaincode1", "key1")
	checkStateViaInterface(t, "chaincode1", "key1", "")
	state.txFinish("txUuid2", true)
	checkStateViaInterface(t, "chaincode1", "key1", "")

	// delete state in a failed tx
	state.txBegin("txUuid2")
	state.delete("chaincode2", "key2")
	checkStateViaInterface(t, "chaincode2", "key2", "")
	state.txFinish("txUuid2", false)
	checkStateViaInterface(t, "chaincode2", "key2", "value2")
}

func TestStateTxWrongCallCausePanic_1(t *testing.T) {
	initTestDB(t)
	state := getState()
	defer panicRecoveringFunc(t, "A panic should occur when a set state is invoked with out calling a tx-begin")
	state.set("chaincodeID1", "key1", []byte("value1"))
}

func TestStateTxWrongCallCausePanic_2(t *testing.T) {
	initTestDB(t)
	state := getState()
	defer panicRecoveringFunc(t, "A panic should occur when a tx-begin is invoked before tx-finish for on-going tx")
	state.txBegin("txUuid")
	state.txBegin("anotherUuid")
}

func TestStateTxWrongCallCausePanic_3(t *testing.T) {
	initTestDB(t)
	state := getState()
	defer panicRecoveringFunc(t, "A panic should occur when Uuid for tx-begin and tx-finish ends")
	state.txBegin("txUuid")
	state.txFinish("anotherUuid", true)
}

func panicRecoveringFunc(t *testing.T, msg string) {
	x := recover()
	if x == nil {
		t.Fatal(msg)
	} else {
		t.Logf("A panic was caught successfully. Actual msg = %s", x)
	}
}

func checkStateInDB(t *testing.T, chaincodeID string, key string, expectedValue string) {
	checkState(t, chaincodeID, key, expectedValue, true)
}

func checkStateViaInterface(t *testing.T, chaincodeID string, key string, expectedValue string) {
	checkState(t, chaincodeID, key, expectedValue, false)
}

func checkState(t *testing.T, chaincodeID string, key string, expectedValue string, fetchFromDB bool) {
	var value []byte
	if fetchFromDB {
		value = fetchStateFromDB(t, chaincodeID, key)
	} else {
		value = fetchStateViaInterface(t, chaincodeID, key)
	}
	if expectedValue == "" {
		if value != nil {
			t.Fatalf("Value expected 'nil', value found = [%s]", string(value))
		}
	} else if string(value) != expectedValue {
		t.Fatalf("Value expected = [%s], value found = [%s]", expectedValue, string(value))
	}
}

func fetchStateFromDB(t *testing.T, chaincodeID string, key string) []byte {
	value, err := db.GetDBHandle().GetFromStateCF(encodeStateDBKey(chaincodeID, key))
	if err != nil {
		t.Fatalf("Error in fetching state from db for chaincode=[%s], key=[%s], error=[%s]", chaincodeID, key, err)
	}
	return value
}

func fetchStateViaInterface(t *testing.T, chaincodeID string, key string) []byte {
	state := getState()
	value, err := state.get(chaincodeID, key, false)
	if err != nil {
		t.Fatalf("Error while fetching state for chaincode=[%s], key=[%s], error=[%s]", chaincodeID, key, err)
	}
	return value
}

func saveTestStateDataInDB(t *testing.T) {
	writeBatch := gorocksdb.NewWriteBatch()
	state := getState()
	state.getHash()
	state.addChangesForPersistence(0, writeBatch)
	opt := gorocksdb.NewDefaultWriteOptions()
	err := db.GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		t.Fatalf("failed to persist state to db")
	}
	state.clearInMemoryChanges()
}
