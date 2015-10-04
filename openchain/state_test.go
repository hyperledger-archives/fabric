package openchain

import (
	"os"
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

func TestStateChanges(t *testing.T) {
	initTestDB(t)
	state := GetState()
	saveTestStateDataInDB(t)

	// add keys
	state.Set("chaincode1", "key1", []byte("value1"))
	state.Set("chaincode1", "key2", []byte("value2"))

	//chehck in-memory
	checkStateViaInterface(t, "chaincode1", "key1", "value1")

	// save to db
	saveTestStateDataInDB(t)

	// check from db
	checkStateInDB(t, "chaincode1", "key1", "value1")

	inMemoryState := state.chaincodeStateMap["chaincode1"]
	if inMemoryState != nil {
		t.Fatalf("In-memory state should be empty here")
	}

	// make changes when data is already in db
	state.Set("chaincode1", "key1", []byte("new_value1"))
	checkStateViaInterface(t, "chaincode1", "key1", "new_value1")

	state.Delete("chaincode1", "key2")
	checkStateViaInterface(t, "chaincode1", "key2", "")
	state.Set("chaincode2", "key3", []byte("value3"))
	state.Set("chaincode2", "key4", []byte("value4"))

	saveTestStateDataInDB(t)
	checkStateInDB(t, "chaincode1", "key1", "new_value1")
	checkStateInDB(t, "chaincode1", "key2", "")
	checkStateInDB(t, "chaincode2", "key3", "value3")
}

func checkStateInDB(t *testing.T, chaincodeID string, key string, expectedValue string) {
	checkState(t, chaincodeID, key, expectedValue, true)
}

func checkStateViaInterface(t *testing.T, chaincodeID string, key string, expectedValue string) {
	checkState(t, chaincodeID, key, expectedValue, false)
}

func checkState(t *testing.T, chaincodeID string, key string, expectedValue string, fetchFromDB bool) {
	var value []byte
	if fetchFromDB {
		value = fetchStateFromDB(t, chaincodeID, key)
	} else {
		value = fetchStateViaInterface(t, chaincodeID, key)
	}
	if expectedValue == "" {
		if value != nil {
			t.Fatalf("Value expected 'nil', value found = [%s]", string(value))
		}
	} else if string(value) != expectedValue {
		t.Fatalf("Value expected = [%s], value found = [%s]", expectedValue, string(value))
	}
}

func fetchStateFromDB(t *testing.T, chaincodeID string, key string) []byte {
	value, err := db.GetDBHandle().GetFromStateCF(encodeStateDBKey(chaincodeID, key))
	if err != nil {
		t.Fatalf("Error in fetching state from db for chaincode=[%s], key=[%s], error=[%s]", chaincodeID, key, err)
	}
	return value
}

func fetchStateViaInterface(t *testing.T, chaincodeID string, key string) []byte {
	state := GetState()
	value, err := state.Get(chaincodeID, key)
	if err != nil {
		t.Fatalf("Error while fetching state for chaincode=[%s], key=[%s], error=[%s]", chaincodeID, key, err)
	}
	return value
}

// db helper functions
var testDBCreated bool

func initTestDB(t *testing.T) {
	if testDBCreated {
		db.GetDBHandle().CloseDB()
	}
	removeTestDBPath()
	err := db.CreateDB()
	if err != nil {
		t.Fatalf("Error in creating test db. Error = [%s]", err)
	}
	testDBCreated = true
}

func removeTestDBPath() {
	dbPath := viper.GetString("peer.db.path")
	os.RemoveAll(dbPath)
}

func saveTestStateDataInDB(t *testing.T) {
	writeBatch := gorocksdb.NewWriteBatch()
	state := GetState()
	state.GetHash()
	state.addChangesForPersistence(writeBatch)
	opt := gorocksdb.NewDefaultWriteOptions()
	err := db.GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		t.Fatalf("failed to persist state to db")
	}
	state.ClearInMemoryChanges()
}
