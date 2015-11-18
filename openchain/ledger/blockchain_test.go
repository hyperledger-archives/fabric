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

	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

func TestChain_Transaction_ContractNew_Golang_FromFile(t *testing.T) {
	chain := initTestBlockChain(t)

	// Create the Chaincode specification
	chaincodeSpec := &protos.ChaincodeSpec{Type: protos.ChaincodeSpec_GOLANG,
		ChaincodeID: &protos.ChaincodeID{Url: "Contracts"},
		CtorMsg:    &protos.ChaincodeInput{Function: "Initialize", Args: []string{"param1"}}}
	chaincodeDeploymentSepc := &protos.ChaincodeDeploymentSpec{ChaincodeSpec: chaincodeSpec}
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		t.Fatalf("Error generating UUID. Error = [%s]", uuidErr)
	}
	newChaincodeTx, err := protos.NewChaincodeDeployTransaction(chaincodeDeploymentSepc, uuid)
	if err != nil {
		t.Fail()
		t.Logf("Failed to create new chaincode Deployment Transaction: %s", err)
		return
	}
	t.Logf("New chaincode tx: %v", newChaincodeTx)

	block1 := protos.NewBlock("sheehan", []*protos.Transaction{newChaincodeTx})

	err = chain.addBlock(context.TODO(), block1)
	if err != nil {
		t.Logf("Error adding block to chain: %s", err)
		t.Fail()
	} else {
		t.Logf("New chain: %v", chain)
	}
	checkChainSize(t, 1)
}

func TestBlockChainSimpleChain(t *testing.T) {
	initTestBlockChain(t)

	allBlocks, allStateHashes := buildSimpleChain(t)
	checkChainSize(t, uint64(len(allBlocks)))
	checkHash(t, getLastBlock(t).GetStateHash(), allStateHashes[len(allStateHashes)-1])

	for i := range allStateHashes {
		t.Logf("Checking state hash for block number = [%d]", i)
		checkHash(t, getBlock(t, i).GetStateHash(), allStateHashes[i])
	}

	for i := range allBlocks {
		t.Logf("Checking block hash for block number = [%d]", i)
		checkHash(t, getBlockHash(t, getBlock(t, i)), getBlockHash(t, allBlocks[i]))
	}

	checkHash(t, allBlocks[0].PreviousBlockHash, []byte{})

	i := 1
	for i < len(allBlocks) {
		t.Logf("Checking previous block hash for block number = [%d]", i)
		checkHash(t, getBlock(t, i).PreviousBlockHash, getBlockHash(t, allBlocks[i-1]))
		i++
	}
}

func TestBlockChainEmptyChain(t *testing.T) {
	initTestBlockChain(t)
	checkChainSize(t, 0)
	block := getLastBlock(t)
	if block != nil {
		t.Fatalf("Get last block on an empty chain should return nil.")
	}
	t.Logf("last block = [%s]", block)
}

func buildSimpleChain(t *testing.T) (blocks []*protos.Block, hashes [][]byte) {
	var allBlocks []*protos.Block
	var allHashes [][]byte

	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	chain, err := getBlockchain()
	if err != nil {
		t.Fatalf("Error while getting handle to block chain. Error = [%s]", err)
	}
	state := getState()
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block>-------------------------------
	// Add the first (genesis block)
	stateHash := getTestStateHash(t)
	block1 := protos.NewBlock("sheehan", nil)

	allBlocks = append(allBlocks, block1)
	allHashes = append(allHashes, stateHash)
	chain.addBlock(context.TODO(), block1)

	// -----------------------------</Genisis block>------------------------------

	// -----------------------------<Block 2>-------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chaincode instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChaincodeID{Url: "Contracts"}, generateUUID(t), "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.set("MyContract1", "code", []byte("code example"))

	// Now we add the transaction to the block 2 and add the block to the chain
	stateHash = getTestStateHash(t)
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a)

	allBlocks = append(allBlocks, block2)
	allHashes = append(allHashes, stateHash)
	chain.addBlock(context.TODO(), block2)

	// -----------------------------</Block 2>------------------------------------

	// -----------------------------<Block 3>-------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChaincodeID{Url: "MyContract"}, generateUUID(t), "setX", []string{"{x: \"hello\"}"})

	// Run this transction in the VM. The VM updates the state
	state.set("MyContract", "x", []byte("hello"))

	// Create the thrid block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a}
	stateHash = getTestStateHash(t)
	block3 := protos.NewBlock("sheehan", transactions3a)

	allBlocks = append(allBlocks, block3)
	allHashes = append(allHashes, stateHash)
	chain.addBlock(context.TODO(), block3)

	// -----------------------------</Block 3>------------------------------------

	return allBlocks, allHashes
}

func buildTestTx() (*protos.Transaction, string) {
	uuid, _ := util.GenerateUUID()
	return protos.NewTransaction(protos.ChaincodeID{Url: "testUrl", Version: "1.1"}, uuid, "anyfunction", []string{"param1, param2"}), uuid
}

func checkHash(t *testing.T, hash []byte, expectedHash []byte) {
	if !bytes.Equal(hash, expectedHash) {
		t.Fatalf("hash is not same as exepected. Expected=[%x], found=[%x]", expectedHash, hash)
	}
	t.Logf("Hash value = [%x]", hash)
}

func getTestStateHash(t *testing.T) []byte {
	state := getState()
	stateHash, err := state.getHash()
	if err != nil {
		t.Fatalf("Error while getting state hash. Error = [%s]", err)
	}
	return stateHash
}

func getBlockHash(t *testing.T, block *protos.Block) []byte {
	hash, err := block.GetHash()
	if err != nil {
		t.Fatalf("Error while getting blockhash from in-memory block. Error = [%s]", err)
	}
	return hash
}

func getLastBlock(t *testing.T) *protos.Block {
	chain := getTestBlockchain(t)
	lastBlock, err := chain.getLastBlock()
	if err != nil {
		t.Fatalf("Error while getting last block from chain. [%s]", err)
	}
	return lastBlock
}

func getBlock(t *testing.T, blockNumber int) *protos.Block {
	chain := getTestBlockchain(t)
	block, err := chain.getBlock(uint64(blockNumber))
	if err != nil {
		t.Fatalf("Error while getting block from chain. [%s]", err)
	}
	return block
}

func buildTestBlock() *protos.Block {
	transactions := []*protos.Transaction{}
	tx, _ := buildTestTx()
	transactions = append(transactions, tx)
	block := protos.NewBlock("ErrorCreator", transactions)
	return block
}

func checkChainSize(t *testing.T, expectedSize uint64) {
	chain, _ := getBlockchain()
	chainSize := chain.getSize()
	chainSizeInDb, err := fetchBlockchainSizeFromDB()
	t.Logf("Chain size in-memory=[%d] and in db=[%d]", chainSize, chainSizeInDb)
	if err != nil {
		t.Fatalf("Error in getting chain size from DB. Error = [%s]", err)
	}
	if chainSize != expectedSize {
		t.Fatalf("wrong chain size. Expected =[%d], found=[%d]", expectedSize, chainSize)
	}
	if chainSize != chainSizeInDb {
		t.Fatalf("chain size value different in DB from in-memory. in-memory=[%d], in db=[%d]", chainSize, chainSizeInDb)
	}
}
