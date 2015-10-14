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
	"encoding/binary"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/tecbot/gorocksdb"
	"golang.org/x/net/context"
)

// Blockchain holds basic information in memory. Operations on Blockchain are not thread-safe
type Blockchain struct {
	size              uint64
	previousBlockHash []byte
}

var blockchainInstance *Blockchain

// GetBlockchain get handle to block chain singleton
func GetBlockchain() (*Blockchain, error) {
	if blockchainInstance == nil {
		blockchainInstance = new(Blockchain)
		size, err := fetchBlockchainSizeFromDB()
		if err != nil {
			return nil, err
		}
		blockchainInstance.size = size
		if size > 0 {
			previousBlock, err := fetchBlockFromDB(size - 1)
			if err != nil {
				return nil, err
			}
			previousBlockHash, err := previousBlock.GetHash()
			if err != nil {
				return nil, err
			}
			blockchainInstance.previousBlockHash = previousBlockHash
		}
		err = startIndexer()
		if err != nil {
			return nil, err
		}
	}
	return blockchainInstance, nil
}

// GetLastBlock get last block in blockchain
func (blockchain *Blockchain) GetLastBlock() (*protos.Block, error) {
	if blockchain.size == 0 {
		return nil, nil
	}
	return blockchain.GetBlock(blockchain.size - 1)
}

// GetSize number of blocks in blockchain
func (blockchain *Blockchain) GetSize() uint64 {
	return blockchain.size
}

// GetBlock get block at arbitrary height in block chain
func (blockchain *Blockchain) GetBlock(blockNumber uint64) (*protos.Block, error) {
	return fetchBlockFromDB(blockNumber)
}

// GetBlockByHash get block by block hash
func (blockchain *Blockchain) GetBlockByHash(blockHash []byte) (*protos.Block, error) {
	return fetchBlockByHash(blockHash)
}

// GetTransactions get all transactions in a block identified by block number
func (blockchain *Blockchain) GetTransactions(blockNumber uint64) ([]*protos.Transaction, error) {
	block, err := blockchain.GetBlock(blockNumber)
	if err != nil {
		return nil, err
	}
	return block.GetTransactions(), nil
}

// GetTransactionsByBlockHash get all transactions in a block identified by block hash
func (blockchain *Blockchain) GetTransactionsByBlockHash(blockHash []byte) ([]*protos.Transaction, error) {
	block, err := blockchain.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	return block.GetTransactions(), nil
}

// GetTransaction get a transaction identified by blocknumber and index within the block
func (blockchain *Blockchain) GetTransaction(blockNumber uint64, txIndex uint64) (*protos.Transaction, error) {
	block, err := blockchain.GetBlock(blockNumber)
	if err != nil {
		return nil, err
	}
	return block.GetTransactions()[txIndex], nil
}

// GetTransactionByBlockHash get a transaction identified by blockhash and index within the block
func (blockchain *Blockchain) GetTransactionByBlockHash(blockHash []byte, txIndex uint64) (*protos.Transaction, error) {
	block, err := blockchain.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	return block.GetTransactions()[txIndex], nil
}

// AddBlock add a new block to blockchain
func (blockchain *Blockchain) AddBlock(ctx context.Context, block *protos.Block) error {
	block.SetPreviousBlockHash(blockchain.previousBlockHash)
	state := GetState()
	stateHash, err := state.GetHash()
	if err != nil {
		return err
	}
	block.StateHash = stateHash
	currentBlockNumber := blockchain.size
	err = blockchain.persistBlock(block, currentBlockNumber)
	if err != nil {
		return err
	}
	blockchain.size++
	currentBlockHash, err := block.GetHash()
	if err != nil {
		return err
	}
	blockchain.previousBlockHash = currentBlockHash
	state.ClearInMemoryChanges()

	createIndexesAsync(block, currentBlockNumber, currentBlockHash)
	return nil
}

func fetchBlockFromDB(blockNumber uint64) (*protos.Block, error) {
	blockBytes, err := db.GetDBHandle().GetFromBlockchainCF(encodeBlockNumberDBKey(blockNumber))
	if err != nil {
		return nil, err
	}
	return protos.UnmarshallBlock(blockBytes)
}

func fetchTransactionFromDB(blockNum uint64, txIndex uint64) (*protos.Transaction, error) {
	block, err := fetchBlockFromDB(blockNum)
	if err != nil {
		return nil, err
	}
	return block.GetTransactions()[txIndex], nil
}

func fetchBlockchainSizeFromDB() (uint64, error) {
	bytes, err := db.GetDBHandle().GetFromBlockchainCF(blockCountKey)
	if err != nil {
		return 0, err
	}
	if bytes == nil {
		return 0, nil
	}
	return decodeToUint64(bytes), nil
}

func (blockchain *Blockchain) persistBlock(block *protos.Block, blockNumber uint64) error {
	state := GetState()
	blockBytes, blockBytesErr := block.Bytes()
	if blockBytesErr != nil {
		return blockBytesErr
	}
	writeBatch := gorocksdb.NewWriteBatch()
	writeBatch.PutCF(db.GetDBHandle().BlockchainCF, encodeBlockNumberDBKey(blockNumber), blockBytes)

	sizeBytes := encodeUint64(blockNumber + 1)
	writeBatch.PutCF(db.GetDBHandle().BlockchainCF, blockCountKey, sizeBytes)

	state.addChangesForPersistence(writeBatch)

	opt := gorocksdb.NewDefaultWriteOptions()
	err := db.GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		return err
	}
	return nil
}

var blockCountKey = []byte("blockCount")

func encodeBlockNumberDBKey(blockNumber uint64) []byte {
	return encodeUint64(blockNumber)
}

func decodeBlockNumberDBKey(dbKey []byte) uint64 {
	return decodeToUint64(dbKey)
}

func encodeUint64(number uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, number)
	return bytes
}

func decodeToUint64(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}

func (blockchain *Blockchain) String() string {
	var buffer bytes.Buffer
	size := blockchain.GetSize()
	for i := uint64(0); i < size; i++ {
		block, blockErr := blockchain.GetBlock(i)
		if blockErr != nil {
			return ""
		}
		buffer.WriteString("\n----------<block>----------\n")
		buffer.WriteString(block.String())
		buffer.WriteString("\n----------<\\block>----------\n")
	}
	return buffer.String()
}
