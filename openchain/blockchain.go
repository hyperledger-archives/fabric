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

	"golang.org/x/net/context"

	"hub.jazz.net/openchain-peer/protos"
)

// Blockchain defines list of blocks that make up a blockchain.
type Blockchain struct {
	blocks []protos.Block
}

// NewBlockchain creates a new empty blockchain.
func NewBlockchain() *Blockchain {
	blockchain := new(Blockchain)
	return blockchain
}

// AddBlock adds a block to the blockchain.
func (blockchain *Blockchain) AddBlock(ctx context.Context, block protos.Block) error {
	size := len(blockchain.blocks)
	if size > 0 {
		previousBlock := blockchain.blocks[size-1]
		hash, err := previousBlock.GetHash()
		if err != nil {
			return errors.New(fmt.Sprintf("Error adding block: %s", err))
		}
		block.SetPreviousBlockHash(hash)
	}
	blockchain.blocks = append(blockchain.blocks, block)
	return nil
}

func (blockchain *Blockchain) String() string {
	var buffer bytes.Buffer
	for i := 0; i < len(blockchain.blocks); i++ {
		buffer.WriteString("\n----------<block>----------\n")
		buffer.WriteString(blockchain.blocks[i].String())
		buffer.WriteString("\n----------<\\block>----------\n")
	}
	return buffer.String()
}
