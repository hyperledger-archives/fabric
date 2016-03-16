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
	"sync"
	"testing"
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	. "github.com/openblockchain/obc-peer/openchain/consensus/statetransfer" // Bad form, but here until we can figure out how to share tests across packages
	"github.com/openblockchain/obc-peer/protos"
)

type testPartialStack struct {
	*MockRemoteHashLedgerDirectory
	*MockLedger
}

func newPartialStack(ml *MockLedger, rld *MockRemoteHashLedgerDirectory) PartialStack {
	return &testPartialStack{
		MockLedger:                    ml,
		MockRemoteHashLedgerDirectory: rld,
	}
}

func newTestStateTransfer(ml *MockLedger, rld *MockRemoteHashLedgerDirectory) *StateTransferState {
	return NewStateTransferState(loadConfig(), newPartialStack(ml, rld))
}

func newTestThreadlessStateTransfer(ml *MockLedger, rld *MockRemoteHashLedgerDirectory) *StateTransferState {
	return ThreadlessNewStateTransferState(loadConfig(), newPartialStack(ml, rld))
}

type MockRemoteHashLedgerDirectory struct {
	*HashLedgerDirectory
}

func (mrls *MockRemoteHashLedgerDirectory) GetMockRemoteLedgerByPeerID(peerID *protos.PeerID) *MockRemoteLedger {
	ml, _ := mrls.GetLedgerByPeerID(peerID)
	return ml.(*MockRemoteLedger)
}

func createRemoteLedgers(low, high uint64) *MockRemoteHashLedgerDirectory {
	rols := make(map[protos.PeerID]consensus.ReadOnlyLedger)

	for i := low; i <= high; i++ {
		peerID := &protos.PeerID{
			Name: fmt.Sprintf("Peer %d", i),
		}
		l := &MockRemoteLedger{}
		rols[*peerID] = l
	}
	return &MockRemoteHashLedgerDirectory{&HashLedgerDirectory{rols}}
}

func executeStateTransfer(sts *StateTransferState, ml *MockLedger, blockNumber, sequenceNumber uint64, mrls *MockRemoteHashLedgerDirectory) error {

	for peerID, _ := range mrls.remoteLedgers {
		mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = blockNumber + 1
	}

	sts.Initiate(nil)
	result := sts.CompletionChannel()

	blockHash := SimpleGetBlockHash(blockNumber)
	sts.AddTarget(blockNumber, blockHash, nil, nil)

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
	peerID    *protos.PeerID
	mutex     *sync.Mutex
}

func (res filterResult) wasTriggered() bool {
	res.mutex.Lock()
	defer res.mutex.Unlock()
	return res.triggered
}

func makeSimpleFilter(failureTrigger mockRequest, failureType mockResponse) (func(mockRequest, *protos.PeerID) mockResponse, *filterResult) {
	res := &filterResult{triggered: false, mutex: &sync.Mutex{}}
	return func(request mockRequest, peerID *protos.PeerID) mockResponse {
		//fmt.Println("Received a request", request, "for replicaId", replicaId)
		if request != failureTrigger {
			return Normal
		}

		res.mutex.Lock()
		defer res.mutex.Unlock()

		if !res.triggered {
			res.triggered = true
			res.peerID = peerID
		}

		if *peerID == *res.peerID {
			fmt.Println("Failing it with", failureType)
			return failureType
		}
		return Normal
	}, res

}

func TestCatchupSimple(t *testing.T) {
	mrls := createRemoteLedgers(1, 3)

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(0, SimpleGetBlock(0))

	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()
	if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
		t.Fatalf("Simplest case: %s", err)
	}

}

func TestCatchupSyncBlocksErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 1 with valid genesis block
		// Timeouts of 10 milliseconds
		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(mrls, filter)

		ml.PutBlock(0, SimpleGetBlock(0))
		sts := newTestStateTransfer(ml, mrls)
		defer sts.Stop()
		sts.BlockRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
			t.Fatalf("SyncBlocksErrors %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncBlocksErrors case never simulated a %v", failureType)
		}
	}
}

