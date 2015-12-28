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
	"os"
	"testing"
	"time"
)

func createStateTransferEnvironment(blockNumber, sequenceNumber uint64, ml *mockLedger) error {

	sts := newStateTransferState(
		&pbftCore{
			replicaCount:    4,
			id:              uint64(0),
			h:               uint64(0),
			K:               uint64(2),
			L:               uint64(4),
			f:               int(1),
			checkpointStore: make(map[*Checkpoint]bool),
		},
		readConfig(),
		ml,
	)

	var chkpt *Checkpoint

	ml.forceRemoteStateBlock(blockNumber - 2)

	for i := uint64(1); i <= 3; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber - i,
			BlockHash:      simpleGetBlockHash(blockNumber - i),
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
			BlockHash:      simpleGetBlockHash(blockNumber),
			ReplicaId:      uint64(i),
			BlockNumber:    blockNumber,
		}
		sts.pbft.checkpointStore[chkpt] = true
	}

	go func() {
		for {
			// In ordinary operation, the weak cert would advance, but to simply testing, send it over and over again
			time.Sleep(time.Millisecond * 10)
			sts.WitnessCheckpointWeakCert(chkpt)
		}
	}()

	select {
	case <-time.After(time.Second * 10):
		return fmt.Errorf("Timed out waiting for state to catch up, error in state transfer")
	case <-sts.completeStateSync:
		// Do nothing, continue the test
	}

	if sts.ledger.getBlockchainSize() != blockNumber+1 {
		return fmt.Errorf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, sts.ledger.getBlockchainSize())
	}

	block, err := sts.ledger.getBlock(blockNumber)

	if nil != err {
		return fmt.Errorf("Error retrieving last block in the mock chain.")
	}

	if !bytes.Equal(sts.ledger.getCurrentStateHash(), block.StateHash) {
		return fmt.Errorf("Current state does not validate against the latest block.")
	}

	return nil
}

func TestCatchupSimple(t *testing.T) {

	// Test from blockheight of 1, with valid genesis block
	rand.Seed(0)
	ml := newMockLedger(nil)
	ml.putBlock(0, simpleGetBlock(0))
	if err := createStateTransferEnvironment(7, 10, ml); nil != err {
		t.Fatalf("Simplest case: %s", err)
	}

}

func TestCatchupMissingEarlyChain(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3
	rand.Seed(1)
	ml := newMockLedger(nil)
	ml.putBlock(4, simpleGetBlock(4))
	if err := createStateTransferEnvironment(7, 10, ml); nil != err {
		t.Fatalf("MissingEarlyChain case: %s", err)
	}
}

func TestCatchupStateSyncTimeout(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3
	rand.Seed(1)
	timeouts := make(map[uint64]bool)
	timeouts[2] = true
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_FULLSTATE", "1s")        //
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_SINGLESTATEDELTA", "1s") // TODO Probably better to set this directly rather than rely on config
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_SINGLEBLOCK", "1s")      //
	ml := newMockLedger(timeouts)
	ml.putBlock(4, simpleGetBlock(4))
	if err := createStateTransferEnvironment(7, 10, ml); nil != err {
		t.Fatalf("StateSyncTimeout case: %s", err)
	}
}

func TestCatchupStateDeltaTimeout(t *testing.T) {
	// Test from blockheight of 5 (with missing blocks 0-3
	rand.Seed(4)
	timeouts := make(map[uint64]bool)
	timeouts[1] = true
	timeouts[3] = true
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_FULLSTATE", "1s")        //
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_SINGLESTATEDELTA", "1s") // TODO Probably better to set this directly rather than rely on config
	os.Setenv("OPENCHAIN_OBCPBFT_STATETRANSFER_TIMEOUT_SINGLEBLOCK", "1s")      //
	ml := newMockLedger(timeouts)
	ml.putBlock(4, simpleGetBlock(4))
	if err := createStateTransferEnvironment(7, 10, ml); nil != err {
		t.Fatalf("StateDeltaTimeout case: %s", err)
	}
}

func TestFixChains(t *testing.T) {
	testChain := func(length uint64, description string) {
		ml := newMockLedger(nil)
		ml.putBlock(length, simpleGetBlock(length))
		sts := threadlessNewStateTransferState(
			&pbftCore{
				replicaCount:    4,
				id:              uint64(0),
				h:               uint64(0),
				K:               uint64(2),
				L:               uint64(4),
				f:               int(1),
				checkpointStore: make(map[*Checkpoint]bool),
			},
			readConfig(),
			ml,
		)

		w := make(chan struct{})

		go func() {
			for !sts.verifyAndRecoverBlockchain() {
			}
			w <- struct{}{}
		}()

		select {
		case <-time.After(time.Second * 5):
			t.Fatalf("Timed out waiting for blocks to replicate for %s blockchain", description)
		case <-w:
			// Do nothing, continue the test
		}

		if n, err := ml.verifyBlockChain(7, 0); 0 != n || nil != err {
			t.Fatalf("%s blockchain claims to be up to date, but does not verify", description)
		}
	}

	testChain(7, "Short")

	testChain(700, "Long")
}
