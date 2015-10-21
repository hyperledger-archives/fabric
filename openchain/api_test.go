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
	"google/protobuf"
	"testing"

	"github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

func TestServerOpenchain_API_GetBlockchainInfo(t *testing.T) {
	// Must initialize the Blockchain singleton before initializing the
	// OpenchainServer, as it needs that pointer.

	// Construct a blockchain with 0 blocks.

	chain0 := initTestBlockChain(t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	server.blockchain = chain0
	t.Logf("Chain 0 => %s", server.blockchain)

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

	// Construct a blockchain with 3 blocks.

	chain1 := initTestBlockChain(t)
	chainErr1 := buildTestChain1(chain1, t)
	if chainErr1 != nil {
		t.Fail()
		t.Logf("Error creating chain1: %s", chainErr1)
	}
	server.blockchain = chain1
	t.Logf("Chain 1 => %s", server.blockchain)

	// Attempt to retrieve the blockchain info.

	info, err = server.GetBlockchainInfo(context.Background(), &google_protobuf.Empty{})
	if err != nil {
		t.Logf("Error retrieving blockchain info: %s", err)
		t.Fail()
	} else {
		t.Logf("Blockchain 1 info: %v", info)
	}

	// Construct a blockchain with 5 blocks.

	chain2 := initTestBlockChain(t)
	chainErr2 := buildTestChain2(chain2, t)
	if chainErr2 != nil {
		t.Fail()
		t.Logf("Error creating chain2: %s", chainErr2)
	}

	server.blockchain = chain2
	t.Logf("Chain 2 => %s", server.blockchain)

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
	// Must initialize the Blockchain singleton before initializing the
	// OpenchainServer, as it needs that pointer.

	// Construct a blockchain with 0 blocks.

	chain0 := initTestBlockChain(t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	server.blockchain = chain0
	t.Logf("Chain 0 => %s", server.blockchain)

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

	// Construct a blockchain with 3 blocks.

	chain1 := initTestBlockChain(t)
	chainErr1 := buildTestChain1(chain1, t)
	if chainErr1 != nil {
		t.Fail()
		t.Logf("Error creating chain1: %s", chainErr1)
	}
	server.blockchain = chain1
	t.Logf("Chain 1 => %s", server.blockchain)

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
	// Must initialize the Blockchain singleton before initializing the
	// OpenchainServer, as it needs that pointer.

	// Construct a blockchain with 0 blocks.

	chain0 := initTestBlockChain(t)

	// Initialize the OpenchainServer object.
	server, err := NewOpenchainServer()
	if err != nil {
		t.Logf("Error creating OpenchainServer: %s", err)
		t.Fail()
	}

	server.blockchain = chain0
	t.Logf("Chain 0 => %s", server.blockchain)

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

	// Construct a blockchain with 3 blocks.

	chain1 := initTestBlockChain(t)
	chainErr1 := buildTestChain1(chain1, t)
	if chainErr1 != nil {
		t.Fail()
		t.Logf("Error creating chain1: %s", chainErr1)
	}

	server.blockchain = chain1
	t.Logf("Chain 1 => %s", server.blockchain)

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

	// Construct a blockchain with 5 blocks.

	chain2 := initTestBlockChain(t)
	chainErr2 := buildTestChain2(chain2, t)
	if chainErr2 != nil {
		t.Fail()
		t.Logf("Error creating chain2: %s", chainErr2)
	}

	server.blockchain = chain2
	t.Logf("Chain 2 => %s", server.blockchain)

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

// buildTestChain1 builds a simple blockchain data structure that contains 3
// blocks.
func buildTestChain1(chain *Blockchain, t *testing.T) error {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	state := GetState()
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block, block #0>---------------------
	// Add the 0th (genesis block)
	block1 := protos.NewBlock("sheehan", nil)
	chain.AddBlock(context.TODO(), block1)
	// -----------------------------</Genisis block, block #0>--------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chainlet instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChainletID{Url: "Contracts"}, generateUUID(t), "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.Set("MyContract1", "code", []byte("code example"))

	// Now we add the transaction to block 1 and add the block to the chain
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a)
	chain.AddBlock(context.TODO(), block2)

	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Set("MyContract", "x", []byte("hello"))

	// Create the 2nd block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a}
	block3 := protos.NewBlock("sheehan", transactions3a)
	chain.AddBlock(context.TODO(), block3)

	// -----------------------------</Block #2>-----------------------------------
	return nil
}

// buildTestChain2 builds a simple blockchain data structure that contains 5
// blocks, with each block containing the same number of transactions as its
// index within the blockchain. Block 0, 0 transactions. Block 1, 1 transaction,
// and so on.
func buildTestChain2(chain *Blockchain, t *testing.T) error {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	state := GetState()
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block, block #0>---------------------
	// Add the 0th (genesis block)
	block1 := protos.NewBlock("sheehan", nil)
	chain.AddBlock(context.TODO(), block1)
	// -----------------------------</Genisis block, block #0>--------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chainlet instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChainletID{Url: "Contracts"}, generateUUID(t), "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.Set("MyContract1", "code", []byte("code example"))

	// Now we add the transaction to block 1 and add the block to the chain
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a)
	chain.AddBlock(context.TODO(), block2)

	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction3b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Set("MyContract", "x", []byte("hello"))
	state.Set("MyOtherContract", "y", []byte("goodbuy"))

	// Create the 2nd block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a, transaction3b}
	block3 := protos.NewBlock("sheehan", transactions3a)
	chain.AddBlock(context.TODO(), block3)

	// -----------------------------</Block #2>-----------------------------------

	// -----------------------------<Block #3>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction4a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction4b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})
	transaction4c := protos.NewTransaction(protos.ChainletID{Url: "MyImportantContract"}, generateUUID(t), "setZ", []string{"{z: \"super\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Set("MyContract", "x", []byte("hello"))
	state.Set("MyOtherContract", "y", []byte("goodbuy"))
	state.Set("MyImportantContract", "z", []byte("super"))

	// Create the 3rd block and add it to the chain
	transactions4a := []*protos.Transaction{transaction4a, transaction4b, transaction4c}
	block4 := protos.NewBlock("sheehan", transactions4a)
	chain.AddBlock(context.TODO(), block4)

	// -----------------------------</Block #3>-----------------------------------

	// -----------------------------<Block #4>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction5a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})
	transaction5b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, generateUUID(t), "setY", []string{"{y: \"goodbuy\"}"})
	transaction5c := protos.NewTransaction(protos.ChainletID{Url: "MyImportantContract"}, generateUUID(t), "setZ", []string{"{z: \"super\"}"})
	transaction5d := protos.NewTransaction(protos.ChainletID{Url: "MyMEGAContract"}, generateUUID(t), "setMEGA", []string{"{mega: \"MEGA\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Set("MyContract", "x", []byte("hello"))
	state.Set("MyOtherContract", "y", []byte("goodbuy"))
	state.Set("MyImportantContract", "z", []byte("super"))
	state.Set("MyMEGAContract", "mega", []byte("MEGA"))

	// Create the 4th block and add it to the chain
	transactions5a := []*protos.Transaction{transaction5a, transaction5b, transaction5c, transaction5d}
	block5 := protos.NewBlock("sheehan", transactions5a)
	chain.AddBlock(context.TODO(), block5)

	// -----------------------------</Block #4>-----------------------------------

	return nil
}
