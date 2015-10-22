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
	"errors"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestIndexesAsync_IndexingErrorScenario(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)
	chain := getBlockchain(t)
	if chain.indexer.isSynchronous() {
		t.Skip("Skipping because blockchain is configured to index block data synchronously")
	}
	asyncIndexer, _ := chain.indexer.(*blockchainIndexerAsync)

	t.Log("Setting an error artificially so as to client query gets an error")
	asyncIndexer.indexerState.setError(errors.New("Error created for testing"))
	// index query should throw error
	_, err := chain.GetBlockByHash(getBlockHash(t, blocks[0]))
	if err == nil {
		t.Fatal("Error expected during execution of client query")
	}
	asyncIndexer.indexerState.setError(nil)
}

func TestIndexesAsync_ClientWaitScenario(t *testing.T) {
	initTestBlockChain(t)
	blocks, _ := buildSimpleChain(t)
	chain := getBlockchain(t)
	if chain.indexer.isSynchronous() {
		t.Skip("Skipping because blockchain is configured to index block data synchronously")
	}
	t.Log("Increasing size of blockchain by one artificially so as to make client wait")
	chain.size = chain.size + 1
	t.Log("Resetting size of blockchain to original and adding one block in a separate go routine so as to wake up the client")
	go func() {
		time.Sleep(2 * time.Second)
		chain.size = chain.size - 1
		chain.AddBlock(context.TODO(), buildTestBlock())
	}()
	t.Log("Executing client query. The client would wait and will be woken up")
	block := getBlockByHash(t, getBlockHash(t, blocks[0]))
	compareProtoMessages(t, block, blocks[0])
}
