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

var lastBlockIndexed uint64

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
		return nil
	}
	return nil
}

func fetchBlockByHash(blockHash []byte) (*protos.Block, error) {
	blockNumberBytes, err := db.GetDBHandle().GetFromIndexesCF(encodeBlockHashKey(blockHash))
	if err != nil {
		return nil, err
	}
	blockNumber := decodeBlockNumber(blockNumberBytes)
	return fetchBlockFromDB(blockNumber)
}

func fetchTransactionByGUID(txGUID []byte) (*protos.Transaction, error) {
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
