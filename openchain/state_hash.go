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
	"sort"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/tecbot/gorocksdb"
)

var stateHashLogger = logging.MustGetLogger("stateHash")

type stateHash struct {
	globalHash  []byte
	updatedHash map[string][]byte
}

func newStatehash() *stateHash {
	sh := &stateHash{nil, make(map[string][]byte)}
	return sh
}

func (statehash *stateHash) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	stateHashLogger.Debug("Adding changes for state hash")
	openchainDB := db.GetDBHandle()
	for chaincodeID, updatedhash := range statehash.updatedHash {
		stateHashLogger.Debug("Adding changed hash for chaincode = [%s]", chaincodeID)
		writeBatch.PutCF(openchainDB.StateHashCF, encodeStateHashDBKey(chaincodeID), updatedhash)
	}
	stateHashLogger.Debug("Finished adding changes for state hash")
}

// computeStateHash computes hash of bytes obtained by iterating over db and updated chaincodes.
// Add to buffer hash for each chaincode in the sorted order of chaincodeID e.g., chaincodeHash1 + chaincodeHash2 + ...
func computeStateHash(stateDelta *stateDelta) (*stateHash, error) {
	stateHashLogger.Debug("Computing global state hash")
	openchainDB := db.GetDBHandle()
	statehash := newStatehash()

	// first compute hashes for updated chaincodes
	for chaincodeID, updatedChaincode := range stateDelta.chaincodeStateDeltas {
		updatedHash, err := computeChaincodeHash(updatedChaincode)
		if err != nil {
			return nil, err
		}
		statehash.updatedHash[chaincodeID] = updatedHash
	}

	itr := openchainDB.GetStateHashCFIterator()
	itr.SeekToFirst()
	defer itr.Close()
	var buffer bytes.Buffer

	updatedChaincodeIDs := getSortedMapKeys2(statehash.updatedHash)
	index := 0

	// iterate over db and changed chaincodes
	for itr.Valid() && index < len(updatedChaincodeIDs) {
		dbChaincodeID := decodeStateHashDBKey(itr.Key().Data())
		updatedChaincodeID := updatedChaincodeIDs[index]

		var finalKey string
		var finalValue []byte

		comparisonResult := compareStrings(dbChaincodeID, updatedChaincodeID)
		switch comparisonResult {
		case -1:
			finalKey = dbChaincodeID
			finalValue = itr.Value().Data()
			itr.Next()
			stateHashLogger.Debug("Selected chaincode [%s] from db", finalKey)
		case 0, 1:
			finalKey = updatedChaincodeID
			finalValue = statehash.updatedHash[updatedChaincodeID]
			index++
			if comparisonResult == 0 {
				itr.Next()
			}
			stateHashLogger.Debug("Selected chaincode [%s] from changed chaincode", finalKey)
		}
		buffer.Write(finalValue)
	}

	if index < len(updatedChaincodeIDs) {
		// add remaining key, values from updated chaincodes
		stateHashLogger.Debug("Adding remaining updated chaincodes")
		for i := index; i < len(updatedChaincodeIDs); i++ {
			updatedChaincodeID := updatedChaincodeIDs[i]
			stateHashLogger.Debug("Adding updated chaincode [%s]", updatedChaincodeID)
			buffer.Write(statehash.updatedHash[updatedChaincodeID])
		}
	} else {
		// add remaining key values from db
		stateHashLogger.Debug("Adding remaining chaincode hashes from db")
		for itr.Valid() {
			stateHashLogger.Debug("Adding chaincode hash from db for chaincode = [%s]", string(itr.Key().Data()))
			buffer.Write(itr.Value().Data())
			itr.Next()
		}
	}
	statehash.globalHash = util.ComputeCryptoHash(buffer.Bytes())
	return statehash, nil
}

