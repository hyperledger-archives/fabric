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
	"os"
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
)

// InitTestLedger exported for testing from other packages
func InitTestLedger(t *testing.T) *Ledger {
	initTestBlockChain(t)
	ledger := getLedger(t)
	ledger.currentID = nil
	return ledger
}

///////////////////////////
// Test db creation and cleanup functions
var performTestDBCleanup bool

func initTestDB(t *testing.T) {
	// cleaning up test db here so that each test does not have to call it explicitly
	// at the end of the test
	cleanupTestDB()
	removeTestDBPath()
	err := db.CreateDB()
	if err != nil {
		t.Fatalf("Error in creating test db. Error = [%s]", err)
	}
	performTestDBCleanup = true
}

func cleanupTestDB() {
	if performTestDBCleanup {
		db.GetDBHandle().CloseDB()
		performTestDBCleanup = false
	}
}

func removeTestDBPath() {
	dbPath := viper.GetString("peer.db.path")
	os.RemoveAll(dbPath)
}

////////////////////////////////////////////////////
//  test block chain creation and cleanup functions
var performTestBlockchainCleanup bool

func initTestBlockChain(t *testing.T) *Blockchain {
	// cleaning up blockchain instance for test here so that each test does
	// not have to call it explicitly at the end of the test
	cleanupTestBlockchain(t)
	initTestDB(t)
	chain := getTestBlockchain(t)
	err := chain.init()
	if err != nil {
		t.Fatalf("Error during initializing block chain. Error: %s", err)
	}
	t.Logf("Reinitialized Blockchain for testing.....")
	GetState().ClearInMemoryChanges()
	performTestBlockchainCleanup = true
	return chain
}

func cleanupTestBlockchain(t *testing.T) {
	if performTestBlockchainCleanup {
		t.Logf("Cleaning up previously created blockchain for testing.....")
		chain := getTestBlockchain(t)
		if chain.indexer != nil {
			chain.indexer.stop()
		}
		chain.size = 0
		chain.previousBlockHash = []byte{}
		performTestBlockchainCleanup = false
	}
}

func generateUUID(t *testing.T) string {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Error generating UUID: %s", err)
	}
	return uuid
}

func getLedger(t *testing.T) *Ledger {
	ledger, err := GetLedger()
	if err != nil {
		t.Fatalf("Error while creating a test ledger: %s", err)
	}
	return ledger
}

func getStateFromLedger(t *testing.T, chaincodeID string, key string) []byte {
	value, err := getLedger(t).GetState(chaincodeID, key)
	if err != nil {
		t.Fatalf("Error while getting value from test ledger: %s", err)
	}
	return value
}

func beginTxBatch(t *testing.T, id interface{}) {
	err := getLedger(t).BeginTxBatch(id)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func commitTxBatch(t *testing.T, id interface{}, transactions []*protos.Transaction, proof []byte) {
	err := getLedger(t).CommitTxBatch(id, transactions, proof)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func rollbackTxBatch(t *testing.T, id interface{}) {
	err := getLedger(t).RollbackTxBatch(id)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func getTestBlockchain(t *testing.T) *Blockchain {
	chain, err := GetBlockchain()
	if err != nil {
		t.Fatalf("Error while getting handle to chain. [%s]", err)
	}
	return chain
}
