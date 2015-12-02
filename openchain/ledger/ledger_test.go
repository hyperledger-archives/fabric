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
	"os"
	"reflect"
	"testing"

	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func TestLedgerCommit(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	commitTxBatch(t, 1, []*protos.Transaction{transaction}, []byte("prrof"))
	if !reflect.DeepEqual(getStateFromLedger(t, "chaincode1", "key1"), []byte("value1")) {
		t.Fatalf("state value not same after Tx commit")
	}
}

func TestLedgerRollback(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	rollbackTxBatch(t, 1)

	valueAfterRollback := getStateFromLedger(t, "chaincode1", "key1")

	if valueAfterRollback != nil {
		t.Logf("Value after rollback = [%s]", valueAfterRollback)
		t.Fatalf("state value not nil after Tx rollback")
	}
}

func TestLedgerDifferentID(t *testing.T) {
	ledger := InitTestLedger(t)
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	err := ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, []byte("prrof"))
	if err == nil {
		t.Fatalf("ledger should throw error")
	}
}

func TestLedgerGetTempStateHashWithTxDeltaStateHashes(t *testing.T) {
	ledger := InitTestLedger(t)
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid1")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.TxFinished("txUuid1", true)

	ledger.TxBegin("txUuid2")
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.TxFinished("txUuid2", true)

	ledger.TxBegin("txUuid3")
	ledger.TxFinished("txUuid3", true)

	ledger.TxBegin("txUuid4")
	ledger.SetState("chaincode4", "key4", []byte("value4"))
	ledger.TxFinished("txUuid4", false)

	_, txDeltaHashes, _ := ledger.GetTempStateHashWithTxDeltaStateHashes()
	checkStateDeltaHash(t, "chaincode1key1value1", txDeltaHashes["txUuid1"])
	checkStateDeltaHash(t, "chaincode2key2value2", txDeltaHashes["txUuid2"])
	checkStateDeltaHash(t, "", txDeltaHashes["txUuid3"])
	_, ok := txDeltaHashes["txUuid4"]
	if ok {
		t.Fatalf("Entry for a failed Tx should not be present in txDeltaHashes map")
	}
	ledger.CommitTxBatch(1, []*protos.Transaction{}, []byte("proof"))

	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid1")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.TxFinished("txUuid1", true)
	_, txDeltaHashes, _ = ledger.GetTempStateHashWithTxDeltaStateHashes()
	if len(txDeltaHashes) != 1 {
		t.Fatalf("Entries in txDeltaHashes map should only be from current batch")
	}
}

func TestStateSnapshot(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	commitTxBatch(t, 1, []*protos.Transaction{transaction}, []byte("proof"))

	snapshot, err := ledger.GetStateSnapshot()

	if err != nil {
		t.Fatalf("Error fetching snapshot %s", err)
	}

	defer snapshot.Release()

	// Modify keys to ensure they do not impact the snapshot
	beginTxBatch(t, 2)
	ledger.TxBegin("txUuid")
	ledger.DeleteState("chaincode1", "key1")
	ledger.SetState("chaincode4", "key4", []byte("value4"))
	ledger.SetState("chaincode5", "key5", []byte("value5"))
	ledger.SetState("chaincode6", "key6", []byte("value6"))
	ledger.TxFinished("txUuid", true)
	transaction, _ = buildTestTx()
	commitTxBatch(t, 2, []*protos.Transaction{transaction}, []byte("proof"))

	var count = 0
	for snapshot.Next() {
		k, v := snapshot.GetRawKeyValue()
		t.Logf("Key %v, Val %v", k, v)
		count++
	}
	if count != 3 {
		t.Fatalf("Expected 3 keys, but got %d", count)
	}

	if snapshot.GetBlockNumber() != 1 {
		t.Fatalf("Expected blocknumber to be 1, but got %d", snapshot.GetBlockNumber())
	}

}

func TestSetRawState(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	commitTxBatch(t, 1, []*protos.Transaction{transaction}, []byte("prrof"))

	// Ensure values are in the DB
	val, err := ledger.GetState("chaincode1", "key1", true)
	if bytes.Compare(val, []byte("value1")) != 0 {
		t.Fatalf("Expected initial chaincode1 key1 to be %s, but got %s", []byte("value1"), val)
	}
	val, err = ledger.GetState("chaincode2", "key2", true)
	if bytes.Compare(val, []byte("value2")) != 0 {
		t.Fatalf("Expected initial chaincode1 key2 to be %s, but got %s", []byte("value2"), val)
	}
	val, err = ledger.GetState("chaincode3", "key3", true)
	if bytes.Compare(val, []byte("value3")) != 0 {
		t.Fatalf("Expected initial chaincode1 key3 to be %s, but got %s", []byte("value3"), val)
	}

	snapshot, snapshotError := ledger.GetStateSnapshot()
	if snapshotError != nil {
		t.Fatalf("Error fetching snapshot %s", snapshotError)
	}
	defer snapshot.Release()

	// Delete keys
	beginTxBatch(t, 2)
	ledger.TxBegin("txUuid")
	ledger.DeleteState("chaincode1", "key1")
	ledger.DeleteState("chaincode2", "key2")
	ledger.DeleteState("chaincode3", "key3")
	ledger.TxFinished("txUuid", true)
	transaction, _ = buildTestTx()
	commitTxBatch(t, 2, []*protos.Transaction{transaction}, []byte("proof"))

	// ensure keys are deleted
	val, err = ledger.GetState("chaincode1", "key1", true)
	if val != nil {
		t.Fatalf("Expected chaincode1 key1 to be nil, but got %s", val)
	}
	val, err = ledger.GetState("chaincode2", "key2", true)
	if val != nil {
		t.Fatalf("Expected chaincode2 key2 to be nil, but got %s", val)
	}
	val, err = ledger.GetState("chaincode3", "key3", true)
	if val != nil {
		t.Fatalf("Expected chaincode3 key3 to be nil, but got %s", val)
	}

	// put key/values from the snapshot back in the DB
	//var keys, values [][]byte
	delta := newStateDelta()
	for i := 0; snapshot.Next(); i++ {
		k, v := snapshot.GetRawKeyValue()
		cID, kID := decodeStateDBKey(k)
		delta.set(cID, kID, v)
	}

	err = ledger.ApplyRawStateDelta(delta)
	if err != nil {
		t.Fatalf("Error applying raw state delta, %s", err)
	}

	// Ensure values are back in the DB
	val, err = ledger.GetState("chaincode1", "key1", true)
	if bytes.Compare(val, []byte("value1")) != 0 {
		t.Fatalf("Expected chaincode1 key1 to be %s, but got %s", []byte("value1"), val)
	}
	val, err = ledger.GetState("chaincode2", "key2", true)
	if bytes.Compare(val, []byte("value2")) != 0 {
		t.Fatalf("Expected chaincode1 key2 to be %s, but got %s", []byte("value2"), val)
	}
	val, err = ledger.GetState("chaincode3", "key3", true)
	if bytes.Compare(val, []byte("value3")) != 0 {
		t.Fatalf("Expected chaincode1 key3 to be %s, but got %s", []byte("value3"), val)
	}

}

func setupTestConfig() {
	viper.AddConfigPath(".")
	viper.SetConfigName("ledger_test")
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}
