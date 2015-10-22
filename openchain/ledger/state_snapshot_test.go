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

import "testing"

func TestSnapshot(t *testing.T) {
	initTestDB(t)
	historyStateDeltaSize = 2
	state := getState()

	state.clearInMemoryChanges()
	state.set("chaincode1", "key1", []byte("value1"))
	state.set("chaincode2", "key2", []byte("value2"))
	commitTestState(t, 0)

	state.clearInMemoryChanges()
	state.set("chaincode1", "key3", []byte("value3"))
	state.set("chaincode2", "key4", []byte("value4"))
	commitTestState(t, 1)

	state.clearInMemoryChanges()
	state.set("chaincode1", "key5", []byte("value5"))
	state.set("chaincode2", "key6", []byte("value6"))
	commitTestState(t, 2)

	state.clearInMemoryChanges()
	state.set("chaincode1", "key7", []byte("value7"))
	state.set("chaincode2", "key8", []byte("value8"))
	commitTestState(t, 3)

	snapshot, err := state.getSnapshot()

	if err != nil {
		t.Fatalf("Error fetching snapshot: %s", err)
	}

	defer snapshot.Release()

	var count = 0
	for snapshot.Next() {
		k, v := snapshot.GetRawKeyValue()
		t.Logf("Key %v, Val %v", k, v)
		count++
	}
	if count != 9 {
		t.Fatalf("Expected 9 keys, but got %d", count)
	}
}
