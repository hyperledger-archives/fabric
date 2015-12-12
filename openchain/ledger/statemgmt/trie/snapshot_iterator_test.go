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

package trie

import (
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"testing"
)

func TestStateSnapshotIterator(t *testing.T) {
	testDBWrapper.CreateFreshDB(t)
	stateTrieTestWrapper := newStateTrieTestWrapper(t)
	stateTrie := stateTrieTestWrapper.stateTrie
	stateDelta := statemgmt.NewStateDelta()

	// insert keys
	stateDelta.Set("chaincodeID1", "key1", []byte("value1"))
	stateDelta.Set("chaincodeID2", "key2", []byte("value2"))
	stateDelta.Set("chaincodeID3", "key3", []byte("value3"))
	stateDelta.Set("chaincodeID4", "key4", []byte("value4"))
	stateDelta.Set("chaincodeID5", "key5", []byte("value5"))
	stateDelta.Set("chaincodeID6", "key6", []byte("value6"))
	stateTrie.PrepareWorkingSet(stateDelta)
	stateTrieTestWrapper.PersistChangesAndResetInMemoryChanges()
	//check that the key is persisted
	testutil.AssertEquals(t, stateTrieTestWrapper.Get("chaincodeID5", "key5"), []byte("value5"))

	// take db snapeshot
	dbSnapshot := db.GetDBHandle().GetSnapshot()

	// delete keys
	stateDelta.Delete("chaincodeID1", "key1")
	stateDelta.Delete("chaincodeID2", "key2")
	stateDelta.Delete("chaincodeID3", "key3")
	stateDelta.Delete("chaincodeID4", "key4")
	stateDelta.Delete("chaincodeID5", "key5")
	stateDelta.Delete("chaincodeID6", "key6")
	stateTrie.PrepareWorkingSet(stateDelta)
	stateTrieTestWrapper.PersistChangesAndResetInMemoryChanges()
	//check that the key is deleted
	testutil.AssertNil(t, stateTrieTestWrapper.Get("chaincodeID5", "key5"))

	itr, err := newStateSnapshotIterator(dbSnapshot)
	testutil.AssertNoError(t, err, "Error while getting state snapeshot iterator")
	numKeys := 0
	for itr.Next() {
		key, value := itr.GetRawKeyValue()
		t.Logf("key=[%s], value=[%s]", string(key), string(value))
		numKeys++
	}
	testutil.AssertEquals(t, numKeys, 6)
}
