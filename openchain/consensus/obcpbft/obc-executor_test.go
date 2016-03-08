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
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/statetransfer"
	pb "github.com/openblockchain/obc-peer/protos"
)

func makePartialStack(mrls map[pb.PeerID]*MockRemoteLedger) statetransfer.PartialStack {
	rols := make(map[pb.PeerID]consensus.ReadOnlyLedger)
	ml := NewMockLedger(&rols, nil)

	peerEndpoints := make([]*pb.PeerEndpoint, 4)
	peerIDs := make([]*pb.PeerID, 4)

	for i := uint64(0); i <= 3; i++ {
		peerID, _ := getValidatorHandle(i)
		if 0 != i {
			l := &MockRemoteLedger{}
			rols[*peerID] = l
			mrls[*peerID] = l
		}

		peerIDs[i] = peerID

		peerEndpoints[i] = &pb.PeerEndpoint{
			ID:   peerID,
			Type: pb.PeerEndpoint_VALIDATOR,
		}

	}

	return &omniProto{
		// LedgerStack
		BeginTxBatchImpl:           ml.BeginTxBatch,
		ExecTxsImpl:                ml.ExecTxs,
		CommitTxBatchImpl:          ml.CommitTxBatch,
		RollbackTxBatchImpl:        ml.RollbackTxBatch,
		PreviewCommitTxBatchImpl:   ml.PreviewCommitTxBatch,
		GetRemoteBlocksImpl:        ml.GetRemoteBlocks,
		GetRemoteStateSnapshotImpl: ml.GetRemoteStateSnapshot,
		GetRemoteStateDeltasImpl:   ml.GetRemoteStateDeltas,
		PutBlockImpl:               ml.PutBlock,
		ApplyStateDeltaImpl:        ml.ApplyStateDelta,
		CommitStateDeltaImpl:       ml.CommitStateDelta,
		RollbackStateDeltaImpl:     ml.RollbackStateDelta,
		EmptyStateImpl:             ml.EmptyState,
		HashBlockImpl:              ml.HashBlock,
		VerifyBlockchainImpl:       ml.VerifyBlockchain,
		GetBlockImpl:               ml.GetBlock,
		GetCurrentStateHashImpl:    ml.GetCurrentStateHash,
		GetBlockchainSizeImpl:      ml.GetBlockchainSize,
		// Inquirer
		GetNetworkInfoImpl: func() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
			return peerEndpoints[0], peerEndpoints, nil
		},
		GetNetworkHandlesImpl: func() (self *pb.PeerID, network []*pb.PeerID, err error) {
			return peerIDs[0], peerIDs, nil
		},
	}

}

func TestExecutorIdle(t *testing.T) {
	obcex := NewOBCExecutor(0, loadConfig(), 30, nil, nil)
	done := make(chan struct{})
	go func() {
		obcex.BlockUntilIdle()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Executor did not become idle within 1 second")
	}

}

func TestExecutorSimpleStateTransfer(t *testing.T) {
	mrls := make(map[pb.PeerID]*MockRemoteLedger)
	ps := makePartialStack(mrls)
	obcex := NewOBCExecutor(0, loadConfig(), 30, &omniProto{}, ps)

	i := uint64(0)
	for _, rl := range mrls {
		rl.blockHeight = 6 + i
		i++
	}

	bi := &BlockInfo{
		BlockNumber: 6,
		BlockHash:   SimpleGetBlockHash(6),
	}

	biAsBytes, _ := proto.Marshal(bi)

	obcex.SkipTo(6, biAsBytes, nil)
	obcex.Execute(7, []*pb.Transaction{&pb.Transaction{}}, &ExecutionInfo{})
	obcex.BlockUntilIdle()

	if obcex.lastExec != 7 {
		t.Fatalf("Expected execution")
	}

}

func TestExecutorDivergentStateTransfer(t *testing.T) {
	mrls := make(map[pb.PeerID]*MockRemoteLedger)
	ps := makePartialStack(mrls)
	obcex := NewOBCExecutor(0, loadConfig(), 30, &omniProto{}, ps)

	i := uint64(0)
	for _, rl := range mrls {
		rl.blockHeight = 6 + i
		i++
	}

	bi := &BlockInfo{
		BlockNumber: 6,
		BlockHash:   SimpleGetBlockHash(6),
	}

	biAsBytes, _ := proto.Marshal(bi)

	obcex.SkipTo(12, biAsBytes, nil)
	obcex.Execute(15, []*pb.Transaction{&pb.Transaction{}}, &ExecutionInfo{})
	obcex.BlockUntilIdle()

	if obcex.lastExec != 15 {
		t.Fatalf("Expected execution")
	}
}
