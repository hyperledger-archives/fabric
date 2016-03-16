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
	//"github.com/openblockchain/obc-peer/openchain/consensus/statetransfer"
	pb "github.com/openblockchain/obc-peer/protos"
)

func newTestExecutor() (*obcExecutor, map[pb.PeerID]consensus.ReadOnlyLedger) {
	mrls := createRemoteLedgers(0, 3)

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(mrls, nil)
	ml.PutBlock(0, SimpleGetBlock(0))

	ps := newPartialStack(ml, mrls)

	return NewOBCExecutor(loadConfig(), &omniProto{
		StartupImpl: func(uint64, []byte) {},
	}, ps), mrls.remoteLedgers
}

func TestExecutorIdle(t *testing.T) {
	obcex, _ := newTestExecutor()
	defer obcex.Stop()
	done := make(chan struct{})
	go func() {
		<-obcex.IdleChan()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Executor did not become idle within 1 second")
	}

}

func TestExecutorSimpleStateTransfer(t *testing.T) {
	obcex, mrls := newTestExecutor()
	defer obcex.Stop()

	i := uint64(0)
	for _, rl := range mrls {
		rl.(*MockRemoteLedger).blockHeight = 6 + i
		i++
	}

	bi := &BlockInfo{
		BlockNumber: 6,
		BlockHash:   SimpleGetBlockHash(6),
	}

	biAsBytes, _ := proto.Marshal(bi)

	obcex.SkipTo(6, biAsBytes, nil)
	obcex.Execute(7, []*pb.Transaction{&pb.Transaction{}}, &ExecutionInfo{})
	<-obcex.IdleChan()

	if obcex.lastExec != 7 {
		t.Fatalf("Expected execution")
	}

}

func TestExecutorDivergentStateTransfer(t *testing.T) {
	obcex, mrls := newTestExecutor()
	defer obcex.Stop()

	i := uint64(0)
	for _, rl := range mrls {
		rl.(*MockRemoteLedger).blockHeight = 6 + i
		i++
	}

	bi := &BlockInfo{
		BlockNumber: 6,
		BlockHash:   SimpleGetBlockHash(6),
	}

	biAsBytes, _ := proto.Marshal(bi)

	obcex.SkipTo(12, biAsBytes, nil)
	obcex.Execute(15, []*pb.Transaction{&pb.Transaction{}}, &ExecutionInfo{})
	<-obcex.IdleChan()

	if obcex.lastExec != 15 {
		t.Fatalf("Expected execution")
	}
}
