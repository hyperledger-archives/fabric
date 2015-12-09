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
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"github.com/openblockchain/obc-peer/protos"
	"testing"
)

func TestLedgerCommit(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	ledger.CommitTxBatch(1, []*protos.Transaction{transaction}, []byte("proof"))
	testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode1", "key1", false), []byte("value1"))
	testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode1", "key1", true), []byte("value1"))
}

func TestLedgerRollback(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	ledger.RollbackTxBatch(1)
	testutil.AssertNil(t, ledgerTestWrapper.GetState("chaincode1", "key1", false))
}

func TestLedgerDifferentID(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	err := ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, []byte("prrof"))
	testutil.AssertError(t, err, "ledger should throw error for wrong batch ID")
}

func TestLedgerGetTempStateHashWithTxDeltaStateHashes(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
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
	testutil.AssertEquals(t, testutil.ComputeCryptoHash([]byte("chaincode1key1value1")), txDeltaHashes["txUuid1"])
	testutil.AssertEquals(t, testutil.ComputeCryptoHash([]byte("chaincode2key2value2")), txDeltaHashes["txUuid2"])
	testutil.AssertNil(t, txDeltaHashes["txUuid3"])
	_, ok := txDeltaHashes["txUuid4"]
	if ok {
		t.Fatalf("Entry for a failed Tx should not be present in txDeltaHashes map")
	}
	ledger.CommitTxBatch(1, []*protos.Transaction{}, []byte("proof"))

	ledger.BeginTxBatch(2)
	ledger.TxBegin("txUuid1")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.TxFinished("txUuid1", true)
	_, txDeltaHashes, _ = ledger.GetTempStateHashWithTxDeltaStateHashes()
	if len(txDeltaHashes) != 1 {
		t.Fatalf("Entries in txDeltaHashes map should only be from current batch")
	}
}

func TestLedgerStateSnapshot(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid", true)
	transaction, _ := buildTestTx()
	ledger.CommitTxBatch(1, []*protos.Transaction{transaction}, []byte("proof"))

	snapshot, err := ledger.GetStateSnapshot()

	if err != nil {
		t.Fatalf("Error fetching snapshot %s", err)
	}
	defer snapshot.Release()

	// Modify keys to ensure they do not impact the snapshot
	ledger.BeginTxBatch(2)
	ledger.TxBegin("txUuid")
	ledger.DeleteState("chaincode1", "key1")
	ledger.SetState("chaincode4", "key4", []byte("value4"))
	ledger.SetState("chaincode5", "key5", []byte("value5"))
	ledger.SetState("chaincode6", "key6", []byte("value6"))
	ledger.TxFinished("txUuid", true)
	transaction, _ = buildTestTx()
	ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, []byte("proof"))

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

func TestLedgerPutRawBlock(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	block := new(protos.Block)
	block.ProposerID = "test"
	block.PreviousBlockHash = []byte("foo")
	block.StateHash = []byte("bar")
	ledger.PutRawBlock(block, 4)
	testutil.AssertEquals(t, ledgerTestWrapper.GetBlockByNumber(4), block)
}

func TestLedgerSetRawState(t *testing.T) {
	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(t)
	ledger := ledgerTestWrapper.ledger
	ledger.BeginTxBatch(1)
	ledger.TxBegin("txUuid1")
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	ledger.TxFinished("txUuid1", true)
	transaction, _ := buildTestTx()
	ledger.CommitTxBatch(1, []*protos.Transaction{transaction}, []byte("proof"))

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

	hash1, hash1Err := ledger.GetTempStateHash()
	if hash1Err != nil {
		t.Fatalf("Error getting hash1 %s", hash1Err)
	}

	snapshot, snapshotError := ledger.GetStateSnapshot()
	if snapshotError != nil {
		t.Fatalf("Error fetching snapshot %s", snapshotError)
	}
	defer snapshot.Release()

	// Delete keys
	ledger.BeginTxBatch(2)
	ledger.TxBegin("txUuid2")
	ledger.DeleteState("chaincode1", "key1")
	ledger.DeleteState("chaincode2", "key2")
	ledger.DeleteState("chaincode3", "key3")
	ledger.TxFinished("txUuid2", true)
	transaction, _ = buildTestTx()
	ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, []byte("proof"))

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

	hash2, hash2Err := ledger.GetTempStateHash()
	if hash2Err != nil {
		t.Fatalf("Error getting hash2 %s", hash2Err)
	}

	if bytes.Compare(hash1, hash2) == 0 {
		t.Fatalf("Expected hashes to not match, but they both equal %s", hash1)
	}

	// put key/values from the snapshot back in the DB
	//var keys, values [][]byte
	delta := statemgmt.NewStateDelta()
	for i := 0; snapshot.Next(); i++ {
		k, v := snapshot.GetRawKeyValue()
		cID, kID := statemgmt.DecodeCompositeKey(k)
		delta.Set(cID, kID, v)
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

	hash3, hash3Err := ledger.GetTempStateHash()
	if hash3Err != nil {
		t.Fatalf("Error getting hash3 %s", hash3Err)
	}
	if bytes.Compare(hash1, hash3) != 0 {
		t.Fatalf("Expected hashes to be equal, but they are %s and %s", hash1, hash3)
	}
}
