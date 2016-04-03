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
	"fmt"
	"sync"

	"github.com/hyperledger/fabric/core/db"
	"github.com/hyperledger/fabric/protos"
	"github.com/tecbot/gorocksdb"
)

var lastIndexedBlockKey = []byte{byte(0)}

type blockWrapper struct {
	block       *protos.Block
	blockNumber uint64
	blockHash   []byte
	stopNow     bool
}

type blockchainIndexerAsync struct {
	blockchain *blockchain
	// Channel for transferring block from block chain for indexing
	blockChan    chan blockWrapper
	indexerState *blockchainIndexerState
}

func newBlockchainIndexerAsync() *blockchainIndexerAsync {
	return new(blockchainIndexerAsync)
}

func (indexer *blockchainIndexerAsync) isSynchronous() bool {
	return false
}

func (indexer *blockchainIndexerAsync) start(blockchain *blockchain) error {
	indexer.blockchain = blockchain
	indexerState, err := newBlockchainIndexerState(indexer)
	if err != nil {
		return err
	}
	indexer.indexerState = indexerState
	indexLogger.Debug("staring indexer, lastIndexedBlockNum = [%d]",
		indexer.indexerState.getLastIndexedBlockNumber())

	err = indexer.indexPendingBlocks()
	if err != nil {
		return err
	}
	indexLogger.Debug("staring indexer, lastIndexedBlockNum = [%d] after processing pending blocks",
		indexer.indexerState.getLastIndexedBlockNumber())
	indexer.blockChan = make(chan blockWrapper)
	go func() {
		for {
			indexLogger.Debug("Going to wait on channel for next block to index")
			blockWrapper := <-indexer.blockChan

			indexLogger.Debug("Blockwrapper received on channel: block number = [%d]", blockWrapper.blockNumber)

			if blockWrapper.stopNow {
				indexLogger.Debug("stop command received on channel. Closing channel")
				close(indexer.blockChan)
				return
			}
			if indexer.indexerState.hasError() {
				indexLogger.Debug("Not indexing block number [%d]. Because of previous error: %s.",
					blockWrapper.blockNumber, indexer.indexerState.getError())
				continue
			}

			err := indexer.createIndexesInternal(blockWrapper.block, blockWrapper.blockNumber, blockWrapper.blockHash)
			if err != nil {
				indexer.indexerState.setError(err)
				indexLogger.Debug(
					"Error occured while indexing block number [%d]. Error: %s. Further blocks will not be indexed",
					blockWrapper.blockNumber, err)

			} else {
				indexLogger.Debug("Finished indexing block number [%d]", blockWrapper.blockNumber)
			}
		}
	}()
	return nil
}

func (indexer *blockchainIndexerAsync) createIndexesSync(
	block *protos.Block, blockNumber uint64, blockHash []byte, writeBatch *gorocksdb.WriteBatch) error {
	return fmt.Errorf("Method not applicable")
}

func (indexer *blockchainIndexerAsync) createIndexesAsync(block *protos.Block, blockNumber uint64, blockHash []byte) error {
	indexer.blockChan <- blockWrapper{block, blockNumber, blockHash, false}
	return nil
}

// createIndexes adds entries into db for creating indexes on various atributes
func (indexer *blockchainIndexerAsync) createIndexesInternal(block *protos.Block, blockNumber uint64, blockHash []byte) error {
	openchainDB := db.GetDBHandle()
	writeBatch := gorocksdb.NewWriteBatch()
	defer writeBatch.Destroy()
	addIndexDataForPersistence(block, blockNumber, blockHash, writeBatch)
	writeBatch.PutCF(openchainDB.IndexesCF, lastIndexedBlockKey, encodeBlockNumber(blockNumber))
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	err := openchainDB.DB.Write(opt, writeBatch)
	if err != nil {
		return err
	}
	indexer.indexerState.blockIndexed(blockNumber)
	return nil
}

func (indexer *blockchainIndexerAsync) fetchBlockNumberByBlockHash(blockHash []byte) (uint64, error) {
	err := indexer.indexerState.checkError()
	if err != nil {
		return 0, err
	}
	indexer.indexerState.waitForLastCommittedBlock()
	return fetchBlockNumberByBlockHashFromDB(blockHash)
}

func (indexer *blockchainIndexerAsync) fetchTransactionIndexByUUID(txUUID string) (uint64, uint64, error) {
	err := indexer.indexerState.checkError()
	if err != nil {
		return 0, 0, err
	}
	indexer.indexerState.waitForLastCommittedBlock()
	return fetchTransactionIndexByUUIDFromDB(txUUID)
}

