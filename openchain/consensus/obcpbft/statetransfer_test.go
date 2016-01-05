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
	"encoding/base64"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func newTestStateTransfer(ml *mockLedger) *stateTransferState {
	// The implementation of the state transfer depends on randomness
	// Seed it here so that independent test executions are consistent
	rand.Seed(0)

	return newStateTransferState(
		&pbftCore{
			replicaCount:    4,
			id:              uint64(0),
			h:               uint64(0),
			K:               uint64(2),
			L:               uint64(4),
			f:               int(1),
			checkpointStore: make(map[Checkpoint]bool),
		},
		readConfig(),
		ml,
	)
}

func executeStateTransfer(sts *stateTransferState, ml *mockLedger, blockNumber, sequenceNumber uint64) error {

	var chkpt *Checkpoint

	ml.forceRemoteStateBlock(blockNumber - 2)

	for i := uint64(1); i <= 3; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber - i,
			BlockHash:      base64.StdEncoding.EncodeToString(simpleGetBlockHash(blockNumber - i)),
			ReplicaId:      i,
			BlockNumber:    blockNumber - i,
		}
		sts.WitnessCheckpoint(chkpt)
	}

	if !sts.OutOfDate {
		return fmt.Errorf("Replica did not detect itself falling behind to initiate the state transfer")
	}

	for i := 1; i < sts.pbft.replicaCount; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber,
			BlockHash:      base64.StdEncoding.EncodeToString(simpleGetBlockHash(blockNumber)),
			ReplicaId:      uint64(i),
			BlockNumber:    blockNumber,
		}
		sts.pbft.checkpointStore[*chkpt] = true
	}

	go func() {
		for {
			// In ordinary operation, the weak cert would advance, but to simply testing, send it over and over again
			time.Sleep(time.Millisecond * 10)
			sts.WitnessCheckpointWeakCert(chkpt)
		}
	}()

	select {
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timed out waiting for state to catch up, error in state transfer")
	case <-sts.completeStateSync:
		// Do nothing, continue the test
	}

	if size, _ := sts.ledger.GetBlockchainSize(); size != blockNumber+1 { // Will never fail
		return fmt.Errorf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, size)
	}

	block, err := sts.ledger.GetBlock(blockNumber)

	if nil != err {
		return fmt.Errorf("Error retrieving last block in the mock chain.")
	}

	if stateHash, _ := sts.ledger.GetCurrentStateHash(); !bytes.Equal(stateHash, block.StateHash) {
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

	// Test from blockheight of 1, with valid genesis block
	ml := newMockLedger(nil)
	ml.PutBlock(0, simpleGetBlock(0))
	sts := newTestStateTransfer(ml)
	if err := executeStateTransfer(sts, ml, 7, 10); nil != err {
		t.Fatalf("Simplest case: %s", err)
	}

}

func TestCatchupSyncBlocksTimeout(t *testing.T) {
	// Test from blockheight of 1 with valid genesis block
	// Timeouts of 1 second
	filter, result := makeSimpleFilter(SyncBlocks, Timeout)
	ml := newMockLedger(filter)

	ml.PutBlock(0, simpleGetBlock(0))
	sts := newTestStateTransfer(ml)
	sts.blockRequestTimeout = 10 * time.Millisecond
	if err := executeStateTransfer(sts, ml, 7, 10); nil != err {
		t.Fatalf("SyncSnapshotTimeout case: %s", err)
	}
	if !result.wasTriggered() {
		t.Fatalf("SyncSnapshotTimeout case never simulated a timeout")
	}
}

func TestCatchupMissingEarlyChain(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3)
	ml := newMockLedger(nil)
	ml.PutBlock(4, simpleGetBlock(4))
	sts := newTestStateTransfer(ml)
	if err := executeStateTransfer(sts, ml, 7, 10); nil != err {
		t.Fatalf("MissingEarlyChain case: %s", err)
	}
}

func TestCatchupSyncSnapshotTimeout(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3)
	// Timeouts of 1 second
	filter, result := makeSimpleFilter(SyncSnapshot, Timeout)
	ml := newMockLedger(filter)
	ml.PutBlock(4, simpleGetBlock(4))
	sts := newTestStateTransfer(ml)
	sts.stateSnapshotRequestTimeout = 10 * time.Millisecond
	if err := executeStateTransfer(sts, ml, 7, 10); nil != err {
		t.Fatalf("SyncSnapshotTimeout case: %s", err)
	}
	if !result.wasTriggered() {
		t.Fatalf("SyncSnapshotTimeout case never simulated a timeout")
	}
}

