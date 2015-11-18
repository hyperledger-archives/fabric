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
	"testing"

	"google/protobuf"

	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

func TestServerOpenchain_API_GetBlockchainInfo(t *testing.T) {
	// Construct a ledger with 0 blocks.
	ledger.InitTestLedger(t)
	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}
	// Attempt to retrieve the blockchain info. There are no blocks
	// in this blockchain, therefore this test should intentionally fail.
	info, err := server.GetBlockchainInfo(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		// Success
		t.Logf("Error retrieving blockchain info: %s", err)
	} else {
		// Failure
		t.Logf("Error attempting to retrive info from emptry blockchain: %v", info)
		t.Fail()
	}

	// Construct a ledger with 3 blocks.
	ledger1 := ledger.InitTestLedger(t)
	buildTestLedger1(ledger1, t)

	// Attempt to retrieve the blockchain info.
	info, err = server.GetBlockchainInfo(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		t.Logf("Error retrieving blockchain info: %s", err)
		t.Fail()
	} else {
		t.Logf("Blockchain 1 info: %v", info)
	}

	// Construct a ledger with 5 blocks.
	ledger2 := ledger.InitTestLedger(t)
	buildTestLedger2(ledger2, t)

	// Attempt to retrieve the blockchain info.
	info, err = server.GetBlockchainInfo(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		t.Logf("Error retrieving blockchain info: %s", err)
		t.Fail()
	} else {
		t.Logf("Blockchain 2 info: %v", info)
	}
}

