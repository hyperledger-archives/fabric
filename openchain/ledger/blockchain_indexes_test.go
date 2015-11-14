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
	"bytes"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

func TestIndexes_GetBlockByBlockNumber(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)

	for i := range blocks {
		compareProtoMessages(t, getBlock(t, i), blocks[i])
	}
}

func TestIndexes_GetBlockByBlockHash(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)

	for i := range blocks {
		compareProtoMessages(t, getBlockByHash(t, getBlockHash(t, blocks[i])), blocks[i])
	}
}

func TestIndexes_GetTransactionByBlockNumberAndTxIndex(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)

	for i, block := range blocks {
		for j, tx := range block.GetTransactions() {
			compareProtoMessages(t, getTransactionByBlockNumberAndIndex(t, i, j), tx)
		}
	}
}

func TestIndexes_GetTransactionByBlockHashAndTxIndex(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)

	for _, block := range blocks {
		for j, tx := range block.GetTransactions() {
			compareProtoMessages(t, getTransactionByBlockHashAndIndex(t, getBlockHash(t, block), j), tx)
		}
	}
}

func TestIndexes_GetTransactionByUUID(t *testing.T) {
	initTestBlockChain(t)
	chain := getTestBlockchain(t)
	tx1, uuid1 := buildTestTx()
	tx2, uuid2 := buildTestTx()
	block1 := protos.NewBlock("DummyBlock", []*protos.Transaction{tx1, tx2})
	chain.addBlock(context.TODO(), block1)

	tx3, uuid3 := buildTestTx()
	tx4, uuid4 := buildTestTx()
	block2 := protos.NewBlock("DummyBlock", []*protos.Transaction{tx3, tx4})
	chain.addBlock(context.TODO(), block2)

	compareProtoMessages(t, getTransactionByUUID(t, uuid1), tx1)
	compareProtoMessages(t, getTransactionByUUID(t, uuid2), tx2)
	compareProtoMessages(t, getTransactionByUUID(t, uuid3), tx3)
	compareProtoMessages(t, getTransactionByUUID(t, uuid4), tx4)
}

func getBlockByHash(t *testing.T, blockHash []byte) *protos.Block {
	chain := getTestBlockchain(t)
	block, err := chain.getBlockByHash(blockHash)
	if err != nil {
		t.Fatalf("Error while retrieving block from chain %s", err)
	}
	return block
}

func getTransactionByBlockNumberAndIndex(t *testing.T, blockNumber int, txIndex int) *protos.Transaction {
	chain := getTestBlockchain(t)
	tx, err := chain.getTransaction(uint64(blockNumber), uint64(txIndex))
	if err != nil {
		t.Fatalf("Error in API blockchain.GetTransaction(): %s", err)
	}
	return tx
}

func getTransactionByBlockHashAndIndex(t *testing.T, blockHash []byte, txIndex int) *protos.Transaction {
	chain := getTestBlockchain(t)
	tx, err := chain.getTransactionByBlockHash(blockHash, uint64(txIndex))
	if err != nil {
		t.Fatalf("Error in API blockchain.getTransactionByBlockHash(): %s", err)
	}
	return tx
}

func getTransactionByUUID(t *testing.T, txUUID string) *protos.Transaction {
	chain := getTestBlockchain(t)
	tx, err := chain.getTransactionByUUID(txUUID)
	if err != nil {
		t.Fatalf("Error in API blockchain.getTransactionByUUID(): %s", err)
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
