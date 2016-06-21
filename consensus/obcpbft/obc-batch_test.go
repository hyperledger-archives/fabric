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
	"github.com/hyperledger/fabric/consensus/obcpbft/events"
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
		t.Errorf("External request was not processed by backup: %v", err)
	}
	err = net.endpoints[2].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), broadcaster)
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	net.process()
	net.process()

	if l := len(net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore); l != 0 {
		t.Errorf("%d messages expected in primary's batchStore, found %v", 0,
			net.endpoints[0].(*consumerEndpoint).consumer.(*obcBatch).batchStore)
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

	b.reqStore.storeOutstanding(&Request{})

	b.manager.Queue() <- stateUpdatedEvent{
		chkpt: &checkpointMessage{
			seqNo: 10,
		},
	}

	b.manager.Queue() <- nil

	if len(*(b.reqStore.outstandingRequests)) != 0 {
		t.Fatalf("Should not have any requests outstanding after completing state transfer")
	}
}

func TestOutstandingReqsIngestion(t *testing.T) {
	bs := [3]*obcBatch{}
	for i := range bs {
		omni := &omniProto{
			UnicastImpl: func(ocMsg *pb.Message, peer *pb.PeerID) error { return nil },
		}
		bs[i] = newObcBatch(uint64(i), loadConfig(), omni)
		defer bs[i].Close()

		// Have vp1 only deliver messages
		if i == 1 {
			omni.UnicastImpl = func(ocMsg *pb.Message, peer *pb.PeerID) error {
				dest, _ := getValidatorID(peer)
				if dest == 0 || dest == 2 {
					bs[dest].RecvMsg(ocMsg, &pb.PeerID{Name: "vp1"})
				}
				return nil
			}
		}
	}

	err := bs[1].RecvMsg(createOcMsgWithChainTx(1), &pb.PeerID{Name: "vp1"})
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	for _, b := range bs {
		b.manager.Queue() <- nil
		b.broadcaster.Wait()
		b.manager.Queue() <- nil
	}

	for i, b := range bs {
		b.manager.Queue() <- nil
		count := len(*(b.reqStore.outstandingRequests))
		if i == 0 {
			if count != 0 {
				t.Errorf("Batch primary should not have the request in its store: %v", b.reqStore.outstandingRequests)
			}
		} else {
			if count != 1 {
				t.Errorf("Batch backup %d should have the request in its store", i)
			}
		}
	}
}

func TestOutstandingReqsResubmission(t *testing.T) {
	omni := &omniProto{}
	b := newObcBatch(0, loadConfig(), omni)
	defer b.Close() // The broadcasting threads only cause problems here... but this test stalls without them

	transactionsBroadcast := 0
	omni.ExecuteImpl = func(tag interface{}, txs []*pb.Transaction) {
		transactionsBroadcast += len(txs)
		logger.Debugf("\nExecuting %d transactions (%v)\n", len(txs), txs)
		nextExec := b.pbft.lastExec + 1
		b.pbft.currentExec = &nextExec
		b.manager.Inject(executedEvent{tag: tag})
	}

	omni.CommitImpl = func(tag interface{}, meta []byte) {
		b.manager.Inject(committedEvent{})
	}

	omni.UnicastImpl = func(ocMsg *pb.Message, dest *pb.PeerID) error {
		return nil
	}

	reqs := make([]*Request, 8)
	for i := 0; i < len(reqs); i++ {
		reqs[i] = createPbftRequestWithChainTx(int64(i), 0)
	}

	// Add three requests, with a batch size of 2
	b.reqStore.storeOutstanding(reqs[0])
	b.reqStore.storeOutstanding(reqs[1])
	b.reqStore.storeOutstanding(reqs[2])
	b.reqStore.storeOutstanding(reqs[3])

	executed := make(map[string]struct{})
	execute := func() {
		for d, req := range b.pbft.outstandingReqs {
			if _, ok := executed[d]; ok {
				continue
			}
			executed[d] = struct{}{}
			b.execute(b.pbft.lastExec+1, req.Payload)
		}
	}

	events.SendEvent(b, committedEvent{})
	execute()

	if len(*(b.reqStore.outstandingRequests)) != 0 {
		t.Fatalf("All requests should have been executed and deleted after exec")
	}

	// Simulate changing views, with a request in the qSet, and one outstanding which is not
	wreq := reqs[4]

	reqsPacked, err := proto.Marshal(&RequestBlock{[]*Request{wreq}})
	if err != nil {
		t.Fatalf("Unable to pack block for new batch request")
	}

	breq := &Request{Payload: reqsPacked}
	prePrep := &PrePrepare{
		View:           0,
		SequenceNumber: b.pbft.lastExec + 1,
		RequestDigest:  "foo",
		Request:        breq,
	}

	b.pbft.certStore[msgID{v: prePrep.View, n: prePrep.SequenceNumber}] = &msgCert{prePrepare: prePrep}

	// Add the request, which is already pre-prepared, to be outstanding, and one outstanding not pending, not prepared
	b.reqStore.storeOutstanding(wreq) // req 6
	b.reqStore.storeOutstanding(reqs[5])
	b.reqStore.storeOutstanding(reqs[6])
	b.reqStore.storeOutstanding(reqs[7])

	events.SendEvent(b, viewChangedEvent{})
	execute()

	if b.reqStore.hasNonPending() {
		t.Errorf("All requests should have been resubmitted after view change")
	}

	// We should have one request in batch which has not been sent yet
	expected := 6
	if transactionsBroadcast != expected {
		t.Errorf("Expected %d transactions broadcast, got %d", expected, transactionsBroadcast)
	}

	events.SendEvent(b, batchTimerEvent{})
	execute()

	// If the already prepared request were to be resubmitted, we would get count 8 here
	expected = 7
	if transactionsBroadcast != expected {
		t.Errorf("Expected %d transactions broadcast, got %d", expected, transactionsBroadcast)
	}
}

func TestViewChangeOnPrimarySilence(t *testing.T) {
	b := newObcBatch(1, loadConfig(), &omniProto{
		UnicastImpl: func(ocMsg *pb.Message, peer *pb.PeerID) error { return nil },
		SignImpl:    func(msg []byte) ([]byte, error) { return msg, nil },
		VerifyImpl:  func(peerID *pb.PeerID, signature []byte, message []byte) error { return nil },
	})
	b.pbft.requestTimeout = 50 * time.Millisecond
	defer b.Close()

	// Send a request, which will be ignored, triggering view change
	b.manager.Queue() <- batchMessageEvent{createOcMsgWithChainTx(1), &pb.PeerID{Name: "vp0"}}
	time.Sleep(time.Second)
	b.manager.Queue() <- nil

	if b.pbft.activeView {
		t.Fatalf("Should have caused a view change")
	}
}
