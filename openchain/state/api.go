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

package state

import (
	"fmt"

	"golang.org/x/net/context"

	google_protobuf1 "google/protobuf"

	pb "github.com/openblockchain/obc-peer/protos"
)

// ServerOpenchain defines the Openchain server object, which holds the
// blockchain data structure.
type ServerOpenchain struct {
	blockchain *Blockchain
}

// NewOpenchainServer creates a new instance of the ServerOpenchain.
func NewOpenchainServer() *ServerOpenchain {
	s := new(ServerOpenchain)
	return s
}

// GetBlockchainInfo returns information about the blockchain ledger such as
// height, current block hash, and previous block hash.
func (s *ServerOpenchain) GetBlockchainInfo(ctx context.Context, e *google_protobuf1.Empty) (*pb.BlockchainInfo, error) {
	// Total number of blocks in the blockchain.
	size := s.blockchain.GetSize()

	// Check the number of blocks in the blockchain. If the blockchain is empty,
	// return error. There will always be at least one block in the blockchain,
	// the genesis block.
	if size > 0 {
		currentBlock, currentBlockErr := s.blockchain.GetLastBlock()
		if currentBlockErr != nil {
			return nil, currentBlockErr
		}
		currentHash, currentHashErr := currentBlock.GetHash()
		if currentHashErr != nil {
			return nil, fmt.Errorf("Could not get hash of last block in blockchain: %s", currentHashErr)
		}

		info := &pb.BlockchainInfo{Height: size, CurrentBlockHash: currentHash, PreviousBlockHash: currentBlock.PreviousBlockHash}
		return info, nil
	}

	return nil, fmt.Errorf("Error: No blocks in blockchain.")
}

// GetBlockByNumber returns the data contained within a specific block in the
// blockchain. The genesis block is block zero.
func (s *ServerOpenchain) GetBlockByNumber(ctx context.Context, num *pb.BlockNumber) (*pb.Block, error) {
	// Total number of blocks in the blockchain.
	size := s.blockchain.GetSize()

	// Check the number of blocks in the blockchain. If the blockchain is empty,
	// return error. There will always be at least one block in the blockchain,
	// the genesis block.
	if size > 0 {
		// If the block number requested is not in the blockchain, return error.
		if num.Number > (size - 1) {
			return nil, fmt.Errorf("Error: Requested block not in blockchain.")
		}

		block, blockErr := s.blockchain.GetBlock(num.Number)
		if blockErr != nil {
			return nil, fmt.Errorf("Error retrieving block from blockchain: %s", blockErr)
		}

		return block, nil
	}

	return nil, fmt.Errorf("Error: No blocks in blockchain.")
}

// GetBlockCount returns the current number of blocks in the blockchain data
// structure.
func (s *ServerOpenchain) GetBlockCount(ctx context.Context, e *google_protobuf1.Empty) (*pb.BlockCount, error) {
	// Total number of blocks in the blockchain.
	size := s.blockchain.GetSize()

	// Check the number of blocks in the blockchain. If the blockchain is empty,
	// return error. There will always be at least one block in the blockchain,
	// the genesis block.
	if size > 0 {
		count := &pb.BlockCount{Count: size}
		return count, nil
	}

	return nil, fmt.Errorf("Error: No blocks in blockchain.")
}
