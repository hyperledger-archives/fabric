package openchain

import (
	"google/protobuf"
	"os"
	"testing"

	"github.com/tecbot/gorocksdb"

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/protos"
)

func TestServerOpenchain_API_GetBlockchainInfo(t *testing.T) {
	server := NewOpenchainServer()

	opts := gorocksdb.NewDefaultOptions()

	// Construct a blockchain with 0 blocks.

	chain0Path := os.TempDir() + "/OpenchainAPITestChain0"
	destroy0Err := gorocksdb.DestroyDb(chain0Path, opts)
	if destroy0Err != nil {
		t.Error("Error destroying chain0 DB", destroy0Err)
	}

	state0Path := os.TempDir() + "/OpenchainAPITestState0"
	destroyState0Err := gorocksdb.DestroyDb(state0Path, opts)
	if destroyState0Err != nil {
		t.Error("Error destroying state0 DB", destroyState0Err)
	}

	chain0, _, chainErr0 := buildTestChain0(chain0Path, state0Path)
	if chainErr0 != nil {
		t.Fail()
		t.Logf("Error creating chain0: %s", chainErr0)
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

	chain1Path := os.TempDir() + "/OpenchainAPITestChain1"
	destroy1Err := gorocksdb.DestroyDb(chain1Path, opts)
	if destroy1Err != nil {
		t.Error("Error destroying chain1 DB", destroy1Err)
	}

	state1Path := os.TempDir() + "/OpenchainAPITestState1"
	destroyState1Err := gorocksdb.DestroyDb(state1Path, opts)
	if destroyState1Err != nil {
		t.Error("Error destroying state1 DB", destroyState1Err)
	}

	chain1, _, chainErr1 := buildTestChain1(chain1Path, state1Path)
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

	chain2Path := os.TempDir() + "/OpenchainAPITestChain2"
	destroy2Err := gorocksdb.DestroyDb(chain2Path, opts)
	if destroy2Err != nil {
		t.Error("Error destroying chain2 DB", destroy2Err)
	}

	state2Path := os.TempDir() + "/OpenchainAPITestState2"
	destroyState2Err := gorocksdb.DestroyDb(state2Path, opts)
	if destroyState2Err != nil {
		t.Error("Error destroying state2 DB", destroyState2Err)
	}

	chain2, _, chainErr2 := buildTestChain2(chain2Path, state2Path)
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
	server := NewOpenchainServer()

	opts := gorocksdb.NewDefaultOptions()

	// Construct a blockchain with 0 blocks.

	chain0Path := os.TempDir() + "/OpenchainAPITestChain0"
	destroy0Err := gorocksdb.DestroyDb(chain0Path, opts)
	if destroy0Err != nil {
		t.Error("Error destroying chain0 DB", destroy0Err)
	}

	state0Path := os.TempDir() + "/OpenchainAPITestState0"
	destroyState0Err := gorocksdb.DestroyDb(state0Path, opts)
	if destroyState0Err != nil {
		t.Error("Error destroying state0 DB", destroyState0Err)
	}

	chain0, _, chainErr0 := buildTestChain0(chain0Path, state0Path)
	if chainErr0 != nil {
		t.Fail()
		t.Logf("Error creating chain0: %s", chainErr0)
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

	chain1Path := os.TempDir() + "/OpenchainAPITestChain1"
	destroy1Err := gorocksdb.DestroyDb(chain1Path, opts)
	if destroy1Err != nil {
		t.Error("Error destroying chain1 DB", destroy1Err)
	}

	state1Path := os.TempDir() + "/OpenchainAPITestState1"
	destroyState1Err := gorocksdb.DestroyDb(state1Path, opts)
	if destroyState1Err != nil {
		t.Error("Error destroying state1 DB", destroyState1Err)
	}

	chain1, _, chainErr1 := buildTestChain1(chain1Path, state1Path)
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
	server := NewOpenchainServer()

	opts := gorocksdb.NewDefaultOptions()

	// Construct a blockchain with 0 blocks.

	chain0Path := os.TempDir() + "/OpenchainAPITestChain0"
	destroy0Err := gorocksdb.DestroyDb(chain0Path, opts)
	if destroy0Err != nil {
		t.Error("Error destroying chain0 DB", destroy0Err)
	}

	state0Path := os.TempDir() + "/OpenchainAPITestState0"
	destroyState0Err := gorocksdb.DestroyDb(state0Path, opts)
	if destroyState0Err != nil {
		t.Error("Error destroying state0 DB", destroyState0Err)
	}

	chain0, _, chainErr0 := buildTestChain0(chain0Path, state0Path)
	if chainErr0 != nil {
		t.Fail()
		t.Logf("Error creating chain0: %s", chainErr0)
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

	chain1Path := os.TempDir() + "/OpenchainAPITestChain1"
	destroy1Err := gorocksdb.DestroyDb(chain1Path, opts)
	if destroy1Err != nil {
		t.Error("Error destroying chain1 DB", destroy1Err)
	}

	state1Path := os.TempDir() + "/OpenchainAPITestState1"
	destroyState1Err := gorocksdb.DestroyDb(state1Path, opts)
	if destroyState1Err != nil {
		t.Error("Error destroying state1 DB", destroyState1Err)
	}

	chain1, _, chainErr1 := buildTestChain1(chain1Path, state1Path)
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

	chain2Path := os.TempDir() + "/OpenchainAPITestChain2"
	destroy2Err := gorocksdb.DestroyDb(chain2Path, opts)
	if destroy2Err != nil {
		t.Error("Error destroying chain2 DB", destroy2Err)
	}

	state2Path := os.TempDir() + "/OpenchainAPITestState2"
	destroyState2Err := gorocksdb.DestroyDb(state2Path, opts)
	if destroyState2Err != nil {
		t.Error("Error destroying state2 DB", destroyState2Err)
	}

	chain2, _, chainErr2 := buildTestChain2(chain2Path, state2Path)
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

// buildTestChain0 builds a simple blockchain data structure that contains 0
// blocks. This chain is created only to verify error conditions, as a
// blockchain will always contain at least one block, the genesis block.
func buildTestChain0(blockchainPath, statePath string) (*Blockchain, *State, error) {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	chain, err := NewBlockchain(blockchainPath, true)
	if err != nil {
		return nil, nil, err
	}
	state, stateErr := NewState(statePath, true)
	if stateErr != nil {
		return nil, nil, err
	}
	// -----------------------------</Initial creation of blockchain and state>---

	return chain, state, nil
}

// buildTestChain1 builds a simple blockchain data structure that contains 3
// blocks.
func buildTestChain1(blockchainPath, statePath string) (*Blockchain, *State, error) {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	chain, err := NewBlockchain(blockchainPath, true)
	if err != nil {
		return nil, nil, err
	}
	state, stateErr := NewState(statePath, true)
	if stateErr != nil {
		return nil, nil, err
	}
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block, block #0>---------------------
	// Add the 0th (genesis block)
	block1 := protos.NewBlock("sheehan", nil, state.GetHash())
	chain.AddBlock(context.TODO(), *block1)
	// -----------------------------</Genisis block, block #0>--------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chainlet instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChainletID{Url: "Contracts"}, "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.Put("MyContract1", []byte("code"), []byte("code example"))

	// Now we add the transaction to block 1 and add the block to the chain
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a, state.GetHash())
	chain.AddBlock(context.TODO(), *block2)

	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, "setX", []string{"{x: \"hello\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Put("MyContract", []byte("x"), []byte("hello"))

	// Create the 2nd block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a}
	block3 := protos.NewBlock("sheehan", transactions3a, state.GetHash())
	chain.AddBlock(context.TODO(), *block3)

	// -----------------------------</Block #2>-----------------------------------

	return chain, state, nil
}

// buildTestChain2 builds a simple blockchain data structure that contains 5
// blocks, with each block containing the same number of transactions as its
// index within the blockchain. Block 0, 0 transactions. Block 1, 1 transaction,
// and so on.
func buildTestChain2(blockchainPath, statePath string) (*Blockchain, *State, error) {
	// -----------------------------<Initial creation of blockchain and state>----
	// Define an initial blockchain and state
	chain, err := NewBlockchain(blockchainPath, true)
	if err != nil {
		return nil, nil, err
	}
	state, stateErr := NewState(statePath, true)
	if stateErr != nil {
		return nil, nil, err
	}
	// -----------------------------</Initial creation of blockchain and state>---

	// -----------------------------<Genisis block, block #0>---------------------
	// Add the 0th (genesis block)
	block1 := protos.NewBlock("sheehan", nil, state.GetHash())
	chain.AddBlock(context.TODO(), *block1)
	// -----------------------------</Genisis block, block #0>--------------------

	// -----------------------------<Block #1>------------------------------------

	// Deploy a contract
	// To deploy a contract, we call the 'NewContract' function in the 'Contracts' contract
	// TODO Use chainlet instead of contract?
	// TODO Two types of transactions. Execute transaction, deploy/delete/update contract
	transaction2a := protos.NewTransaction(protos.ChainletID{Url: "Contracts"}, "NewContract", []string{"name: MyContract1, code: var x; function setX(json) {x = json.x}}"})

	// VM runs transaction2a and updates the global state with the result
	// In this case, the 'Contracts' contract stores 'MyContract1' in its state
	state.Put("MyContract1", []byte("code"), []byte("code example"))

	// Now we add the transaction to block 1 and add the block to the chain
	transactions2a := []*protos.Transaction{transaction2a}
	block2 := protos.NewBlock("sheehan", transactions2a, state.GetHash())
	chain.AddBlock(context.TODO(), *block2)

	// -----------------------------</Block #1>-----------------------------------

	// -----------------------------<Block #2>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction3a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, "setX", []string{"{x: \"hello\"}"})
	transaction3b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, "setY", []string{"{y: \"goodbuy\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Put("MyContract", []byte("x"), []byte("hello"))
	state.Put("MyOtherContract", []byte("y"), []byte("goodbuy"))

	// Create the 2nd block and add it to the chain
	transactions3a := []*protos.Transaction{transaction3a, transaction3b}
	block3 := protos.NewBlock("sheehan", transactions3a, state.GetHash())
	chain.AddBlock(context.TODO(), *block3)

	// -----------------------------</Block #2>-----------------------------------

	// -----------------------------<Block #3>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction4a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, "setX", []string{"{x: \"hello\"}"})
	transaction4b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, "setY", []string{"{y: \"goodbuy\"}"})
	transaction4c := protos.NewTransaction(protos.ChainletID{Url: "MyImportantContract"}, "setZ", []string{"{z: \"super\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Put("MyContract", []byte("x"), []byte("hello"))
	state.Put("MyOtherContract", []byte("y"), []byte("goodbuy"))
	state.Put("MyImportantContract", []byte("z"), []byte("super"))

	// Create the 3rd block and add it to the chain
	transactions4a := []*protos.Transaction{transaction4a, transaction4b, transaction4c}
	block4 := protos.NewBlock("sheehan", transactions4a, state.GetHash())
	chain.AddBlock(context.TODO(), *block4)

	// -----------------------------</Block #3>-----------------------------------

	// -----------------------------<Block #4>------------------------------------

	// Now we want to run the function 'setX' in 'MyContract

	// Create a transaction'
	transaction5a := protos.NewTransaction(protos.ChainletID{Url: "MyContract"}, "setX", []string{"{x: \"hello\"}"})
	transaction5b := protos.NewTransaction(protos.ChainletID{Url: "MyOtherContract"}, "setY", []string{"{y: \"goodbuy\"}"})
	transaction5c := protos.NewTransaction(protos.ChainletID{Url: "MyImportantContract"}, "setZ", []string{"{z: \"super\"}"})
	transaction5d := protos.NewTransaction(protos.ChainletID{Url: "MyMEGAContract"}, "setMEGA", []string{"{mega: \"MEGA\"}"})

	// Run this transction in the VM. The VM updates the state
	state.Put("MyContract", []byte("x"), []byte("hello"))
	state.Put("MyOtherContract", []byte("y"), []byte("goodbuy"))
	state.Put("MyImportantContract", []byte("z"), []byte("super"))
	state.Put("MyMEGAContract", []byte("mega"), []byte("MEGA"))

	// Create the 4th block and add it to the chain
	transactions5a := []*protos.Transaction{transaction5a, transaction5b, transaction5c, transaction5d}
	block5 := protos.NewBlock("sheehan", transactions5a, state.GetHash())
	chain.AddBlock(context.TODO(), *block5)

	// -----------------------------</Block #4>-----------------------------------

	return chain, state, nil
}