func (indexer *blockchainIndexerAsync) indexPendingBlocks() error {
	blockchain := indexer.blockchain
	if blockchain.getSize() == 0 {
		// chain is empty as yet
		return nil
	}

	lastCommittedBlockNum := blockchain.getSize() - 1
	lastIndexedBlockNum := indexer.indexerState.getLastIndexedBlockNumber()
	if lastCommittedBlockNum == lastIndexedBlockNum {
		// all committed blocks are indexed
		return nil
	}

	for ; lastIndexedBlockNum < lastCommittedBlockNum; lastIndexedBlockNum++ {
		blockNumToIndex := lastIndexedBlockNum + 1
		blockToIndex, errBlockFetch := blockchain.getBlock(blockNumToIndex)
		if errBlockFetch != nil {
			return errBlockFetch
		}

		blockHash, errBlockHash := blockToIndex.GetHash()
		if errBlockHash != nil {
			return errBlockHash
		}
		indexer.createIndexesInternal(blockToIndex, blockNumToIndex, blockHash)
	}
	return nil
}

func (indexer *blockchainIndexerAsync) stop() {
	indexer.indexerState.waitForLastCommittedBlock()
	indexer.blockChan <- blockWrapper{nil, 0, nil, true}
}

// Code related to tracking the block number that has been indexed
// and if there has been an error in indexing a block
// Since, we index blocks asynchronously, there may be a case when
// a client query arrives before a block has been indexed.
//
// Do we really need strict symantics such that an index query results
// should include upto block number (or higher) that may have been committed
// when user query arrives?
// If a delay of a couple of blocks are allowed, we can get rid of this synchronization stuff
type blockchainIndexerState struct {
	indexer *blockchainIndexerAsync

	zerothBlockIndexed bool
	lastBlockIndexed   uint64
	err                error
	lock               *sync.RWMutex
	newBlockIndexed    *sync.Cond
}

func newBlockchainIndexerState(indexer *blockchainIndexerAsync) (*blockchainIndexerState, error) {
	var lock sync.RWMutex
	zerothBlockIndexed, lastIndexedBlockNum, err := fetchLastIndexedBlockNumFromDB()
	if err != nil {
		return nil, err
	}
	return &blockchainIndexerState{indexer, zerothBlockIndexed, lastIndexedBlockNum, nil, &lock, sync.NewCond(&lock)}, nil
}

func (indexerState *blockchainIndexerState) blockIndexed(blockNumber uint64) {
	indexerState.newBlockIndexed.L.Lock()
	defer indexerState.newBlockIndexed.L.Unlock()
	indexerState.lastBlockIndexed = blockNumber
	indexerState.zerothBlockIndexed = true
	indexerState.newBlockIndexed.Broadcast()
}

func (indexerState *blockchainIndexerState) getLastIndexedBlockNumber() uint64 {
	indexerState.lock.RLock()
	defer indexerState.lock.RUnlock()
	return indexerState.lastBlockIndexed
}

func (indexerState *blockchainIndexerState) waitForLastCommittedBlock() (err error) {
	chain := indexerState.indexer.blockchain
	if err != nil || chain.getSize() == 0 {
		return
	}

	lastBlockCommitted := chain.getSize() - 1

	indexerState.newBlockIndexed.L.Lock()
	defer indexerState.newBlockIndexed.L.Unlock()

	if !indexerState.zerothBlockIndexed {
		indexLogger.Debug(
			"Waiting for zeroth block to be indexed. lastBlockCommitted=[%d] and lastBlockIndexed=[%d]",
			lastBlockCommitted, indexerState.lastBlockIndexed)
		indexerState.newBlockIndexed.Wait()
	}

	for indexerState.lastBlockIndexed < lastBlockCommitted {
		indexLogger.Debug(
			"Waiting for index to catch up with block chain. lastBlockCommitted=[%d] and lastBlockIndexed=[%d]",
			lastBlockCommitted, indexerState.lastBlockIndexed)
		indexerState.newBlockIndexed.Wait()
	}
	return
}

func (indexerState *blockchainIndexerState) setError(err error) {
	indexerState.lock.Lock()
	defer indexerState.lock.Unlock()
	indexerState.err = err
}

func (indexerState *blockchainIndexerState) hasError() bool {
	indexerState.lock.RLock()
	defer indexerState.lock.RUnlock()
	return indexerState.err != nil
}

func (indexerState *blockchainIndexerState) getError() error {
	indexerState.lock.RLock()
	defer indexerState.lock.RUnlock()
	return indexerState.err
}

func (indexerState *blockchainIndexerState) checkError() error {
	indexerState.lock.RLock()
	defer indexerState.lock.RUnlock()
	if indexerState.err != nil {
		return fmt.Errorf(
			"An error had occured during indexing block number [%d]. So, index is out of sync. Detail of the error = %s",
			indexerState.getLastIndexedBlockNumber()+1, indexerState.err)
	}
	return indexerState.err
}

func fetchLastIndexedBlockNumFromDB() (zerothBlockIndexed bool, lastIndexedBlockNum uint64, err error) {
	lastIndexedBlockNumberBytes, err := db.GetDBHandle().GetFromIndexesCF(lastIndexedBlockKey)
	if err != nil {
		return
	}
	if lastIndexedBlockNumberBytes == nil {
		return
	}
	lastIndexedBlockNum = decodeBlockNumber(lastIndexedBlockNumberBytes)
	zerothBlockIndexed = true
	return
}