// computeChaincodeHash computes hash of bytes obtained by "chaincodeID + key1 + value1 + key2 + value2 +..."
// iterates over db + keys in the updated map of changedChaincode (sort order of keys is combined both from db and updated map)
func computeChaincodeHash(changedChaincode *chaincodeStateDelta) ([]byte, error) {
	stateHashLogger.Debug("Computing state hash for chaincodeID", changedChaincode.chaincodeID)
	openchainDB := db.GetDBHandle()
	var buffer bytes.Buffer

	// add chaincodeID
	stateHashLogger.Debug("Adding chaincodeID = [%s]", changedChaincode.chaincodeID)
	buffer.Write([]byte(changedChaincode.chaincodeID))

	// obtain db iterator
	itr := openchainDB.GetStateCFIterator()
	defer itr.Close()
	itr.Seek(buildLowestStateDBKey(changedChaincode.chaincodeID))

	// obtain sorted keys of update map
	updatedKeys := getSortedMapKeys1(changedChaincode.updatedKVs)

	updatedKeysIndex := 0
	itrNext := true
	var dbChaincodeID, dbKey string

	// loop until one of the db iterator and update keys are exhausted
	for itr.Valid() && updatedKeysIndex < len(updatedKeys) {
		// check for just to avoid unnecessary decoding if iterator has not moved
		if itrNext {
			dbChaincodeID, dbKey = decodeStateDBKey(itr.Key().Data())
		}
		if dbChaincodeID != changedChaincode.chaincodeID {
			break
		}
		updatedKey := updatedKeys[updatedKeysIndex]

		// final key, value to be considered for state hash
		var finalKey string
		var finalValue []byte
		comparisonResult := compareStrings(dbKey, updatedKey)
		switch comparisonResult {
		case -1:
			finalKey = dbKey
			finalValue = itr.Value().Data()
			itr.Next()
			itrNext = true
			stateHashLogger.Debug("Selected db key = [%s]", finalKey)
		case 0, 1:
			finalKey = updatedKey
			updatedValue := changedChaincode.updatedKVs[updatedKey]
			if !updatedValue.isDelete() {
				finalValue = updatedValue.value
				stateHashLogger.Debug("Selected updated key = [%s]", finalKey)
			} else {
				stateHashLogger.Debug("Skipping updated key = [%s]", finalKey)
			}
			updatedKeysIndex++
			itrNext = false
			if comparisonResult == 0 {
				itr.Next()
				itrNext = true
			}
		}
		if finalValue != nil {
			stateHashLogger.Debug("Adding selected key = [%s]", finalKey)
			buffer.Write([]byte(finalKey))
			buffer.Write(finalValue)
		}
	}

	if updatedKeysIndex < len(updatedKeys) {
		// add remaining keys from update map
		stateHashLogger.Debug("Adding remaining updated keys")
		for ; updatedKeysIndex < len(updatedKeys); updatedKeysIndex++ {
			updatedKey := updatedKeys[updatedKeysIndex]
			updatedValue := changedChaincode.updatedKVs[updatedKey]
			if !updatedValue.isDelete() {
				stateHashLogger.Debug("Adding updated key = [%s]", updatedKey)
				buffer.Write([]byte(updatedKey))
				buffer.Write(updatedValue.value)
			} else {
				stateHashLogger.Debug("Skipping updated key = [%s]", updatedKey)
			}
		}
	} else {
		// add remaining keys from db
		stateHashLogger.Debug("Adding remaining keys from db")
		dbChaincodeID, dbKey = decodeStateDBKey(itr.Key().Data())
		for itr.Valid() && dbChaincodeID == changedChaincode.chaincodeID {
			dbChaincodeID, dbKey = decodeStateDBKey(itr.Key().Data())
			stateHashLogger.Debug("Adding from db key = [%s]", dbKey)
			buffer.Write([]byte(dbKey))
			buffer.Write(itr.Value().Data())
			itr.Next()
		}
	}
	stateHashLogger.Debug("Finished computing state hash for chaincodeID", changedChaincode.chaincodeID)
	return util.ComputeCryptoHash(buffer.Bytes()), nil
}

func getSortedMapKeys1(dict map[string]*valueHolder) []string {
	keys := make([]string, len(dict))
	i := 0
	for k := range dict {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func getSortedMapKeys2(dict map[string][]byte) []string {
	keys := make([]string, len(dict))
	i := 0
	for k := range dict {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func encodeStateHashDBKey(chaincodeID string) []byte {
	return []byte(chaincodeID)
}

func decodeStateHashDBKey(dbKey []byte) string {
	return string(dbKey)
}

func compareStrings(str1 string, str2 string) int {
	return bytes.Compare([]byte(str1), []byte(str2))
}
