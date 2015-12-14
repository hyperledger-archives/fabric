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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetBatch(f int, batchSize int) *testnet {
	replicaCount := 3*f + 1
	net := &testnet{}
	net.cond = sync.NewCond(&sync.Mutex{})
	config := readConfig()
	for i := 0; i < replicaCount; i++ {
		inst := &instance{id: i, addr: strconv.Itoa(i), net: net} // XXX ugly hack
		inst.batch = newObcBatch(uint64(i), config, inst)
		inst.batch.batchSize = batchSize
		inst.batch.pbft.replicaCount = replicaCount
		inst.batch.pbft.f = f
		net.replicas = append(net.replicas, inst)
		net.addresses = append(net.addresses, inst.addr)
	}

	return net
}

func (net *testnet) closeBatch() {
	if net.closed {
		return
	}
	for _, inst := range net.replicas {
		inst.batch.pbft.close()
		inst.batch.close()
	}
	net.cond.L.Lock()
	net.closed = true
	net.cond.Signal()
	net.cond.L.Unlock()
}

func (net *testnet) processBatch(filterFns ...func(bool, int, []byte) []byte) error {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()

	for len(net.msgs) > 0 {
		msgs := net.msgs
		net.msgs = nil

		for _, taggedMsg := range msgs {
			for _, msg := range net.filterMsg(taggedMsg, filterFns...) {
				net.cond.L.Unlock()
				net.replicas[msg.id].batch.RecvMsg(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg.msg})
				net.cond.L.Lock()
			}
		}
	}

	return nil
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
	net := makeTestnetBatch(1, 2)
	defer net.closeBatch()
	err := net.replicas[1].batch.RecvMsg(createExternalRequest(1))
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}
	if len(net.msgs) != 1 {
		t.Fatalf("%d message was expected to be broadcasted, got %d instead", 1, len(net.msgs))
	}

	time.Sleep(100 * time.Millisecond)

	err = net.processBatch()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if len(net.replicas[0].batch.batchStore) != 1 {
		t.Fatalf("%d message expected in primary's batchStore, found %d", 1, len(net.replicas[0].batch.batchStore))
	}

	err = net.replicas[2].batch.RecvMsg(createExternalRequest(2))
	err = net.processBatch()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if len(net.replicas[0].batch.batchStore) != 0 {
		t.Fatalf("%d messages expected in primary's batchStore, found %d", 0, len(net.replicas[0].batch.batchStore))
	}

	if len(net.replicas[3].executed) != 2 {
		t.Fatalf("%d requests executed, expected %d", len(net.replicas[3].executed), net.replicas[3].batch.batchSize)
	}

}
