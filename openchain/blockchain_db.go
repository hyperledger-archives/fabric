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
	"encoding/binary"
	"fmt"

	"github.com/openblockchain/obc-peer/protos"

	"github.com/tecbot/gorocksdb"
)

// BlockchainDB defines the database struct
type BlockchainDB struct {
	db *gorocksdb.DB
}

// OpenBlockchainDB opens an existing or creates a new blockchain DB
func OpenBlockchainDB(dbPath string, createIfMissing bool) (*BlockchainDB, error) {
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(createIfMissing)
	db, err := gorocksdb.OpenDb(opts, dbPath)
	if err != nil {
		fmt.Println("Error opening DB", err)
		return nil, err
	}
	return &BlockchainDB{db}, nil
}

var blockCountKey = []byte("blockCount")

// AddBlock adds a new block to the database.
func (blockchainDB *BlockchainDB) AddBlock(block protos.Block) error {

	size, sizeErr := blockchainDB.GetSize()
	if sizeErr != nil {
		return sizeErr
	}
	blockBytes, blockBytesErr := block.Bytes()
	if blockBytesErr != nil {
		return blockBytesErr
	}
	size++
	sizeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBytes, size)

	blockKeyBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockKeyBytes, size-1)

	writeBatch := gorocksdb.NewWriteBatch()
	writeBatch.Put(blockKeyBytes, blockBytes)
	writeBatch.Put(blockCountKey, sizeBytes)

	// TODO Synchrnous write?
	writeOpts := gorocksdb.NewDefaultWriteOptions()
	err := blockchainDB.db.Write(writeOpts, writeBatch)
	if err != nil {
		return err
	}

	return nil
}

// GetLastBlock returns the last block in the blockchain.
func (blockchainDB *BlockchainDB) GetLastBlock() (*protos.Block, error) {
	size, sizeErr := blockchainDB.GetSize()
	if sizeErr != nil {
		return nil, sizeErr
	}
	return blockchainDB.GetBlock(size - 1)
}

// GetBlock returns the specified block number. Like an array, block numbers
// start at 0.
func (blockchainDB *BlockchainDB) GetBlock(blockNumber uint64) (*protos.Block, error) {

	blockNumberBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockNumberBytes, blockNumber)

	readOpts := gorocksdb.NewDefaultReadOptions()
	blockBytes, readErr := blockchainDB.db.GetBytes(readOpts, blockNumberBytes)
	if readErr != nil {
		return nil, readErr
	}

	return protos.UnmarshallBlock(blockBytes)

}

// GetSize returns the size of the blockchain.
func (blockchainDB *BlockchainDB) GetSize() (uint64, error) {
	readOpts := gorocksdb.NewDefaultReadOptions()
	size, err := blockchainDB.db.GetBytes(readOpts, blockCountKey)
	if err != nil {
		return 0, err
	}
	if size == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint64(size), nil
}
