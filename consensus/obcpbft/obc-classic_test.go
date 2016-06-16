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

	"github.com/spf13/viper"
)

func (op *obcClassic) getPBFTCore() *pbftCore {
	return op.legacyGenericShim.pbft.pbftCore
}

func obcClassicHelper(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
	// It's not entirely obvious why the compiler likes the parent function, but not newObcClassic directly
	return newObcClassic(id, config, stack)
}

func TestClassicNetwork(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcClassicHelper)
	defer net.stop()

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)

	net.process()

	for _, ep := range net.endpoints {
		ce := ep.(*consumerEndpoint)
		block, err := ce.consumer.(*obcClassic).stack.GetBlock(1)
		if nil != err {
			t.Errorf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
		}
		numTrans := len(block.Transactions)
		if numTxResults := len(block.NonHashData.TransactionResults); numTxResults != numTrans {
			t.Fatalf("Replica %d has %d txResults, expected %d", ce.id, numTxResults, numTrans)
		}
	}
}

func TestClassicStateTransfer(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcClassicHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcClassic).pbft.K = 2
		ce.consumer.(*obcClassic).pbft.L = 4
	})
	defer net.stop()
	// net.debug = true

	filterMsg := true
	net.filterFn = func(src int, dst int, msg []byte) []byte {
		if filterMsg && dst == 3 { // 3 is byz
			return nil
		}
		return msg
	}

	// Advance the network one seqNo past so that Replica 3 will have to do statetransfer
	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	net.process()

	// Move the seqNo to 9, at seqNo 6, Replica 3 will realize it's behind, transfer to seqNo 8, then execute seqNo 9
	filterMsg = false
	for n := 2; n <= 9; n++ {
		net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(int64(n)), broadcaster)
	}

	net.process()

	for _, ep := range net.endpoints {
		ce := ep.(*consumerEndpoint)
		obc := ce.consumer.(*obcClassic)
		_, err := obc.stack.GetBlock(9)
		if nil != err {
			t.Errorf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
		}
		if !obc.pbft.activeView || obc.pbft.view != 0 {
			t.Errorf("Replica %d not active in view 0, is %v %d", ce.id, obc.pbft.activeView, obc.pbft.view)
		}
	}
}

func TestClassicBackToBackStateTransfer(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcClassicHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcClassic).pbft.K = 2
		ce.consumer.(*obcClassic).pbft.L = 4
		ce.consumer.(*obcClassic).pbft.requestTimeout = time.Hour // We do not want any view changes
	})
	defer net.stop()
	// net.debug = true

	filterMsg := true
	net.filterFn = func(src int, dst int, msg []byte) []byte {
		if filterMsg && dst == 3 { // 3 is byz
			return nil
		}
		return msg
	}

	// Get the group to advance past seqNo 1, leaving Replica 3 behind
	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	net.process()

	// Now start including Replica 3, go to sequence number 10, Replica 3 will trigger state transfer
	// after seeing seqNo 8, then pass another target for seqNo 10 and 12, but transfer to 8, but the network
	// will have already moved on and be past to seqNo 13, outside of Replica 3's watermarks, but
	// Replica 3 will execute through seqNo 12
	filterMsg = false
	for n := 2; n <= 21; n++ {
		net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(int64(n)), broadcaster)
	}

	net.process()

	for _, ep := range net.endpoints {
		ce := ep.(*consumerEndpoint)
		obc := ce.consumer.(*obcClassic)
		_, err := obc.stack.GetBlock(21)
		if nil != err {
			t.Errorf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
		}
		if !obc.pbft.activeView || obc.pbft.view != 0 {
			t.Errorf("Replica %d not active in view 0, is %v %d", ce.id, obc.pbft.activeView, obc.pbft.view)
		}
	}
}
