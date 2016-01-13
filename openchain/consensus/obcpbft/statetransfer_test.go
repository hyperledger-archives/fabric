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

package obcpbft

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	. "github.com/openblockchain/obc-peer/openchain/consensus/statetransfer" // Bad form, but here until we can figure out how to share tests across packages
)

func newTestStateTransfer(ml *MockLedger) *StateTransferState {
	// The implementation of the state transfer depends on randomness
	// Seed it here so that independent test executions are consistent
	rand.Seed(0)

	return NewStateTransferState("State Transfer Test", readConfig(), ml)
}

func createRemoteLedgers(low, high uint64) (*map[uint64]ReadOnlyLedger, *map[uint64]*MockRemoteLedger) {
	rols := make(map[uint64]ReadOnlyLedger)
	mrls := make(map[uint64]*MockRemoteLedger)
	for i := low; i <= high; i++ {
		l := &MockRemoteLedger{}
		rols[i] = l
		mrls[i] = l
	}
	return &rols, &mrls
}

func executeStateTransfer(sts *StateTransferState, ml *MockLedger, blockNumber, sequenceNumber uint64, mrls *map[uint64]*MockRemoteLedger) error {

	for i := uint64(1); i <= 3; i++ {
		(*mrls)[i].blockHeight = blockNumber + 1
	}

	result := sts.AsynchronousStateTransfer(blockNumber-3, []uint64{1, 2, 3})

	blockHash := SimpleGetBlockHash(blockNumber)
	sts.AsynchronousStateTransferValidHash(blockNumber, blockHash, []uint64{1, 2, 3})

	select {
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timed out waiting for state to catch up, error in state transfer")
	case <-result:
		// Do nothing, continue the test
	}

	if size, _ := ml.GetBlockchainSize(); size != blockNumber+1 { // Will never fail
		return fmt.Errorf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, size)
	}

	block, err := ml.GetBlock(blockNumber)

	if nil != err {
		return fmt.Errorf("Error retrieving last block in the mock chain.")
	}

	if stateHash, _ := ml.GetCurrentStateHash(); !bytes.Equal(stateHash, block.StateHash) {
		return fmt.Errorf("Current state does not validate against the latest block.")
	}

	return nil
}

type filterResult struct {
	triggered bool
	replica   uint64
	mutex     *sync.Mutex
}

func (res filterResult) wasTriggered() bool {
	res.mutex.Lock()
	defer res.mutex.Unlock()
	return res.triggered
}

func makeSimpleFilter(failureTrigger mockRequest, failureType mockResponse) (func(mockRequest, uint64) mockResponse, *filterResult) {
	res := &filterResult{triggered: false, mutex: &sync.Mutex{}}
	return func(request mockRequest, replicaId uint64) mockResponse {
		//fmt.Println("Received a request", request, "for replicaId", replicaId)
		if request != failureTrigger {
			return Normal
		}

		res.mutex.Lock()
		defer res.mutex.Unlock()

		if !res.triggered {
			res.triggered = true
			res.replica = replicaId
		}

		if replicaId == res.replica {
			//fmt.Println("Failing it with", failureType)
			return failureType
		}
		return Normal
	}, res

}

func TestCatchupSimple(t *testing.T) {
	rols, mrls := createRemoteLedgers(1, 3)

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))

	sts := newTestStateTransfer(ml)
	if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
		t.Fatalf("Simplest case: %s", err)
	}

}

func TestCatchupSyncBlocksErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 1 with valid genesis block
		// Timeouts of 10 milliseconds
		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(rols, filter)

		ml.PutBlock(0, SimpleGetBlock(0))
		sts := newTestStateTransfer(ml)
		sts.BlockRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
			t.Fatalf("SyncBlocksErrors %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncBlocksErrors case never simulated a %v", failureType)
		}
	}
}

func TestCatchupMissingEarlyChain(t *testing.T) {
	rols, mrls := createRemoteLedgers(1, 3)

	// Test from blockheight of 5 (with missing blocks 0-3)
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(4, SimpleGetBlock(4))
	sts := newTestStateTransfer(ml)
	if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
		t.Fatalf("MissingEarlyChain case: %s", err)
	}
}

func TestCatchupSyncSnapshotError(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second, also test corrupt snapshot
		filter, result := makeSimpleFilter(SyncSnapshot, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml)
		sts.StateSnapshotRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
			t.Fatalf("SyncSnapshotError %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncSnapshotError case never simulated a %s", failureType)
		}
	}
}

