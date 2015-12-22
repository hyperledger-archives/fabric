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
	"testing"
	"time"
)

func TestSimpleCatchup(t *testing.T) {
	ml := newMockLedger()
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
		ml,
	)

	blockNumber := uint64(7)
	sequenceNumber := uint64(10)
	var chkpt *Checkpoint

	ml.forceRemoteStateBlock(blockNumber - 2)

	for i := uint64(1); i <= 4; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber - i,
			BlockHash:      simpleGetBlockHash(blockNumber - i),
			ReplicaId:      i,
			BlockNumber:    blockNumber - i,
		}
		sts.WitnessCheckpoint(chkpt)
	}

	if !sts.OutOfDate {
		t.Fatalf("Replica did not detect itself falling behind to initiate the state transfer")
	}

	for i := uint64(1); i <= 4; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber,
			BlockHash:      simpleGetBlockHash(blockNumber),
			ReplicaId:      i,
			BlockNumber:    blockNumber,
		}
		sts.pbft.checkpointStore[chkpt] = true
	}

	// This sleep is okay, because in normal operation, if we missed the weakcert, there would be another with a later sequence number shortly which we could sync to
	time.Sleep(time.Second * 1)

	sts.WitnessCheckpointWeakCert(chkpt)

	select {
	case <-time.After(time.Second * 5):
		t.Fatalf("Timed out waiting for state to catch up, error in state transfer")
	case <-sts.completeStateSync:
		// Do nothing, continue the test
	}

	if sts.ledger.getBlockchainSize() != blockNumber+1 {
		t.Fatalf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, sts.ledger.getBlockchainSize())
	}

	block, err := sts.ledger.getBlock(blockNumber)

	if nil != err {
		t.Fatalf("Error retrieving last block in the mock chain.")
	}

	if !bytes.Equal(sts.ledger.getCurrentStateHash(), block.StateHash) {
		t.Fatalf("Current state does not validate against the latest block")
	}

}
