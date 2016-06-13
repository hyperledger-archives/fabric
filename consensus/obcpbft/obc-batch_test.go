/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"testing"
	"time"

	"github.com/hyperledger/fabric/consensus"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

func (op *obcBatch) getPBFTCore() *pbftCore {
	return op.pbft
}

func obcBatchHelper(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
	// It's not entirely obvious why the compiler likes the parent function, but not newObcBatch directly
	return newObcBatch(id, config, stack)
}

func TestNetworkBatch(t *testing.T) {
	batchSize := 2
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcBatchHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcBatch).batchSize = batchSize
	})
	defer net.stop()

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	err := net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	net.process()

	if l := len(net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore); l != 1 {
		t.Fatalf("%d message expected in primary's batchStore, found %d", 1, l)
	}

	err = net.endpoints[2].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), broadcaster)
	net.process()

	if l := len(net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore); l != 0 {
		t.Fatalf("%d messages expected in primary's batchStore, found %d", 0, l)
	}

	for _, ep := range net.endpoints {
		ce := ep.(*consumerEndpoint)
		block, err := ce.consumer.(*obcBatch).stack.GetBlock(1)
		if nil != err {
			t.Fatalf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
		}
		numTrans := len(block.Transactions)
		if numTrans != batchSize {
			t.Fatalf("Replica %d executed %d requests, expected %d",
				ce.id, numTrans, batchSize)
		}
		if numTxResults := len(block.NonHashData.TransactionResults); numTxResults != 1 /*numTrans*/ {
			t.Fatalf("Replica %d has %d txResults, expected %d", ce.id, numTxResults, numTrans)
		}
	}
}

func TestClearOustandingReqsOnStateRecovery(t *testing.T) {
	b := newObcBatch(0, loadConfig(), &omniProto{})
	defer b.Close()

	b.outstandingReqs[&Request{}] = struct{}{}

	b.manager.Queue() <- stateUpdatedEvent{
		chkpt: &checkpointMessage{
			seqNo: 10,
		},
	}

	b.manager.Queue() <- nil

	if len(b.outstandingReqs) != 0 {
		t.Fatalf("Should not have any requests outstanding after completing state transfer")
	}
}

func TestOutstandingReqsIngestion(t *testing.T) {
	batchSize := 2
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcBatchHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcBatch).batchSize = batchSize
	})
	defer net.stop()

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	err := net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	net.process()

	for i, ep := range net.endpoints {
		count := len(ep.(*consumerEndpoint).consumer.(*obcBatch).outstandingReqs)
		if i == 0 {
			if count != 0 {
				t.Errorf("Batch primary should not have the request in its store")
			}
		} else {
			if count != 1 {
				t.Errorf("Batch backup %d should have the request in its store", i)
			}
		}
	}
}

func TestOutstandingReqsResubmission(t *testing.T) {
	omni := &omniProto{
		ExecuteImpl: func(tag interface{}, txs []*pb.Transaction) {},
	}
	b := newObcBatch(0, loadConfig(), omni)
	defer b.Close()

	omni.BroadcastImpl = func(ocMsg *pb.Message, peerType pb.PeerEndpoint_Type) error {
		batchMsg := &BatchMessage{}
		err := proto.Unmarshal(ocMsg.Payload, batchMsg)

		msgRaw := batchMsg.GetPbftMessage()
		if msgRaw == nil {
			t.Fatalf("Expected PBFT message only")
		}

		msg := &Message{}
		err = proto.Unmarshal(msgRaw, msg)
		if err != nil {
			t.Fatalf("Error unpacking payload from message: %s", err)
		}

		prePrepare := msg.GetPrePrepare()

		if err != nil {
			t.Fatalf("Expected only a prePrepare")
		}

		// Shortcuts the whole 3 phase protocol, and executes whatever is in the prePrepare
		b.execute(prePrepare.SequenceNumber, prePrepare.Request.Payload)

		return nil
	}

	// Add two requests
	b.outstandingReqs[createPbftRequestWithChainTx(1, 0)] = struct{}{}
	b.outstandingReqs[createPbftRequestWithChainTx(2, 0)] = struct{}{}

	seqNo := uint64(1)
	b.pbft.currentExec = &seqNo

	b.manager.Queue() <- committedEvent{}
	b.manager.Queue() <- nil

	if len(b.outstandingReqs) != 0 {
		t.Fatalf("All requests should have been resubmitted")
	}
}

func TestViewChangeOnPrimarySilence(t *testing.T) {
	b := newObcBatch(1, loadConfig(), &omniProto{
		BroadcastImpl: func(ocMsg *pb.Message, peerType pb.PeerEndpoint_Type) error { return nil },
		SignImpl:      func(msg []byte) ([]byte, error) { return msg, nil },
		VerifyImpl:    func(peerID *pb.PeerID, signature []byte, message []byte) error { return nil },
	})
	b.pbft.requestTimeout = 50 * time.Millisecond
	defer b.Close()

	// Send a request, which will be ignored, triggering view change
	b.manager.Queue() <- batchMessageEvent{createOcMsgWithChainTx(1), &pb.PeerID{"vp0"}}
	time.Sleep(time.Second)
	b.manager.Queue() <- nil

	if b.pbft.activeView {
		t.Fatalf("Should have caused a view change")
	}
}
