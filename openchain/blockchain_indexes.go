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
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/tecbot/gorocksdb"
)

var indexLogger = logging.MustGetLogger("indexes")

var lastIndexedBlockKey = []byte{byte(0)}
var prefixBlockHashKey = byte(1)
var prefixTxGUIDKey = byte(2)
var prefixAddressBlockNumCompositeKey = byte(3)

type blockWrapper struct {
	block       *protos.Block
	blockNumber uint64
	blockHash   []byte
}

// Channel for transferring block from block chain for indexing
var blockChan = make(chan blockWrapper)

// block number tracker for making sure that client query results include upto latest block indexed
var blockNumberTkr *blockNumberTracker

// If an error occurs during indexing, store it here and index queries are returned the error
var indexingError error

func createIndexesAsync(block *protos.Block, blockNumber uint64, blockHash []byte) {
	blockChan <- blockWrapper{block, blockNumber, blockHash}
}

func startIndexer() error {
	lastIndexedBlockNum, err := indexPendingBlocks()
	if err != nil {
		return err
	}

	blockNumberTkr = newBlockNumberTracker(lastIndexedBlockNum)

	go func() {
		for {
			indexLogger.Debug("Going to wait on channel for next block to index")
			blockWrapper := <-blockChan

			if indexingError != nil {
				indexLogger.Debug(
					"Not indexing block number [%d]. Because of previous error: %s.",
					blockWrapper.blockNumber, err)
				continue
			}

			err := createIndexes(blockWrapper.block, blockWrapper.blockNumber, blockWrapper.blockHash)
			if err != nil {
				indexLogger.Debug(
					"Error occured while indexing block number [%d]. Error: %s. Further blocks will not be indexed",
					blockWrapper.blockNumber, err)
				indexingError = err
			}
			indexLogger.Debug("Finished indexing block number [%d]", blockWrapper.blockNumber)
		}
	}()
	return nil
}

// createIndexes adds entries into db for creating indexes on various atributes
func createIndexes(block *protos.Block, blockNumber uint64, blockHash []byte) error {
	openchainDB := db.GetDBHandle()
	writeBatch := gorocksdb.NewWriteBatch()
	cf := openchainDB.IndexesCF

	// add blockhash -> blockNumber
	indexLogger.Debug("Indexing block number [%d] by hash = [%x]", blockNumber, blockHash)
	writeBatch.PutCF(cf, encodeBlockHashKey(blockHash), encodeBlockNumber(blockNumber))

	addressToTxIndexesMap := make(map[string][]uint64)
	addressToChainletIDsMap := make(map[string][]*protos.ChainletID)

	transactions := block.GetTransactions()
	for txIndex, tx := range transactions {
		// add TxGUID -> (blockNumber,indexWithinBlock)
		writeBatch.PutCF(cf, encodeTxGUIDKey(getTxGUIDBytes(tx)), encodeBlockNumTxIndex(blockNumber, uint64(txIndex)))

		txExecutingAddress := getTxExecutingAddress(tx)
		addressToTxIndexesMap[txExecutingAddress] = append(addressToTxIndexesMap[txExecutingAddress], uint64(txIndex))

		switch tx.Type {
		case protos.Transaction_CHAINLET_NEW, protos.Transaction_CHAINLET_UPDATE:
			authroizedAddresses, chainletID := getAuthorisedAddresses(tx)
			for _, authroizedAddress := range authroizedAddresses {
				addressToChainletIDsMap[authroizedAddress] = append(addressToChainletIDsMap[authroizedAddress], chainletID)
			}
		}
	}

	for address, txsIndexes := range addressToTxIndexesMap {
		writeBatch.PutCF(cf, encodeAddressBlockNumCompositeKey(address, blockNumber), encodeListTxIndexes(txsIndexes))
	}
	writeBatch.PutCF(cf, lastIndexedBlockKey, encodeBlockNumber(blockNumber))
	opt := gorocksdb.NewDefaultWriteOptions()
	err := openchainDB.DB.Write(opt, writeBatch)
	if err != nil {
		return err
	}

	blockNumberTkr.blockIndexed(blockNumber)
	return nil
}

func fetchBlockByHash(blockHash []byte) (*protos.Block, error) {
	err := checkIndexingError()
	if err != nil {
		return nil, err
	}
	blockNumberTkr.waitForLastCommittedBlock()
	blockNumberBytes, err := db.GetDBHandle().GetFromIndexesCF(encodeBlockHashKey(blockHash))
	if err != nil {
		return nil, err
	}
	blockNumber := decodeBlockNumber(blockNumberBytes)
	return fetchBlockFromDB(blockNumber)
}

func fetchTransactionByGUID(txGUID []byte) (*protos.Transaction, error) {
	err := checkIndexingError()
	if err != nil {
		return nil, err
	}
	blockNumberTkr.waitForLastCommittedBlock()
	blockNumTxIndexBytes, err := db.GetDBHandle().GetFromIndexesCF(encodeTxGUIDKey(txGUID))
	if err != nil {
		return nil, err
	}
	blockNum, txIndex, err := decodeBlockNumTxIndex(blockNumTxIndexBytes)
	if err != nil {
		return nil, err
	}
	return fetchTransactionFromDB(blockNum, txIndex)
}

// fetch TxGUID
func getTxGUIDBytes(tx *protos.Transaction) (guid []byte) {
	guid = []byte("TODO:Fetch guid from Tx")
	return
}

// encode / decode BlockNumber
func encodeBlockNumber(blockNumber uint64) []byte {
	return proto.EncodeVarint(blockNumber)
}

func decodeBlockNumber(blockNumberBytes []byte) (blockNumber uint64) {
	blockNumber, _ = proto.DecodeVarint(blockNumberBytes)
	return
}

