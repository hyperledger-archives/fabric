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
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/protos"
)

// Blockchain defines list of blocks that make up a blockchain.
type Blockchain struct {
	db BlockchainDB
}

// NewBlockchain creates a new empty blockchain.
func NewBlockchain(blockchainPath string, createIfMissing bool) (*Blockchain, error) {
	blockchain := new(Blockchain)
	db, err := OpenBlockchainDB(blockchainPath, createIfMissing)
	if err != nil {
		return nil, err
	}
	blockchain.db = *db
	return blockchain, nil
}

// AddBlock adds a block to the blockchain.
func (blockchain *Blockchain) AddBlock(ctx context.Context, block protos.Block) error {

	size, sizeErr := blockchain.db.GetSize()
	if sizeErr != nil {
		return sizeErr
	}

	if size > 0 {
		previousBlock, previousBlockErr := blockchain.db.GetLastBlock()
		if previousBlockErr != nil {
			return previousBlockErr
		}
		prviousBlockHash, prviousBlockHashErr := previousBlock.GetHash()
		if prviousBlockHashErr != nil {
			return errors.New(fmt.Sprintf("Error adding block: %s", prviousBlockHashErr))
		}
		block.SetPreviousBlockHash(prviousBlockHash)
	}

	addBlockErr := blockchain.db.AddBlock(block)
	if addBlockErr != nil {
		return addBlockErr
	}
	return nil
}

// GetLastBlock returns the last block added to the blockchain
func (blockchain *Blockchain) GetLastBlock() (*protos.Block, error) {
	return blockchain.db.GetLastBlock()
}

func (blockchain *Blockchain) String() string {
	var buffer bytes.Buffer
	size, sizeErr := blockchain.db.GetSize()
	if sizeErr != nil {
		return ""
	}

	for i := uint64(0); i < size; i++ {
		block, blockErr := blockchain.db.GetBlock(i)
		if blockErr != nil {
			return ""
		}
		buffer.WriteString("\n----------<block #")
		buffer.WriteString(strconv.FormatUint(i, 10))
		buffer.WriteString(">----------\n")
		buffer.WriteString(block.String())
		buffer.WriteString("\n----------<\\block #")
		buffer.WriteString(strconv.FormatUint(i, 10))
		buffer.WriteString(">----------\n")
	}
	return buffer.String()
}
