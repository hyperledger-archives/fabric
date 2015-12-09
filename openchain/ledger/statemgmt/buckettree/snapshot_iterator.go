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

package buckettree

import (
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/tecbot/gorocksdb"
)

type StateSnapshotIterator struct {
	dbItr *gorocksdb.Iterator
}

func newStateSnapshotIterator(snapshot *gorocksdb.Snapshot) (*StateSnapshotIterator, error) {
	dbItr := db.GetDBHandle().GetStateCFSnapshotIterator(snapshot)
	dbItr.Seek([]byte{0x01})
	dbItr.Prev()
	return &StateSnapshotIterator{dbItr}, nil
}

func (snapshotItr *StateSnapshotIterator) Next() bool {
	snapshotItr.dbItr.Next()
	return snapshotItr.dbItr.Valid()
}

func (snapshotItr *StateSnapshotIterator) GetRawKeyValue() ([]byte, []byte) {
	keyBytes := statemgmt.Copy(snapshotItr.dbItr.Key().Data())
	valueBytes := snapshotItr.dbItr.Value().Data()
	dataNode := unmarshalDataNodeFromBytes(keyBytes, valueBytes)
	return dataNode.getCompositeKey(), dataNode.getValue()
}

func (snapshotItr *StateSnapshotIterator) Close() {
	snapshotItr.dbItr.Close()
}
