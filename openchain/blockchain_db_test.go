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