func TestCatchupSyncDeltasError(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second
		filter, result := makeSimpleFilter(SyncDeltas, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml)
		sts.StateDeltaRequestTimeout = 10 * time.Millisecond
		sts.StateSnapshotRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
			t.Fatalf("SyncDeltasError %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncDeltasError case never simulated a %s", failureType)
		}
	}
}

func TestCatchupSimpleSynchronous(t *testing.T) {
	rols, mrls := createRemoteLedgers(1, 3)

	for i := uint64(1); i <= 3; i++ {
		(*mrls)[i].blockHeight = 8
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml)
	if err := sts.SynchronousStateTransfer(7, SimpleGetBlockHash(7), []uint64{1, 2, 3}); nil != err {
		t.Fatalf("SimpleSynchronous state transfer failed : %s", err)
	}
}

func executeBlockRecovery(ml *MockLedger, millisTimeout int) error {

	sts := ThreadlessNewStateTransferState("Replica 0", readConfig(), ml)
	sts.BlockRequestTimeout = time.Duration(millisTimeout) * time.Millisecond
	sts.RecoverDamage = true

	w := make(chan struct{})

	go func() {
		for !sts.VerifyAndRecoverBlockchain() {
		}
		w <- struct{}{}
	}()

	select {
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timed out waiting for blocks to replicate for blockchain")
	case <-w:
		// Do nothing, continue the test
	}

	if n, err := ml.VerifyBlockchain(7, 0); 0 != n || nil != err {
		return fmt.Errorf("Blockchain claims to be up to date, but does not verify")
	}

	return nil
}

func executeBlockRecoveryWithPanic(ml *MockLedger, millisTimeout int) error {

	sts := ThreadlessNewStateTransferState("Replica 0", readConfig(), ml)
	sts.BlockRequestTimeout = time.Duration(millisTimeout) * time.Millisecond
	sts.RecoverDamage = false

	w := make(chan bool)

	go func() {
		defer func() {
			recover()
			w <- true
		}()
		for !sts.VerifyAndRecoverBlockchain() {
		}
		w <- false
	}()

	select {
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timed out waiting for blocks to replicate for blockchain")
	case didPanic := <-w:
		// Do nothing, continue the test
		if !didPanic {
			return fmt.Errorf("Blockchain was supposed to panic on modification, but did not")
		}
	}

	return nil
}

func TestCatchupLaggingChains(t *testing.T) {
	rols, mrls := createRemoteLedgers(0, 3)

	for i := uint64(0); i <= 3; i++ {
		(*mrls)[i].blockHeight = 701
	}

	ml := NewMockLedger(rols, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	if err := executeBlockRecovery(ml, 10); nil != err {
		t.Fatalf("TestCatchupLaggingChains short chain failure: %s", err)
	}

	ml = NewMockLedger(rols, nil)
	ml.PutBlock(700, SimpleGetBlock(700))
	// Use a large timeout here because the mock ledger is slow for large blocks
	if err := executeBlockRecovery(ml, 1000); nil != err {
		t.Fatalf("TestCatchupLaggingChains long chain failure: %s", err)
	}
}

func TestCatchupLaggingChainsErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls := createRemoteLedgers(0, 3)

		for i := uint64(0); i <= 3; i++ {
			(*mrls)[i].blockHeight = 701
		}

		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(7, SimpleGetBlock(7))
		if err := executeBlockRecovery(ml, 10); nil != err {
			t.Fatalf("TestCatchupLaggingChainsErrors %s short chain with timeout failure: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("TestCatchupLaggingChainsErrors short chain with timeout never simulated a %s", failureType)
		}
	}
}

func TestCatchupCorruptChains(t *testing.T) {
	rols, mrls := createRemoteLedgers(0, 3)

	for i := uint64(0); i <= 3; i++ {
		(*mrls)[i].blockHeight = 701
	}

	ml := NewMockLedger(rols, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	ml.PutBlock(3, SimpleGetBlock(2))
	if err := executeBlockRecovery(ml, 10); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}

	ml = NewMockLedger(rols, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	ml.PutBlock(3, SimpleGetBlock(2))
	defer func() {
		//fmt.Println("Executing defer")
		// We expect a panic, this is great
		recover()
	}()
	if err := executeBlockRecoveryWithPanic(ml, 10); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}
}