// Added for issue #676, for situations all potential sync targets fail, and sync is re-initiated, causing panic
func TestCatchupSyncBlocksAllErrors(t *testing.T) {
	blockNumber := uint64(10)

	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 1 with valid genesis block
		// Timeouts of 10 milliseconds
		succeeding := &filterResult{triggered: false, mutex: &sync.Mutex{}}
		filter := func(request mockRequest, peerID *protos.PeerID) mockResponse {

			succeeding.mutex.Lock()
			defer succeeding.mutex.Unlock()
			if !succeeding.triggered {
				return failureType
			}

			return Normal
		}
		ml := NewMockLedger(mrls, filter)

		ml.PutBlock(0, SimpleGetBlock(0))
		sts := newTestStateTransfer(ml, mrls)
		defer sts.Stop()
		sts.BlockRequestTimeout = 10 * time.Millisecond

		for peerID, _ := range mrls.remoteLedgers {
			mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = blockNumber + 1
		}

		failChannel := make(chan struct{})
		errCount := 0

		protoListener := &ProtoListener{
			ErroredImpl: func(uint64, []byte, []*protos.PeerID, interface{}, error) {
				errCount++
				if errCount == 3 {
					failChannel <- struct{}{}
					failChannel <- struct{}{} // Block the state transfer thread until the second read
				}
			},
			CompletedImpl: func(uint64, []byte, []*protos.PeerID, interface{}) {
				if !succeeding.triggered {
					t.Fatalf("State transfer should not have completed yet")
				} else {

				}
			},
		}

		complete := sts.CompletionChannel()

		sts.RegisterListener(protoListener)

		sts.Initiate(nil)

		blockHash := SimpleGetBlockHash(blockNumber)
		sts.AddTarget(blockNumber, blockHash, nil, nil)

		select {
		case <-time.After(time.Second * 2):
			t.Fatalf("Timed out waiting for state to error out")
		case <-failChannel:
		}

		succeeding.triggered = true
		sts.AddTarget(blockNumber, blockHash, nil, nil)

		<-failChannel // Unblock state transfer

		select {
		case <-time.After(time.Second * 2):
			t.Fatalf("Timed out waiting for state to catch up, error in state transfer")
		case <-complete:
			// Do nothing, continue the test
		}

		if size, _ := ml.GetBlockchainSize(); size != blockNumber+1 { // Will never fail
			t.Fatalf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, size)
		}

		block, err := ml.GetBlock(blockNumber)

		if nil != err {
			t.Fatalf("Error retrieving last block in the mock chain.")
		}

		if stateHash, _ := ml.GetCurrentStateHash(); !bytes.Equal(stateHash, block.StateHash) {
			t.Fatalf("Current state does not validate against the latest block.")
		}
	}
}

func TestCatchupMissingEarlyChain(t *testing.T) {
	mrls := createRemoteLedgers(1, 3)

	// Test from blockheight of 5 (with missing blocks 0-3)
	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(4, SimpleGetBlock(4))
	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()
	if err := executeStateTransfer(sts, ml, 7, 10, mrls); nil != err {
		t.Fatalf("MissingEarlyChain case: %s", err)
	}
}

