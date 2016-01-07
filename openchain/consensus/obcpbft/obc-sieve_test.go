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
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetSieve(inst *instance) {
	config := readConfig()
	inst.consenter = newObcSieve(uint64(inst.id), config, inst)
	sieve := inst.consenter.(*obcSieve)
	sieve.pbft.replicaCount = len(inst.net.replicas)
	sieve.pbft.f = inst.net.f
	inst.deliver = func(msg []byte) {
		sieve.RecvMsg(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg})
	}
}

func TestSieveNetwork(t *testing.T) {
	net := makeTestnet(1, makeTestnetSieve)
	defer net.close()

	err := net.replicas[1].consenter.RecvMsg(createExternalRequest(1))
	if err != nil {
		t.Fatalf("External request was not processed by backup: %v", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.blocks) != 1 {
			t.Errorf("Replica %d executed %d requests, expected %d",
				inst.id, len(inst.blocks), 1)
		}

		if inst.consenter.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveNoDecision(t *testing.T) {
	net := makeTestnet(1, func(i *instance) {
		makeTestnetSieve(i)
		i.consenter.(*obcSieve).pbft.requestTimeout = 100 * time.Millisecond
		i.consenter.(*obcSieve).pbft.newViewTimeout = 100 * time.Millisecond
		i.consenter.(*obcSieve).pbft.lastNewViewTimeout = 100 * time.Millisecond
	})
	defer net.close()
	net.filterFn = func(src int, dst int, raw []byte) []byte {
		if dst == -1 && src == 0 {
			sieve := &SieveMessage{}
			proto.Unmarshal(raw, sieve)
			if sieve.GetPbftMessage() != nil {
				return nil
			}
		}
		return raw
	}

	net.replicas[1].consenter.RecvMsg(createExternalRequest(1))

	go net.processContinually()
	time.Sleep(1 * time.Second)
	net.replicas[3].consenter.RecvMsg(createExternalRequest(1))
	time.Sleep(1 * time.Second)
	net.close()

	for _, inst := range net.replicas {
		if len(inst.blocks) != 1 {
			t.Errorf("replica %d executed %d requests, expected %d",
				inst.id, len(inst.blocks), 1)
		}

		if inst.consenter.(*obcSieve).epoch != 1 {
			t.Errorf("replica %d in epoch %d, expected 1",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveReqBackToBack(t *testing.T) {
	net := makeTestnet(1, makeTestnetSieve)
	defer net.close()

	var delayPkt []taggedMsg
	gotExec := 0
	net.filterFn = func(src int, dst int, payload []byte) []byte {
		if dst == 3 {
			sieve := &SieveMessage{}
			proto.Unmarshal(payload, sieve)
			if gotExec < 2 && sieve.GetPbftMessage() != nil {
				delayPkt = append(delayPkt, taggedMsg{src, dst, payload})
				return nil
			}
			if sieve.GetExecute() != nil {
				gotExec++
				if gotExec == 2 {
					net.msgs = append(net.msgs, delayPkt...)
					delayPkt = nil
				}
			}
		}
		return payload
	}

	net.replicas[1].consenter.RecvMsg(createExternalRequest(1))
	net.replicas[1].consenter.RecvMsg(createExternalRequest(2))

	net.process()

	for _, inst := range net.replicas {
		if len(inst.blocks) != 2 {
			t.Errorf("Replica %d executed %d requests, expected %d",
				inst.id, len(inst.blocks), 2)
		}

		if inst.consenter.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveNonDeterministic(t *testing.T) {
	var instResults []int

	net := makeTestnet(1, func(inst *instance) {
		makeTestnetSieve(inst)
		inst.execTxResult = func(tx []*pb.Transaction) ([]byte, []error) {
			res := fmt.Sprintf("%d %s", instResults[inst.id], tx)
			logger.Debug("State hash for %d: %s", inst.id, res)
			return []byte(res), nil
		}
	})
	defer net.close()

	instResults = []int{1, 2, 3, 4}
	net.replicas[1].consenter.RecvMsg(createExternalRequest(1))
	net.process()

	instResults = []int{5, 5, 6, 6}
	net.replicas[1].consenter.RecvMsg(createExternalRequest(2))
	net.process()

	results := make([]int, len(net.replicas))
	for _, inst := range net.replicas {
		results[inst.id] = len(inst.blocks)
	}
	if !reflect.DeepEqual(results, []int{0, 0, 1, 1}) && !reflect.DeepEqual(results, []int{1, 1, 0, 0}) {
		t.Fatalf("Expected two replicas to execute one request, got: %v", results)
	}
}

func TestSieveRequestHash(t *testing.T) {
	net := makeTestnet(1, makeTestnetSieve)
	defer net.close()

	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Payload: make([]byte, 1000)}
	txPacked, _ := proto.Marshal(tx)
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
		Payload: txPacked,
	}

	r0 := net.replicas[0]
	r0.consenter.RecvMsg(msg)

	txId := r0.txID.(string)
	if len(txId) == 0 || len(txId) > 1000 {
		t.Fatalf("invalid transaction id hash length %d", len(txId))
	}
}
