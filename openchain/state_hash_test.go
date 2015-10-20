package openchain

import (
	"bytes"
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/util"
)

func TestComputeHash_OnlyInMemoryChanges(t *testing.T) {
	initTestDB(t)
	state := GetState()
	state.ClearInMemoryChanges()

	state.Set("chaincode1", "key1", []byte("value1"))
	state.Set("chaincode1", "key2", []byte("value2"))
	state.Set("chaincode2", "key3", []byte("value3"))
	state.Set("chaincode2", "key4", []byte("value4"))
	state.Delete("chaincode2", "key3")
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key4value4"})
}

func TestComputeHash_DBAndInMemoryChanges(t *testing.T) {
	initTestDB(t)
	state := GetState()
	state.ClearInMemoryChanges()

	state.Set("chaincode1", "key1", []byte("value1"))
	state.Set("chaincode1", "key2", []byte("value2"))
	state.Set("chaincode2", "key3", []byte("value3"))
	state.Set("chaincode2", "key4", []byte("value4"))
	saveTestStateDataInDB(t)

	checkCodechainHashInDB(t, "chaincode1", "chaincode1key1value1key2value2")
	checkCodechainHashInDB(t, "chaincode2", "chaincode2key3value3key4value4")
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key3value3key4value4"})

	state.Delete("chaincode2", "key4")
	state.Set("chaincode2", "key5", []byte("value5"))
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key3value3key5value5"})

	saveTestStateDataInDB(t)
	state.Set("chaincode2", "key0", []byte("value0"))
	checkGlobalStateHash(t, []string{"chaincode1key1value1key2value2", "chaincode2key0value0key3value3key5value5"})
}

func checkGlobalStateHash(t *testing.T, expectedHashStr []string) {
	state := GetState()
	stateHash, err := state.GetHash()
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
