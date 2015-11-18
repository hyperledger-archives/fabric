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
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/util"
)

func TestComputeHash_OnlyInMemoryChanges(t *testing.T) {
	initTestDB(t)
	state := getState()
	state.clearInMemoryChanges()
	state.txBegin("txUuid")
	state.set("chaincode1", "key1", []byte("value1"))
	state.set("chaincode1", "key2", []byte("value2"))
	state.set("chaincode2", "key3", []byte("value3"))
	state.set("chaincode2", "key4", []byte("value4"))
	state.delete("chaincode2", "key3")
	state.txFinish("txUuid", true)
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key4value4"})
}

func TestComputeHash_DBAndInMemoryChanges(t *testing.T) {
	initTestDB(t)
	state := getState()
	state.clearInMemoryChanges()

	state.txBegin("txUuid")
	state.set("chaincode1", "key1", []byte("value1"))
	state.set("chaincode1", "key2", []byte("value2"))
	state.set("chaincode2", "key3", []byte("value3"))
	state.set("chaincode2", "key4", []byte("value4"))
	state.txFinish("txUuid", true)
	saveTestStateDataInDB(t)

	checkCodechainHashInDB(t, "chaincode1", "chaincode1key1value1key2value2")
	checkCodechainHashInDB(t, "chaincode2", "chaincode2key3value3key4value4")
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key3value3key4value4"})

	state.txBegin("txUuid")
	state.delete("chaincode2", "key4")
	state.set("chaincode2", "key5", []byte("value5"))
	state.txFinish("txUuid", true)

	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key3value3key5value5"})
	saveTestStateDataInDB(t)

	state.txBegin("txUuid")
	state.set("chaincode2", "key0", []byte("value0"))
	state.txFinish("txUuid", true)

	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key0value0key3value3key5value5"})
}

func checkGlobalStateHash(t *testing.T, expectedHashStr []string) {
	state := getState()
	stateHash, err := state.getHash()
	if err != nil {
		t.Fatalf("Error while getting hash. error = [%s]", err)
	}
	expectedHash := computeExpectedGlobalHash(expectedHashStr)
	if !bytes.Equal(stateHash, expectedHash) {
		t.Fatalf("Hashes not same. Expected hash = [%x], Result hash = [%x]", expectedHash, stateHash)
	}
}

func checkCodechainHashInDB(t *testing.T, codechainID string, expectedHashStr string) {
	savedCodechainHash := fetchCodechainHashFromDB(t, codechainID)
	expectedCodechainHash := computeExpectedChaincodeHash(expectedHashStr)
	if bytes.Compare(savedCodechainHash, expectedCodechainHash) != 0 {
		t.Fatalf("Codechain hash for [%s] save in db not correct. Saved value = [%x], expected value = [%x]",
			codechainID, savedCodechainHash, expectedCodechainHash)
	}
}

func fetchCodechainHashFromDB(t *testing.T, codechainID string) []byte {
	codechainHashInDb, err := db.GetDBHandle().GetFromStateHashCF([]byte(codechainID))
	if err != nil {
		t.Fatal("Error while fetching codechain hash from db")
	}
	return codechainHashInDb
}

func computeExpectedGlobalHash(hashStr []string) []byte {
	var globalHashBuffer bytes.Buffer
	for i := range hashStr {
		globalHashBuffer.Write(computeExpectedChaincodeHash(hashStr[i]))
	}
	return util.ComputeCryptoHash(globalHashBuffer.Bytes())
}

func computeExpectedChaincodeHash(hashStr string) []byte {
	return util.ComputeCryptoHash([]byte(hashStr))
}