func TestCatchupSyncSnapshotError(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second, also test corrupt snapshot
		filter, result := makeSimpleFilter(SyncSnapshot, failureType)
		ml := NewMockLedger(mrls, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml, mrls)
		defer sts.Stop()
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
		mrls := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second
		filter, result := makeSimpleFilter(SyncDeltas, failureType)
		ml := NewMockLedger(mrls, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml, mrls)
		defer sts.Stop()
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
	mrls := createRemoteLedgers(1, 3)

	for peerID, _ := range mrls.remoteLedgers {
		mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = 8
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()
	sts.Initiate(nil)
	if err := sts.BlockingAddTarget(7, SimpleGetBlockHash(7), nil); nil != err {
		t.Fatalf("SimpleSynchronous state transfer failed : %s", err)
	}
}

func TestCatchupSimpleSynchronousSuccess(t *testing.T) {
	mrls := createRemoteLedgers(1, 3)

	for peerID, _ := range mrls.remoteLedgers {
		mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = 8
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()

	done := make(chan struct{})

	go func() {
		sts.Initiate(nil)
		sts.BlockingUntilSuccessAddTarget(7, SimpleGetBlockHash(7), nil)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("SimpleSynchronousSuccess state transfer timed out")
	}
}

func executeBlockRecovery(ml *MockLedger, millisTimeout int, mrls *MockRemoteHashLedgerDirectory) error {

	sts := newTestThreadlessStateTransfer(ml, mrls)
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

func executeBlockRecoveryWithPanic(ml *MockLedger, millisTimeout int, mrls *MockRemoteHashLedgerDirectory) error {

	sts := newTestThreadlessStateTransfer(ml, mrls)
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
	mrls := createRemoteLedgers(0, 3)

	for peerID, _ := range mrls.remoteLedgers {
		mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = 701
	}

	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	if err := executeBlockRecovery(ml, 10, mrls); nil != err {
		t.Fatalf("TestCatchupLaggingChains short chain failure: %s", err)
	}

	ml = NewMockLedger(mrls, nil)
	ml.PutBlock(200, SimpleGetBlock(200))
	// Use a large timeout here because the mock ledger is slow for large blocks
	if err := executeBlockRecovery(ml, 1000, mrls); nil != err {
		t.Fatalf("TestCatchupLaggingChains long chain failure: %s", err)
	}
}

func TestCatchupLaggingChainsErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		mrls := createRemoteLedgers(0, 3)

		for peerID, _ := range mrls.remoteLedgers {
			mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = 701
		}

		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(mrls, filter)
		ml.PutBlock(7, SimpleGetBlock(7))
		if err := executeBlockRecovery(ml, 10, mrls); nil != err {
			t.Fatalf("TestCatchupLaggingChainsErrors %s short chain with timeout failure: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("TestCatchupLaggingChainsErrors short chain with timeout never simulated a %s", failureType)
		}
	}
}

func TestCatchupCorruptChains(t *testing.T) {
	mrls := createRemoteLedgers(0, 3)

	for peerID, _ := range mrls.remoteLedgers {
		mrls.GetMockRemoteLedgerByPeerID(&peerID).blockHeight = 701
	}

	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	ml.PutBlock(3, SimpleGetBlock(2))
	if err := executeBlockRecovery(ml, 10, mrls); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}

	ml = NewMockLedger(mrls, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	ml.PutBlock(3, SimpleGetBlock(2))
	defer func() {
		//fmt.Println("Executing defer")
		// We expect a panic, this is great
		recover()
	}()
	if err := executeBlockRecoveryWithPanic(ml, 10, mrls); nil != err {
		t.Fatalf("TestCatchupCorruptChains short chain failure: %s", err)
	}
}

type listenerHelper struct {
	resultChannel chan struct{}
}

func (lh *listenerHelper) Initiated() {}
func (lh *listenerHelper) Errored(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}, err error) {
	select {
	case lh.resultChannel <- struct{}{}:
	default:
	}
}
func (lh *listenerHelper) Completed(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}) {
}

func TestRegisterUnregisterListener(t *testing.T) {
	mrls := &MockRemoteHashLedgerDirectory{&HashLedgerDirectory{make(map[protos.PeerID]consensus.ReadOnlyLedger)}}

	ml := NewMockLedger(nil, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()
	sts.DiscoveryThrottleTime = 1 * time.Millisecond

	l1 := &listenerHelper{make(chan struct{})}
	l2 := &listenerHelper{make(chan struct{})}

	sts.RegisterListener(l1)
	sts.RegisterListener(l2)

	// Will cause an immediate error loop of errors on state sync, as there are no peerIDs
	sts.InvalidateState()
	sts.Initiate(nil)

	select {
	case <-l1.resultChannel:
		// Everything is good
	case <-time.After(2 * time.Second):
		t.Fatalf("Should have received a first notifaction but did not, timed out.")
	}

	sts.UnregisterListener(l1)

	for i := 0; i < 20; i++ {
		select {
		case <-l1.resultChannel:
			t.Fatalf("Should not have received a notification after deregistration.")
		case <-l2.resultChannel:
			// Everything is good
		case <-time.After(2 * time.Second):
			t.Fatalf("Should have received continuous notifications, but did not, timed out.")

		}
	}

}

func TestIdle(t *testing.T) {
	mrls := createRemoteLedgers(0, 1)

	ml := NewMockLedger(nil, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, mrls)
	defer sts.Stop()

	idle := make(chan struct{})
	go func() {
		sts.BlockUntilIdle()
		idle <- struct{}{}
	}()

	select {
	case <-idle:
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for state transfer to become idle")
	}

	if !sts.IsIdle() {
		t.Fatalf("State transfer unblocked from idle but did not remain that way")
	}
}
