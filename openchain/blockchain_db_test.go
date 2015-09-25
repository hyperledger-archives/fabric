package openchain

import (
	"os"
	"testing"

	"github.com/tecbot/gorocksdb"

	"github.com/openblockchain/obc-peer/protos"
)

func TestSize(t *testing.T) {

	// Open the DB
	dbPath := os.TempDir() + "/OpenchainDBTestSize"

	opts := gorocksdb.NewDefaultOptions()
	destoryErr := gorocksdb.DestroyDb(dbPath, opts)
	if destoryErr != nil {
		t.Error("Error destroying DB", destoryErr)
	}

	blockchainDB, err := OpenBlockchainDB(dbPath, true)
	if err != nil {
		t.Error("Error opening DB", err)
	}

	// Ensure size is 0
	size, err := blockchainDB.GetSize()
	if err != nil {
		t.Error("Error getting DB size", err)
	}
	if size != 0 {
		t.Error("Expected size to be 0, but got", size)
	}

	// Add a block
	state := NewState()
	block := protos.NewBlock("sheehan", nil, state.GetHash())
	err = blockchainDB.AddBlock(*block)
	if err != nil {
		t.Error("Error adding block to DB", err)
	}

	// Ensuze size is 1
	size, err = blockchainDB.GetSize()
	if err != nil {
		t.Error("Error getting DB size", err)
	}
	if size != 1 {
		t.Error("Expected size to be 1, but got", size)
	}

	block, blockErr := blockchainDB.GetBlock(0)
	if blockErr != nil {
		t.Error("Error reading block from DB", blockErr)
	}
	if block.ProposerID != "sheehan" {
		t.Error("Expected ProposerID to be sheehan, got", block.ProposerID)
	}

	lastBlock, lastBlockErr := blockchainDB.GetLastBlock()
	if lastBlockErr != nil {
		t.Error("Error reading last block from DB", lastBlockErr)
	}
	if lastBlock.ProposerID != "sheehan" {
		t.Error("Expected ProposerID to be sheehan, got", lastBlock.ProposerID)
	}
}
