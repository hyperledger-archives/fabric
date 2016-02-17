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

	"github.com/openblockchain/obc-peer/openchain/consensus"
	. "github.com/openblockchain/obc-peer/openchain/consensus/statetransfer" // Bad form, but here until we can figure out how to share tests across packages
	"github.com/openblockchain/obc-peer/protos"
)

func newTestStateTransfer(ml *MockLedger, defaultPeerIDs []*protos.PeerID) *StateTransferState {
	// The implementation of the state transfer depends on randomness
	// Seed it here so that independent test executions are consistent
	rand.Seed(0)

	peerID := &protos.PeerID{
		Name: "State Transfer Test",
	}

	return NewStateTransferState(peerID, loadConfig(), ml, defaultPeerIDs)
}

func createRemoteLedgers(low, high uint64) (*map[protos.PeerID]consensus.ReadOnlyLedger, *map[protos.PeerID]*MockRemoteLedger, []*protos.PeerID) {
	rols := make(map[protos.PeerID]consensus.ReadOnlyLedger)
	mrls := make(map[protos.PeerID]*MockRemoteLedger)
	defaultPeerIDs := make([]*protos.PeerID, 0)

	for i := low; i <= high; i++ {
		peerID := &protos.PeerID{
			Name: fmt.Sprintf("Peer %d", i),
		}
		l := &MockRemoteLedger{}
		rols[*peerID] = l
		mrls[*peerID] = l
		defaultPeerIDs = append(defaultPeerIDs, peerID)
	}
	return &rols, &mrls, defaultPeerIDs
}

func executeStateTransfer(sts *StateTransferState, ml *MockLedger, blockNumber, sequenceNumber uint64, mrls *map[protos.PeerID]*MockRemoteLedger, peerIDs []*protos.PeerID) error {

	for _, remoteLedger := range *mrls {
		remoteLedger.blockHeight = blockNumber + 1
	}

	sts.Initiate(peerIDs)
	result := sts.CompletionChannel()

	blockHash := SimpleGetBlockHash(blockNumber)
	sts.AddTarget(blockNumber, blockHash, peerIDs, nil)

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
	rols, mrls, dps := createRemoteLedgers(1, 3)

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))

	sts := newTestStateTransfer(ml, dps)
	defer sts.Stop()
	if err := executeStateTransfer(sts, ml, 7, 10, mrls, dps); nil != err {
		t.Fatalf("Simplest case: %s", err)
	}

}

func TestCatchupSyncBlocksErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls, dps := createRemoteLedgers(1, 3)

		// Test from blockheight of 1 with valid genesis block
		// Timeouts of 10 milliseconds
		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(rols, filter)

		ml.PutBlock(0, SimpleGetBlock(0))
		sts := newTestStateTransfer(ml, dps)
		defer sts.Stop()
		sts.BlockRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls, dps); nil != err {
			t.Fatalf("SyncBlocksErrors %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncBlocksErrors case never simulated a %v", failureType)
		}
	}
}

func TestCatchupMissingEarlyChain(t *testing.T) {
	rols, mrls, dps := createRemoteLedgers(1, 3)

	// Test from blockheight of 5 (with missing blocks 0-3)
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(4, SimpleGetBlock(4))
	sts := newTestStateTransfer(ml, dps)
	defer sts.Stop()
	if err := executeStateTransfer(sts, ml, 7, 10, mrls, dps); nil != err {
		t.Fatalf("MissingEarlyChain case: %s", err)
	}
}

func TestCatchupSyncSnapshotError(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls, dps := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second, also test corrupt snapshot
		filter, result := makeSimpleFilter(SyncSnapshot, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml, dps)
		defer sts.Stop()
		sts.StateSnapshotRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls, dps); nil != err {
			t.Fatalf("SyncSnapshotError %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncSnapshotError case never simulated a %s", failureType)
		}
	}
}

