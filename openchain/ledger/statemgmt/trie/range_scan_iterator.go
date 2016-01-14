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
	"bytes"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/openchain/ledger/util"
	"github.com/tecbot/gorocksdb"
)

type RangeScanIterator struct {
	dbItr        *gorocksdb.Iterator
	startKey     []byte
	endKey       []byte
	currentKey   []byte
	currentValue []byte
	done         bool
}

func newRangeScanIterator(chaincodeID string, startKey string, endKey string) (*RangeScanIterator, error) {
	dbItr := db.GetDBHandle().GetStateCFIterator()
	encodedStartKey := newTrieKey(chaincodeID, startKey).getEncodedBytes()
	encodedEndKey := newTrieKey(chaincodeID, endKey).getEncodedBytes()
	dbItr.Seek(encodedStartKey)
	return &RangeScanIterator{dbItr, encodedStartKey, encodedEndKey, nil, nil, false}, nil
}

func (rangeScanItr *RangeScanIterator) Next() bool {
	if rangeScanItr.done {
		return false
	}
	var available bool
	for ; rangeScanItr.dbItr.Valid(); rangeScanItr.dbItr.Next() {
		trieKeyBytes := rangeScanItr.dbItr.Key().Data()
		trieNodeBytes := rangeScanItr.dbItr.Value().Data()
		value := unmarshalTrieNodeValue(trieNodeBytes)
		if util.NotNil(value) {
			rangeScanItr.currentKey = trieKeyEncoderImpl.decodeTrieKeyBytes(statemgmt.Copy(trieKeyBytes))
			rangeScanItr.currentValue = value
			if bytes.Compare(rangeScanItr.currentKey, rangeScanItr.endKey) == 1 {
				available = false
				rangeScanItr.done = true
				break
			}
			available = true
			rangeScanItr.dbItr.Next()
			break
		}
	}
	return available
}

// GetRawKeyValue - see interface 'statemgmt.StateIterator' for details
func (rangeScanItr *RangeScanIterator) GetKeyValue() (string, []byte) {
	_, key := statemgmt.DecodeCompositeKey(rangeScanItr.currentKey)
	return key, rangeScanItr.currentValue
}

// Close - see interface 'statemgmt.StateIterator' for details
func (rangeScanItr *RangeScanIterator) Close() {
	rangeScanItr.dbItr.Close()
}