func TestCatchupSyncDeltasTimeout(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3)
	// Timeouts of 1 second
	filter, result := makeSimpleFilter(SyncDeltas, Timeout)
	ml := newMockLedger(filter)
	ml.PutBlock(4, simpleGetBlock(4))
	sts := newTestStateTransfer(ml)
	sts.stateDeltaRequestTimeout = 10 * time.Millisecond
	if err := executeStateTransfer(sts, ml, 7, 10); nil != err {
		t.Fatalf("SyncDeltasTimeout case: %s", err)
	}
	if !result.wasTriggered() {
		t.Fatalf("SyncDeltasTimeout case never simulated a timeout")
	}
}

func executeBlockRecovery(ml *mockLedger, millisTimeout int) error {
	sts := threadlessNewStateTransferState(
		&pbftCore{
			replicaCount:    4,
			id:              uint64(0),
			h:               uint64(0),
			K:               uint64(2),
			L:               uint64(4),
			f:               int(1),
			checkpointStore: make(map[Checkpoint]bool),
		},
		readConfig(),
		ml,
	)
	sts.blockRequestTimeout = time.Duration(millisTimeout) * time.Millisecond
	sts.recoverDamage = true

	w := make(chan struct{})

	go func() {
		for !sts.verifyAndRecoverBlockchain() {
		}
		w <- struct{}{}
	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Errorf("Timed out waiting for blocks to replicate for blockchain")
	case <-w:
		// Do nothing, continue the test
	}

	if n, err := ml.VerifyBlockchain(7, 0); 0 != n || nil != err {
		fmt.Errorf("Blockchain claims to be up to date, but does not verify")
	}

	return nil
}

func executeBlockRecoveryWithPanic(ml *mockLedger, millisTimeout int) error {
	sts := threadlessNewStateTransferState(
		&pbftCore{
			replicaCount:    4,
			id:              uint64(0),
			h:               uint64(0),
			K:               uint64(2),
			L:               uint64(4),
			f:               int(1),
			checkpointStore: make(map[Checkpoint]bool),
		},
		readConfig(),
		ml,
	)
	sts.blockRequestTimeout = time.Duration(millisTimeout) * time.Millisecond
	sts.recoverDamage = false

	w := make(chan bool)

	go func() {
		defer func() {
			recover()
			w <- true
		}()
		for !sts.verifyAndRecoverBlockchain() {
		}
		w <- false
	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Errorf("Timed out waiting for blocks to replicate for blockchain")
	case didPanic := <-w:
		// Do nothing, continue the test
		if !didPanic {
			fmt.Errorf("Blockchain was supposed to panic on modification, but did not")
		}
	}

	return nil
}

func TestCatchupLaggingChains(t *testing.T) {
	ml := newMockLedger(nil)
	ml.PutBlock(7, simpleGetBlock(7))
	if err := executeBlockRecovery(ml, 10); nil != err {
		t.Fatalf("TestCatchupLaggingChains short chain failure: %s", err)
	}

	ml = newMockLedger(nil)
	ml.PutBlock(700, simpleGetBlock(700))
	// Use a large timeout here because the mock ledger is slow for large blocks
	if err := executeBlockRecovery(ml, 1000); nil != err {
		t.Fatalf("TestCatchupLaggingChains long chain failure: %s", err)
	}

	filter, result := makeSimpleFilter(SyncBlocks, Timeout)
	ml = newMockLedger(filter)
	ml.PutBlock(7, simpleGetBlock(7))
	if err := executeBlockRecovery(ml, 10); nil != err {
		t.Fatalf("TestCatchupLaggingChains short chain with timeout failure: %s", err)
	}
	if !result.wasTriggered() {
		t.Fatalf("TestCatchupLaggingChains short chain with timeout never simulated a timeout")
	}
}

func TestCatchupCorruptChains(t *testing.T) {
	ml := newMockLedger(nil)
	ml.PutBlock(7, simpleGetBlock(7))
	ml.PutBlock(3, simpleGetBlock(2))
	if err := executeBlockRecovery(ml, 10); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}

	ml = newMockLedger(nil)
	ml.PutBlock(7, simpleGetBlock(7))
	ml.PutBlock(3, simpleGetBlock(2))
	defer func() {
		//fmt.Println("Executing defer")
		// We expect a panic, this is great
		recover()
	}()
	if err := executeBlockRecoveryWithPanic(ml, 10); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}
}