// encode / decode BlockNumTxIndex
func encodeBlockNumTxIndex(blockNumber uint64, txIndexInBlock uint64) []byte {
	b := proto.NewBuffer([]byte{})
	b.EncodeVarint(blockNumber)
	b.EncodeVarint(txIndexInBlock)
	return b.Bytes()
}

func decodeBlockNumTxIndex(bytes []byte) (blockNum uint64, txIndex uint64, err error) {
	b := proto.NewBuffer(bytes)
	blockNum, err = b.DecodeVarint()
	if err != nil {
		return
	}
	txIndex, err = b.DecodeVarint()
	if err != nil {
		return
	}
	return
}

// encode BlockHashKey
func encodeBlockHashKey(blockHash []byte) []byte {
	return prependKeyPrefix(prefixBlockHashKey, blockHash)
}

// encode TxGUIDKey
func encodeTxGUIDKey(guidBytes []byte) []byte {
	return prependKeyPrefix(prefixTxGUIDKey, guidBytes)
}

func encodeAddressBlockNumCompositeKey(address string, blockNumber uint64) []byte {
	b := proto.NewBuffer([]byte{prefixAddressBlockNumCompositeKey})
	b.EncodeRawBytes([]byte(address))
	b.EncodeVarint(blockNumber)
	return b.Bytes()
}

func encodeListTxIndexes(listTx []uint64) []byte {
	b := proto.NewBuffer([]byte{})
	for i := range listTx {
		b.EncodeVarint(listTx[i])
	}
	return b.Bytes()
}

func encodeChainletID(c *protos.ChainletID) []byte {
	// TODO serialize chainletID
	return []byte{}
}

func getTxExecutingAddress(tx *protos.Transaction) string {
	// TODO Fetch address form tx
	return "address1"
}

func getAuthorisedAddresses(tx *protos.Transaction) ([]string, *protos.ChainletID) {
	// TODO fetch address from chaincode deployment tx
	return []string{"address1", "address2"}, tx.GetChainletID()
}

func prependKeyPrefix(prefix byte, key []byte) []byte {
	modifiedKey := []byte{}
	modifiedKey = append(modifiedKey, prefix)
	modifiedKey = append(modifiedKey, key...)
	return modifiedKey
}

func indexPendingBlocks() (lastIndexedBlockNum uint64, err error) {
	lastIndexedBlockNumberBytes, err := db.GetDBHandle().GetFromIndexesCF(lastIndexedBlockKey)
	if err != nil {
		return
	}
	if lastIndexedBlockNumberBytes == nil {
		return
	}

	lastIndexedBlockNum = decodeBlockNumber(lastIndexedBlockNumberBytes)
	blockchain, err := GetBlockchain()
	if err != nil {
		return
	}

	if blockchain.GetSize() == 0 {
		// chain is empty as yet
		return
	}

	lastCommittedBlockNum := blockchain.GetSize() - 1
	if lastCommittedBlockNum == lastIndexedBlockNum {
		// all committed blocks are indexed
		return
	}

	for ; lastIndexedBlockNum < lastCommittedBlockNum; lastIndexedBlockNum++ {
		blockNumToIndex := lastIndexedBlockNum + 1
		blockToIndex, errBlockFetch := blockchain.GetBlock(blockNumToIndex)
		if errBlockFetch != nil {
			err = errBlockFetch
			return
		}

		blockHash, errBlockHash := blockToIndex.GetHash()
		if errBlockHash != nil {
			err = errBlockHash
			return
		}
		createIndexes(blockToIndex, blockNumToIndex, blockHash)
	}
	return
}

func checkIndexingError() error {
	if indexingError != nil {
		return fmt.Errorf(
			"An error had occured during indexing block number [%d]. So, index is out of sync. Detail of the error = %s",
			blockNumberTkr.lastBlockIndexed+1, indexingError)
	}
	return nil
}

// Code related to tracking the block number that has been indexed.
// Since, we index blocks asynchronously, there may be a case when
// a client query arrives before a block has been indexed.
//
// Do we really need strict symantics such that an index query results
// should include upto block number (or higher) that may have been committed
// when user query arrives?
// If a delay of a couple of blocks are allowed, we can get rid of this synchronization stuff
type blockNumberTracker struct {
	lastBlockIndexed uint64
	newBlockIndexed  *sync.Cond
}

func newBlockNumberTracker(lastBlockIndexed uint64) *blockNumberTracker {
	var lock sync.Mutex
	blockNumberTracker := &blockNumberTracker{lastBlockIndexed, sync.NewCond(&lock)}
	return blockNumberTracker
}

func (tracker *blockNumberTracker) blockIndexed(blockNumber uint64) {
	tracker.newBlockIndexed.L.Lock()
	defer tracker.newBlockIndexed.L.Unlock()
	tracker.lastBlockIndexed = blockNumber
	tracker.newBlockIndexed.Broadcast()
}

func (tracker *blockNumberTracker) waitForLastCommittedBlock() (err error) {
	chain, err := GetBlockchain()
	if err != nil || chain.GetSize() == 0 {
		return
	}

	lastBlockCommitted := chain.GetSize() - 1

	tracker.newBlockIndexed.L.Lock()
	defer tracker.newBlockIndexed.L.Unlock()

	for tracker.lastBlockIndexed < lastBlockCommitted {
		indexLogger.Debug(
			"Waiting for index to catch up with block chain. lastBlockCommitted=[%d] and lastBlockIndexed=[%d]",
			lastBlockCommitted, tracker.lastBlockIndexed)
		tracker.newBlockIndexed.Wait()
	}
	return
}
