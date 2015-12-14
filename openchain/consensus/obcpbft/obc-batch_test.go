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
	gp "google/protobuf"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetBatch(inst *instance, batchSize int) {
	config := readConfig()
	inst.consenter = newObcBatch(uint64(inst.id), config, inst)
	batch := inst.consenter.(*obcBatch)
	batch.batchSize = batchSize
	batch.pbft.replicaCount = len(inst.net.replicas)
	batch.pbft.f = inst.net.f
	inst.deliver = func(msg []byte) {
		batch.RecvMsg(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg})
	}
}

// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
func createExternalRequest(iter int64) (msg *pb.OpenchainMessage) {
	txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)
	msg = &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
		Payload: txPacked,
	}
	return
}

func TestNetworkBatch(t *testing.T) {
	net := makeTestnet(1, func(inst *instance) {
		makeTestnetBatch(inst, 2)
	})
	defer net.close()
	err := net.replicas[1].consenter.RecvMsg(createExternalRequest(1))
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}
	if len(net.msgs) != 1 {
		t.Fatalf("%d message was expected to be broadcasted, got %d instead", 1, len(net.msgs))
	}

	time.Sleep(100 * time.Millisecond)

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if len(net.replicas[0].consenter.(*obcBatch).batchStore) != 1 {
		t.Fatalf("%d message expected in primary's batchStore, found %d", 1, len(net.replicas[0].consenter.(*obcBatch).batchStore))
	}

	err = net.replicas[2].consenter.RecvMsg(createExternalRequest(2))
	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if len(net.replicas[0].consenter.(*obcBatch).batchStore) != 0 {
		t.Fatalf("%d messages expected in primary's batchStore, found %d", 0, len(net.replicas[0].consenter.(*obcBatch).batchStore))
	}

	if len(net.replicas[3].curBatch) != 2 {
		t.Fatalf("%d requests executed, expected %d", len(net.replicas[3].curBatch), net.replicas[3].consenter.(*obcBatch).batchSize)
	}

}
