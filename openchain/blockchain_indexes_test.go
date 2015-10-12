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
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"testing"
)

var chain *Blockchain
var blocks []*protos.Block
var setupDone bool

func setup(t *testing.T) {
	if setupDone {
		return
	}
	viper.Set("peer.db.path", "/tmp/openchain/db")
	chain = initTestBlockChain(t)
	blocks, _ = buildSimpleChain(t)
	setupDone = true
}

func TestIndexes_GetBlockByBlockNumber(t *testing.T) {
	setup(t)
	for i := range blocks {
		compareProtoMessages(t, getBlock(t, i), blocks[i])
	}
}

func TestIndexes_GetBlockByBlockHash(t *testing.T) {
	setup(t)
	for i := range blocks {
		compareProtoMessages(t, getBlockByHash(t, getBlockHash(t, blocks[i])), blocks[i])
	}
}

func TestIndexes_GetTransactionByBlockNumberAndIndex(t *testing.T) {
	setup(t)
	for i, block := range blocks {
		for j, tx := range block.GetTransactions() {
			compareProtoMessages(t, getTransactionByBlockNumberAndIndex(t, i, j), tx)
		}
	}
}

func TestIndexes_GetTransactionByBlockHashAndIndex(t *testing.T) {
	setup(t)
	for _, block := range blocks {
		for j, tx := range block.GetTransactions() {
			compareProtoMessages(t, getTransactionByBlockHashAndIndex(t, getBlockHash(t, block), j), tx)
		}
	}
}

func getBlockByHash(t *testing.T, blockHash []byte) *protos.Block {
	chain := getBlockchain(t)
	block, err := chain.GetBlockByHash(blockHash)
	if err != nil {
		t.Fatalf("Error while retrieving block from chain %s", err)
	}
	return block
}

func getTransactionByBlockNumberAndIndex(t *testing.T, blockNumber int, txIndex int) *protos.Transaction {
	chain := getBlockchain(t)
	tx, err := chain.GetTransaction(uint64(blockNumber), uint64(txIndex))
	if err != nil {
		t.Fatalf("Error in API blockchain.GetTransaction(): %s", err)
	}
	return tx
}

func getTransactionByBlockHashAndIndex(t *testing.T, blockHash []byte, txIndex int) *protos.Transaction {
	chain := getBlockchain(t)
	tx, err := chain.GetTransactionByBlockHash(blockHash, uint64(txIndex))
	if err != nil {
		t.Fatalf("Error in API blockchain.GetTransaction(): %s", err)
	}
	return tx
}

func compareProtoMessages(t *testing.T, found proto.Message, expected proto.Message) {
	if bytes.Compare(serializeProtoMessage(t, found), serializeProtoMessage(t, expected)) != 0 {
		t.Fatalf("Proto messages are not same. Expected = [%s], found = [%s]", expected, found)
	}
}

func serializeProtoMessage(t *testing.T, msg proto.Message) []byte {
	t.Logf("message = [%s]", msg)
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("Error while serializing proto message: %s", err)
	}
	return data
}