func TestCatchupSyncDeltasError(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls, dps := createRemoteLedgers(1, 3)

		// Test from blockheight of 5 (with missing blocks 0-3)
		// Timeouts of 1 second
		filter, result := makeSimpleFilter(SyncDeltas, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(4, SimpleGetBlock(4))
		sts := newTestStateTransfer(ml, dps)
		defer sts.Stop()
		sts.StateDeltaRequestTimeout = 10 * time.Millisecond
		sts.StateSnapshotRequestTimeout = 10 * time.Millisecond
		if err := executeStateTransfer(sts, ml, 7, 10, mrls, dps); nil != err {
			t.Fatalf("SyncDeltasError %s case: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("SyncDeltasError case never simulated a %s", failureType)
		}
	}
}

func TestCatchupSimpleSynchronous(t *testing.T) {
	rols, mrls, dps := createRemoteLedgers(1, 3)

	for _, remoteLedger := range *mrls {
		remoteLedger.blockHeight = 8
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, dps)
	defer sts.Stop()
	sts.Initiate(dps)
	if err := sts.BlockingAddTarget(7, SimpleGetBlockHash(7), dps); nil != err {
		t.Fatalf("SimpleSynchronous state transfer failed : %s", err)
	}
}

func TestCatchupSimpleSynchronousSuccess(t *testing.T) {
	rols, mrls, dps := createRemoteLedgers(1, 3)

	for _, remoteLedger := range *mrls {
		remoteLedger.blockHeight = 8
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := newTestStateTransfer(ml, dps)
	defer sts.Stop()

	done := make(chan struct{})

	go func() {
		sts.Initiate(dps)
		sts.BlockingUntilSuccessAddTarget(7, SimpleGetBlockHash(7), dps)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("SimpleSynchronousSuccess state transfer timed out")
	}
}

func executeBlockRecovery(ml *MockLedger, millisTimeout int, defaultPeerIDs []*protos.PeerID) error {

	sts := ThreadlessNewStateTransferState(&protos.PeerID{"Replica 0"}, loadConfig(), ml, defaultPeerIDs)
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

func executeBlockRecoveryWithPanic(ml *MockLedger, millisTimeout int, defaultPeerIDs []*protos.PeerID) error {

	sts := ThreadlessNewStateTransferState(&protos.PeerID{"Replica 0"}, loadConfig(), ml, defaultPeerIDs)
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
	rols, mrls, dps := createRemoteLedgers(0, 3)

	for _, remoteLedger := range *mrls {
		remoteLedger.blockHeight = 701
	}

	ml := NewMockLedger(rols, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	if err := executeBlockRecovery(ml, 10, dps); nil != err {
		t.Fatalf("TestCatchupLaggingChains short chain failure: %s", err)
	}

	ml = NewMockLedger(rols, nil)
	ml.PutBlock(200, SimpleGetBlock(200))
	// Use a large timeout here because the mock ledger is slow for large blocks
	if err := executeBlockRecovery(ml, 1000, dps); nil != err {
		t.Fatalf("TestCatchupLaggingChains long chain failure: %s", err)
	}
}

func TestCatchupLaggingChainsErrors(t *testing.T) {
	for _, failureType := range []mockResponse{Timeout, Corrupt} {
		rols, mrls, dps := createRemoteLedgers(0, 3)

		for _, remoteLedger := range *mrls {
			remoteLedger.blockHeight = 701
		}

		filter, result := makeSimpleFilter(SyncBlocks, failureType)
		ml := NewMockLedger(rols, filter)
		ml.PutBlock(7, SimpleGetBlock(7))
		if err := executeBlockRecovery(ml, 10, dps); nil != err {
			t.Fatalf("TestCatchupLaggingChainsErrors %s short chain with timeout failure: %s", failureType, err)
		}
		if !result.wasTriggered() {
			t.Fatalf("TestCatchupLaggingChainsErrors short chain with timeout never simulated a %s", failureType)
		}
	}
}

func TestCatchupCorruptChains(t *testing.T) {
	rols, mrls, dps := createRemoteLedgers(0, 3)

	for _, remoteLedger := range *mrls {
		remoteLedger.blockHeight = 701
	}

	ml := NewMockLedger(rols, nil)
	ml.PutBlock(7, SimpleGetBlock(7))
	ml.PutBlock(3, SimpleGetBlock(2))
	if err := executeBlockRecovery(ml, 10, dps); nil != err {
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
	if err := executeBlockRecoveryWithPanic(ml, 10, dps); nil != err {
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
	ml := NewMockLedger(nil, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	sts := NewStateTransferState(&protos.PeerID{"Replica 0"}, loadConfig(), ml, []*protos.PeerID{&protos.PeerID{"nonsense"}})
	defer sts.Stop()

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
