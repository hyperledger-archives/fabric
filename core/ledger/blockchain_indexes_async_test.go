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
	"github.com/hyperledger/fabric/core/ledger/testutil"
	"testing"
	"time"
)

func TestIndexesAsync_IndexingErrorScenario(t *testing.T) {
	testDBWrapper.CreateFreshDB(t)
	testBlockchainWrapper := newTestBlockchainWrapper(t)
	chain := testBlockchainWrapper.blockchain
	if chain.indexer.isSynchronous() {
		t.Skip("Skipping because blockchain is configured to index block data synchronously")
	}

	blocks, _, err := testBlockchainWrapper.populateBlockChainWithSampleData()
	if err != nil {
		t.Logf("Error populating block chain with sample data: %s", err)
		t.Fail()
	}
	asyncIndexer, _ := chain.indexer.(*blockchainIndexerAsync)
	t.Log("Setting an error artificially so as to client query gets an error")
	asyncIndexer.indexerState.setError(errors.New("Error created for testing"))
	blockHash, _ := blocks[0].GetHash()
	// index query should throw error
	_, err = chain.getBlockByHash(blockHash)
	if err == nil {
		t.Fatal("Error expected during execution of client query")
	}
	asyncIndexer.indexerState.setError(nil)
}

func TestIndexesAsync_ClientWaitScenario(t *testing.T) {
	testDBWrapper.CreateFreshDB(t)
	testBlockchainWrapper := newTestBlockchainWrapper(t)
	chain := testBlockchainWrapper.blockchain
	if chain.indexer.isSynchronous() {
		t.Skip("Skipping because blockchain is configured to index block data synchronously")
	}
	blocks, _, err := testBlockchainWrapper.populateBlockChainWithSampleData()
	if err != nil {
		t.Logf("Error populating block chain with sample data: %s", err)
		t.Fail()
	}
	t.Log("Increasing size of blockchain by one artificially so as to make client wait")
	chain.size = chain.size + 1
	t.Log("Resetting size of blockchain to original and adding one block in a separate go routine so as to wake up the client")
	go func() {
		time.Sleep(2 * time.Second)
		chain.size = chain.size - 1
		blk,err := buildTestBlock(t)
		if err != nil {
			t.Logf("Error building test block: %s", err)
			t.Fail()
		}
		testBlockchainWrapper.addNewBlock(blk, []byte("stateHash"))
	}()
	t.Log("Executing client query. The client would wait and will be woken up")
	blockHash, _ := blocks[0].GetHash()
	block := testBlockchainWrapper.getBlockByHash(blockHash)
	testutil.AssertEquals(t, block, blocks[0])
}