func TestServerOpenchain_API_GetBlockByNumber(t *testing.T) {
	// Construct a ledger with 0 blocks.
	ledger.InitTestLedger(t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	// Attempt to retrieve the 0th block from the blockchain. There are no blocks
	// in this blockchain, therefore this test should intentionally fail.

	block, err := server.GetBlockByNumber(context.Background(), &protos.BlockNumber{Number: 0})
	if err != nil {
		// Success
		t.Logf("Error retrieving Block from blockchain: %s", err)
	} else {
		// Failure
		t.Logf("Attempting to retrieve from empty blockchain: %v", block)
		t.Fail()
	}

	// Construct a ledger with 3 blocks.
	ledger1 := ledger.InitTestLedger(t)
	buildTestLedger1(ledger1, t)

	// Retrieve the 0th block from the blockchain.
	block, err = server.GetBlockByNumber(context.Background(), &protos.BlockNumber{Number: 0})
	if err != nil {
		t.Logf("Error retrieving Block from blockchain: %s", err)
		t.Fail()
	} else {
		t.Logf("Block #0: %v", block)
	}

	// Retrieve the 3rd block from the blockchain, blocks are numbered starting
	// from 0.
	block, err = server.GetBlockByNumber(context.Background(), &protos.BlockNumber{Number: 2})
	if err != nil {
		t.Logf("Error retrieving Block from blockchain: %s", err)
		t.Fail()
	} else {
		t.Logf("Block #2: %v", block)
	}

	// Retrieve the 5th block from the blockchain. There are only 3 blocks in this
	// blockchain, therefore this test should intentionally fail.
	block, err = server.GetBlockByNumber(context.Background(), &protos.BlockNumber{Number: 4})
	if err != nil {
		// Success.
		t.Logf("Error retrieving Block from blockchain: %s", err)
	} else {
		// Failure
		t.Logf("Trying to retrieve non-existent block from blockchain: %v", block)
		t.Fail()
	}
}

func TestServerOpenchain_API_GetBlockCount(t *testing.T) {
	// Must initialize the ledger singleton before initializing the
	// OpenchainServer, as it needs that pointer.

	// Construct a ledger with 0 blocks.
	ledger.InitTestLedger(t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	// Retrieve the current number of blocks in the blockchain. There are no blocks
	// in this blockchain, therefore this test should intentionally fail.
	count, err := server.GetBlockCount(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		// Success
		t.Logf("Error retrieving BlockCount from blockchain: %s", err)
	} else {
		// Failure
		t.Logf("Attempting to query an empty blockchain: %v", count.Count)
		t.Fail()
	}

	// Construct a ledger with 3 blocks.
	ledger1 := ledger.InitTestLedger(t)
	buildTestLedger1(ledger1, t)
	// Retrieve the current number of blocks in the blockchain. Must be 3.
	count, err = server.GetBlockCount(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		t.Logf("Error retrieving BlockCount from blockchain: %s", err)
		t.Fail()
	} else if count.Count != 3 {
		t.Logf("Error! Blockchain must have 3 blocks!")
		t.Fail()
	} else {
		t.Logf("Current BlockCount: %v", count.Count)
	}

	// Construct a ledger with 5 blocks.
	ledger2 := ledger.InitTestLedger(t)
	buildTestLedger2(ledger2, t)
	// Retrieve the current number of blocks in the blockchain. Must be 5.
	count, err = server.GetBlockCount(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		t.Logf("Error retrieving BlockCount from blockchain: %s", err)
		t.Fail()
	} else if count.Count != 5 {
		t.Logf("Error! Blockchain must have 5 blocks!")
		t.Fail()
	} else {
		t.Logf("Current BlockCount: %v", count.Count)
	}
}

func TestServerOpenchain_API_GetState(t *testing.T) {
	ledger1 := ledger.InitTestLedger(t)
	// Construct a blockchain with 3 blocks.
	buildTestLedger1(ledger1, t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	// Retrieve the current number of blocks in the blockchain. Must be 3.
	val, stateErr := server.GetState(context.Background(), "MyContract1", "code")
	if stateErr != nil {
		t.Fatalf("Error retrieving state: %s", stateErr)
	} else if bytes.Compare(val, []byte("code example")) != 0 {
		t.Fatalf("Expected %s, but got %s", []byte("code example"), val)
	}

}

// buildTestLedger1 builds a simple ledger data structure that contains a blockchain with 3 blocks.
func buildTestLedger1(ledger1 *ledger.Ledger, t *testing.T) {
	// -----------------------------<Block #0>---------------------
	// Add the 0th (genesis block)
	ledger1.BeginTxBatch(0)
	err := ledger1.CommitTxBatch(0, []*protos.Transaction{}, []byte("dummy-proof"))
	if err != nil {
		t.Fatalf("Error in commit: %s", err)
	}

	// -----------------------------<Block #0>---------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chaincode instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	ledger1.BeginTxBatch(1)
	transaction1a := protos.NewTransaction(protos.ChaincodeID{Url: "Contracts"}, generateUUID(t), "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})
	// VM runs transaction1a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	ledger1.SetState("MyContract1", "code", []byte("code example"))
	ledger1.CommitTxBatch(1, []*protos.Transaction{transaction1a}, []byte("dummy-proof"))
	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	ledger1.BeginTxBatch(2)
	transaction2a := protos.NewTransaction(protos.ChaincodeID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction2b := protos.NewTransaction(protos.ChaincodeID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})

	// Run this transction in the VM. The VM updates the state
	ledger1.SetState("MyContract", "x", []byte("hello"))
	ledger1.SetState("MyOtherContract", "y", []byte("goodbuy"))

	// Commit txbatch that creates the 2nd block on blockchain
	ledger1.CommitTxBatch(2, []*protos.Transaction{transaction2a, transaction2b}, []byte("dummy-proof"))
	// -----------------------------</Block #2>-----------------------------------
	return
}

// buildTestLedger2 builds a simple ledger data structure that contains a blockchain
// of 5 blocks, with each block containing the same number of transactions as its
// index within the blockchain. Block 0, 0 transactions. Block 1, 1 transaction,
// and so on.
func buildTestLedger2(ledger *ledger.Ledger, t *testing.T) {
	// -----------------------------<Block #0>---------------------
	// Add the 0th (genesis block)
	ledger.BeginTxBatch(0)
	ledger.CommitTxBatch(0, []*protos.Transaction{}, []byte("dummy-proof"))
	// -----------------------------<Block #0>---------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chaincode instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	ledger.BeginTxBatch(1)
	transaction1a := protos.NewTransaction(protos.ChaincodeID{Url: "Contracts"}, generateUUID(t), "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})
	// VM runs transaction1a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	ledger.SetState("MyContract1", "code", []byte("code example"))
	ledger.CommitTxBatch(1, []*protos.Transaction{transaction1a}, []byte("dummy-proof"))

	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	ledger.BeginTxBatch(2)
	transaction2a := protos.NewTransaction(protos.ChaincodeID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction2b := protos.NewTransaction(protos.ChaincodeID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})

	// Run this transction in the VM. The VM updates the state
	ledger.SetState("MyContract", "x", []byte("hello"))
	ledger.SetState("MyOtherContract", "y", []byte("goodbuy"))

	// Commit txbatch that creates the 2nd block on blockchain
	ledger.CommitTxBatch(2, []*protos.Transaction{transaction2a, transaction2b}, []byte("dummy-proof"))
	// -----------------------------</Block #2>-----------------------------------

	// -----------------------------<Block #3>------------------------------------

	ledger.BeginTxBatch(3)
	transaction3a := protos.NewTransaction(protos.ChaincodeID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction3b := protos.NewTransaction(protos.ChaincodeID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})
	transaction3c := protos.NewTransaction(protos.ChaincodeID{Url: "MyImportantContract"}, generateUUID(t), "setZ", []string{"{z: \"super\"}"})

	ledger.SetState("MyContract", "x", []byte("hello"))
	ledger.SetState("MyOtherContract", "y", []byte("goodbuy"))
	ledger.SetState("MyImportantContract", "z", []byte("super"))

	ledger.CommitTxBatch(3, []*protos.Transaction{transaction3a, transaction3b, transaction3c}, []byte("dummy-proof"))

	// -----------------------------</Block #3>-----------------------------------

	// -----------------------------<Block #4>------------------------------------

	ledger.BeginTxBatch(4)
	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction4a := protos.NewTransaction(protos.ChaincodeID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction4b := protos.NewTransaction(protos.ChaincodeID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})
	transaction4c := protos.NewTransaction(protos.ChaincodeID{Url: "MyImportantContract"}, generateUUID(t), "setZ", []string{"{z: \"super\"}"})
	transaction4d := protos.NewTransaction(protos.ChaincodeID{Url: "MyMEGAContract"}, generateUUID(t), "setMEGA", []string{"{mega: \"MEGA\"}"})

	// Run this transction in the VM. The VM updates the state
	ledger.SetState("MyContract", "x", []byte("hello"))
	ledger.SetState("MyOtherContract", "y", []byte("goodbuy"))
	ledger.SetState("MyImportantContract", "z", []byte("super"))
	ledger.SetState("MyMEGAContract", "mega", []byte("MEGA"))

	// Create the 4th block and add it to the chain
	ledger.CommitTxBatch(4, []*protos.Transaction{transaction4a, transaction4b, transaction4c, transaction4d}, []byte("dummy-proof"))
	// -----------------------------</Block #4>-----------------------------------

	return
}

func generateUUID(t *testing.T) string {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Error generating UUID: %s", err)
	}
	return uuid
}
