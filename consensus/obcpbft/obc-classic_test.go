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

	"github.com/hyperledger/fabric/consensus"

	"github.com/spf13/viper"
)

func (op *obcClassic) getPBFTCore() *pbftCore {
	return op.pbft
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
		_, err := ce.consumer.(*obcClassic).stack.GetBlock(1)
		if nil != err {
			t.Errorf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", ce.id, err)
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
	net.debug = true

	filterMsg := true
	net.filterFn = func(src int, dst int, msg []byte) []byte {
		if filterMsg && dst == 3 { // 3 is byz
			return nil
		}
		return msg
	}

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)

	net.process()
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
