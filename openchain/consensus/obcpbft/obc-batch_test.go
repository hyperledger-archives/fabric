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
	"fmt"
	"os"
	"testing"

	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetBatch(inst *instance, batchSize int) {
	os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_N", fmt.Sprintf("%d", inst.net.N)) // TODO, a little hacky, but needed for state transfer not to get upset
	defer func() {
		os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_N")
	}()

	config := loadConfig()
	inst.consenter = newObcBatch(uint64(inst.id), config, inst)
	batch := inst.consenter.(*obcBatch)
	batch.batchSize = batchSize
	batch.pbft.replicaCount = len(inst.net.replicas)
	batch.pbft.f = inst.net.f
	inst.deliver = func(msg []byte, senderHandle *pb.PeerID) {
		batch.RecvMsg(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg}, senderHandle)
	}
}

func TestNetworkBatch(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, func(inst *instance) {
		makeTestnetBatch(inst, 2)
	})
	defer net.close()

	broadcaster := net.handles[generateBroadcaster(validatorCount)]
	err := net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	net.processWithoutDrain()

	if len(net.replicas[0].consenter.(*obcBatch).batchStore) != 1 {
		t.Fatalf("%d message expected in primary's batchStore, found %d", 1, len(net.replicas[0].consenter.(*obcBatch).batchStore))
	}

	err = net.replicas[2].consenter.RecvMsg(createOcMsgWithChainTx(2), broadcaster)
	net.process()

	if len(net.replicas[0].consenter.(*obcBatch).batchStore) != 0 {
		t.Fatalf("%d messages expected in primary's batchStore, found %d", 0, len(net.replicas[0].consenter.(*obcBatch).batchStore))
	}

	for i, inst := range net.replicas {
		block, err := inst.GetBlock(1)
		if nil != err {
			t.Fatalf("Replica %d executed requests, expected a new block on the chain, but could not retrieve it : %s", inst.id, err)
		}
		if numTrans := len(block.Transactions); numTrans != net.replicas[i].consenter.(*obcBatch).batchSize {
			t.Fatalf("Replica %d executed %d requests, expected %d",
				inst.id, numTrans, net.replicas[i].consenter.(*obcBatch).batchSize)
		}
	}
}
