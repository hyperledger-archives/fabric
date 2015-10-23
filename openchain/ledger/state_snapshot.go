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
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

type stateSnapshot struct {
	blockNumber uint64
	iterator    *gorocksdb.Iterator
	snapshot    *gorocksdb.Snapshot
}

// newStateSnapshot creates a new snapshot of the global state for the current block.
func newStateSnapshot() (*stateSnapshot, error) {
	snapshot := db.GetDBHandle().GetSnapshot()
	blockNumberBytes, err := db.GetDBHandle().GetFromBlockchainCFSnapshot(snapshot, []byte("blockCount"))
	if err != nil {
		snapshot.Release()
		return nil, err
	}
	stateIterator := db.GetDBHandle().GetStateCFSnapshotIterator(snapshot)

	var blockNumber uint64 = 0
	if blockNumberBytes != nil {
		blockNumber = decodeToUint64(blockNumberBytes)
	} else {
		blockNumber = 0
	}
	stateSnapshot := &stateSnapshot{blockNumber, stateIterator, snapshot}

	stateSnapshot.iterator.SeekToFirst()
	return stateSnapshot, nil
}

// Release the snapshot. This MUST be called when you are done with this resouce.
func (stateSnapshot *stateSnapshot) Release() {
	stateSnapshot.iterator.Close()
	stateSnapshot.snapshot.Release()
}

// Next moves the iterator to the next key/value pair in the state
func (stateSnapshot *stateSnapshot) Next() bool {
	stateSnapshot.iterator.Next()
	return stateSnapshot.iterator.Valid()
}

// GetRawKeyValue returns the raw bytes for the key and value at the current iterator position
func (stateSnapshot *stateSnapshot) GetRawKeyValue() ([]byte, []byte) {
	return stateSnapshot.iterator.Key().Data(), stateSnapshot.iterator.Value().Data()
}

// GetBlockNumber returns the blocknumber associated with this global state snapshot
func (stateSnapshot *stateSnapshot) GetBlockNumber() uint64 {
	return stateSnapshot.blockNumber
}
