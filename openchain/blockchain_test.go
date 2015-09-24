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

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/protos"
)

func TestChain_Transaction_ContractNew_Golang_FromFile(t *testing.T) {
	chain := NewBlockchain()
	state := NewState()

	// Create the Chainlet specification
	chainletSpec := &protos.ChainletSpec{Type: protos.ChainletSpec_GOLANG,
		ChainletID: &protos.ChainletID{Url: "Contracts"},
		CtorMsg:    &protos.ChainletMessage{Function: "Initialize", Args: []string{"param1"}}}

	newChainletTx := protos.NewChainletDeployTransaction(*chainletSpec)
	t.Logf("New chaincode tx: %v", newChainletTx)
	block1 := protos.NewBlock("sheehan", []*protos.Transaction{newChainletTx}, state.GetHash())

	err := chain.AddBlock(context.TODO(), *block1)
	if err != nil {
		t.Logf("Error adding block to chain: %s", err)
		t.Fail()
	} else {
		t.Logf("New chain: %v", chain)
	}
}

func TestChain1(t *testing.T) {

	chain1, state1 := buildSimpleChain()
	t.Logf("Chain1 => %s", chain1)
	t.Logf("State1 => %s", state1)

	chain2, state2 := buildSimpleChain()
	t.Logf("Chain2 => %s", chain2)
	t.Logf("State2 => %s", state2)

	chain1LastBlock := chain1.blocks[len(chain1.blocks)-1]
	hash1, err1 := chain1LastBlock.GetHash()
	if err1 != nil {
		t.Fail()
		t.Logf("Error getting chain1 block hash: %s", err1)
	}

	chain2LastBlock := chain2.blocks[len(chain2.blocks)-1]
	hash2, err2 := chain2LastBlock.GetHash()
	if err2 != nil {
		t.Fail()
		t.Logf("Error getting chain2 block hash: %s", err2)
	}

	if bytes.Compare(hash1, hash2) != 0 {
		t.Error("Expected block hashes to match.")
	}
}

func buildSimpleChain() (*Blockchain, *State) {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	chain := NewBlockchain()
	state := NewState()
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block>-------------------------------
	// Add the first (genesis block)
	block1 := protos.NewBlock("sheehan", nil, state.GetHash())
	chain.AddBlock(context.TODO(), *block1)
	// -----------------------------</Genisis block>------------------------------

	// -----------------------------<Block 2>-------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chaincode instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChainletID{Url: "Contracts"}, "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.Update("Contracts", "{MyContract1: {code: blahblah}}")

	// Now we add the transaction to the block 2 and add the block to the chain
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a, state.GetHash())
	chain.AddBlock(context.TODO(), *block2)

	// -----------------------------</Block 2>------------------------------------

	// -----------------------------<Block 3>-------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, "setX", []string{"{x: \"hello\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Update("MyContract", "{x: \"hello\"}")

	// Create the thrid block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a}
	block3 := protos.NewBlock("sheehan", transactions3a, state.GetHash())
	chain.AddBlock(context.TODO(), *block3)

	// -----------------------------</Block 3>------------------------------------

	return chain, state
}
