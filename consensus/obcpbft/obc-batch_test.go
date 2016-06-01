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
	"reflect"
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

func TestBatchCustody(t *testing.T) {
	t.Skip("test is racy")
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, func(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
		config.Set("general.batchsize", "1")
		config.Set("general.timeout.batch", "250ms")
		if id == 0 {
			// Keep replica 0 from unnecessarilly advancing its view
			config.Set("general.timeout.request", "1500ms")
		} else {
			config.Set("general.timeout.request", "1000ms")
		}
		config.Set("general.timeout.viewchange", "800ms")
		return newObcBatch(id, config, stack)
	})
	defer net.stop()
	net.filterFn = func(src int, dst int, payload []byte) []byte {
		logger.Info("msg from %d to %d", src, dst)
		if src == 0 {
			return nil
		}
		return payload
	}

	// Submit two requests to replica 2, because vp0 is byzantine, they will not be processed until complaints triggers a view change
	// Once the complaints work, we should end up in view 1, with 2 blocks
	r2 := net.endpoints[2].(*consumerEndpoint).consumer
	r2.RecvMsg(createOcMsgWithChainTx(1), net.endpoints[1].getHandle())
	r2.RecvMsg(createOcMsgWithChainTx(2), net.endpoints[1].getHandle())

	//net.debug = true
	net.debugMsg("Stage 1\n")
	// Get the requests into the custody store, will return once vp2 complaints
	net.process()
	net.debugMsg("Stage 2\n")

	// Let the complaint timer expire for vp1/vp3
	time.Sleep(500 * time.Millisecond)

	// Process the new view and execute the requests
	net.process()
	net.debugMsg("Stage 3\n")

	// Let the complaint timer expire for the other request
	time.Sleep(500 * time.Millisecond)

	// Process the complaint, this time without view change
	net.process()
	net.debugMsg("Stage 4\n")

	for i, ep := range net.endpoints {

		b := ep.(*consumerEndpoint).consumer.(*obcBatch)

		if _, err := b.stack.GetBlock(2); nil != err {
			t.Errorf("Expected replica %d to have two blocks", i)
		} else {
			expectedView := uint64(1)
			if b.pbft.view != expectedView {
				t.Errorf("Expected replica %d to have two blocks and be in view %d", b.pbft.id, expectedView)
			}
		}
	}

}

func TestBatchStaleCustody(t *testing.T) {
	config := loadConfig()
	config.Set("general.batchsize", "1")
	config.Set("general.timeout.batch", "250ms")
	config.Set("general.timeout.request", "250ms")
	config.Set("general.timeout.viewchange", "800ms")

	var reqs []*Request
	stack := &omniProto{
		UnicastImpl: func(msg *pb.Message, p *pb.PeerID) error {
			m := &Message{}
			proto.Unmarshal(msg.Payload, m)
			if r := m.GetRequest(); r != nil {
				reqs = append(reqs, r)
			}
			return nil
		},
		BroadcastImpl: func(msg *pb.Message, pt pb.PeerEndpoint_Type) error {
			// we need this mock because occasionally the
			// custody timer for req3 goes off, and a
			// complaint is sent.
			return nil
		},
		BeginTxBatchImpl: func(id interface{}) error {
			return nil
		},
		ExecTxsImpl: func(id interface{}, txs []*pb.Transaction) ([]byte, error) {
			return nil, nil
		},
		CommitTxBatchImpl: func(id interface{}, meta []byte) (*pb.Block, error) {
			return nil, nil
		},
	}
	op := newObcBatch(1, config, stack)
	defer op.Close()

	req1 := createOcMsgWithChainTx(1)
	op.RecvMsg(req1, &pb.PeerID{})
	op.RecvMsg(createOcMsgWithChainTx(2), &pb.PeerID{})
	op.pbft.manager.queue() <- nil
	op.pbft.currentExec = new(uint64) // so that pbft.execDone doesn't get unhappy
	*op.pbft.currentExec = 1
	rblock2raw, _ := proto.Marshal(&RequestBlock{[]*Request{reqs[1]}})
	op.executeImpl(1, rblock2raw)
	time.Sleep(500 * time.Millisecond)
	op.pbft.manager.queue() <- nil
	if len(reqs) != 3 || !reflect.DeepEqual(reqs[2].Payload, req1.Payload) {
		t.Error("expected resubmitted request")
	}
}
