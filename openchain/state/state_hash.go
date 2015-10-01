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

package state

import (
	"bytes"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
	"golang.org/x/crypto/sha3"
)

type stateHash struct {
	globalHash  []byte
	updatedHash map[string][]byte
}

func (statehash *stateHash) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	openchainDB := db.GetDBHandle()
	for chaincodeId, updatedhash := range statehash.updatedHash {
		writeBatch.PutCF(openchainDB.StateHashCF, encodeStateHashDBKey(chaincodeId), updatedhash)
	}
}

func computeStateHash(updatedChaincodes map[string]*chaincodeState) (*stateHash, error) {
	openchainDB := db.GetDBHandle()
	statehash := &stateHash{nil, make(map[string][]byte)}
	for chaincodeId, updatedChaincode := range updatedChaincodes {
		updatedHash, err := computeChaincodeHash(updatedChaincode)
		if err != nil {
			return nil, err
		}
		statehash.updatedHash[chaincodeId] = updatedHash
	}
	itr := openchainDB.GetStateHashCFIterator()
	defer itr.Close()
	var buffer bytes.Buffer

	for ; itr.Valid(); itr.Next() {
		chaincodeId := decodeStateHashDBKey(itr.Key().Data())
		chaincodeHash := statehash.updatedHash[chaincodeId]
		if chaincodeHash == nil {
			chaincodeHash = itr.Value().Data()
		}
		buffer.Write(chaincodeHash)
	}
	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, buffer.Bytes())
	statehash.globalHash = hash
	return statehash, nil
}

func computeChaincodeHash(changedChaincode *chaincodeState) ([]byte, error) {
	openchainDB := db.GetDBHandle()
	var buffer bytes.Buffer
	buffer.Write([]byte(changedChaincode.chaincodeId))
	itr := openchainDB.GetStateCFIterator()
	defer itr.Close()
	itr.Seek(buildLowestStateDBKey(changedChaincode.chaincodeId))
	for ; itr.Valid(); itr.Next() {
		chaincodeId, key := decodeStateDBKey(itr.Key().Data())
		if chaincodeId != changedChaincode.chaincodeId {
			break
		}
		updatedValue := changedChaincode.updatedStateMap[key]
		if updatedValue != nil && updatedValue.isDelete() {
			continue
		}
		buffer.Write([]byte(key))
		var value []byte
		if updatedValue != nil {
			value = updatedValue.value
		} else {
			value = itr.Value().Data()
		}
		buffer.Write(value)
	}
	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, buffer.Bytes())
	return hash, nil
}

func encodeStateHashDBKey(chaincodeId string) []byte {
	return []byte(chaincodeId)
}

func decodeStateHashDBKey(dbKey []byte) string {
	return string(dbKey)
}
