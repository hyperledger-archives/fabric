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
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetSieve(inst *instance) {
	os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_N", fmt.Sprintf("%d", inst.net.N))       // TODO, a little hacky, but needed for state transfer not to get upset
	os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_F", fmt.Sprintf("%d", (inst.net.N-1)/3)) // TODO, a little hacky, but needed for state transfer not to get upset
	defer func() {
		os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_N")
		os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_F")
	}()

	config := loadConfig()
	inst.consenter = newObcSieve(uint64(inst.id), config, inst)
	sieve := inst.consenter.(*obcSieve)
	sieve.pbft.replicaCount = len(inst.net.replicas)
	sieve.pbft.f = inst.net.f
	inst.deliver = func(msg []byte, senderHandle *pb.PeerID) {
		sieve.RecvMsg(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg}, senderHandle)
	}
}

func TestSieveNetwork(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetSieve)
	defer net.close()

	req1 := createOcMsgWithChainTx(1)
	net.replicas[1].consenter.RecvMsg(req1, net.handles[generateBroadcaster(validatorCount)])
	net.process()
	req0 := createOcMsgWithChainTx(2)
	net.replicas[0].consenter.RecvMsg(req0, net.handles[generateBroadcaster(validatorCount)])
	net.process()

	testblock := func(inst *instance, blockNo uint64, msg *pb.OpenchainMessage) {
		block, err := inst.GetBlock(blockNo)
		if err != nil {
			t.Fatalf("Replica %d could not retrieve block %d: %s", inst.id, blockNo, err)
		}
		txs := block.GetTransactions()
		if len(txs) != 1 {
			t.Fatalf("Replica %d block %v contains %d transactions, expected 1", inst.id, blockNo, len(txs))
		}

		msgTx := &pb.Transaction{}
		proto.Unmarshal(msg.Payload, msgTx)
		if !reflect.DeepEqual(txs[0], msgTx) {
			t.Errorf("Replica %d transaction does not match; is %+v, should be %+v", inst.id, txs[0], msgTx)
		}
	}

	for _, inst := range net.replicas {
		blockchainSize, _ := inst.GetBlockchainSize()
		blockchainSize--
		if blockchainSize != 2 {
			t.Errorf("Replica %d has incorrect blockchain size; is %d, should be 2", inst.id, blockchainSize)
		}
		testblock(inst, 1, req1)
		testblock(inst, 2, req0)

		if inst.consenter.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveNoDecision(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, func(i *instance) {
		makeTestnetSieve(i)
		i.consenter.(*obcSieve).pbft.requestTimeout = 200 * time.Millisecond
		i.consenter.(*obcSieve).pbft.newViewTimeout = 400 * time.Millisecond
		i.consenter.(*obcSieve).pbft.lastNewViewTimeout = 400 * time.Millisecond
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

	broadcaster := net.handles[generateBroadcaster(validatorCount)]
	net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(1), broadcaster)

	go net.processContinually()
	time.Sleep(1 * time.Second)
	net.replicas[3].consenter.RecvMsg(createOcMsgWithChainTx(1), broadcaster)
	time.Sleep(3 * time.Second)
	net.close()

	for _, inst := range net.replicas {
		newBlocks, _ := inst.GetBlockchainSize() // Doesn't fail
		newBlocks--
		if newBlocks != 1 {
			t.Errorf("replica %d executed %d requests, expected %d",
				inst.id, newBlocks, 1)
		}

		if inst.consenter.(*obcSieve).epoch != 1 {
			t.Errorf("replica %d in epoch %d, expected 1",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveReqBackToBack(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetSieve)
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

	net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(1), net.handles[generateBroadcaster(validatorCount)])
	net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(2), net.handles[generateBroadcaster(validatorCount)])

	net.process()

	for _, inst := range net.replicas {
		newBlocks, _ := inst.GetBlockchainSize() // Doesn't fail
		newBlocks--
		if newBlocks != 2 {
			t.Errorf("Replica %d executed %d requests, expected %d",
				inst.id, newBlocks, 2)
		}

		if inst.consenter.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				inst.id, inst.consenter.(*obcSieve).epoch)
		}
	}
}

func TestSieveNonDeterministic(t *testing.T) {
	var instResults []int
	validatorCount := 4
	net := makeTestnet(validatorCount, func(inst *instance) {
		makeTestnetSieve(inst)
		inst.execTxResult = func(tx []*pb.Transaction) ([]byte, error) {
			res := fmt.Sprintf("%d %s", instResults[inst.id], tx)
			logger.Debug("State hash for %d: %s", inst.id, res)
			return []byte(res), nil
		}
	})
	defer net.close()

	instResults = []int{1, 2, 3, 4}
	net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(1), net.handles[generateBroadcaster(validatorCount)])
	net.process()

	instResults = []int{5, 5, 6, 6}
	net.replicas[1].consenter.RecvMsg(createOcMsgWithChainTx(2), net.handles[generateBroadcaster(validatorCount)])
	net.process()

	results := make([][]byte, len(net.replicas))
	for _, inst := range net.replicas {
		block, err := inst.GetBlock(1)
		if err != nil {
			t.Fatalf("Expected replica %d to have one block", inst.id)
		}
		blockRaw, _ := proto.Marshal(block)
		results[inst.id] = blockRaw
	}
	if !(reflect.DeepEqual(results[0], results[1]) &&
		reflect.DeepEqual(results[0], results[2]) &&
		reflect.DeepEqual(results[0], results[3])) {
		t.Fatalf("Expected all replicas to reach the same block, got: %v", results)
	}
}

func TestSieveRequestHash(t *testing.T) {
	validatorCount := 1
	net := makeTestnet(validatorCount, makeTestnetSieve)
	defer net.close()

	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Payload: make([]byte, 1000)}
	txPacked, _ := proto.Marshal(tx)
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
		Payload: txPacked,
	}

	r0 := net.replicas[0]
	r0.consenter.RecvMsg(msg, r0.handle)

	txID := r0.ledger.(*MockLedger).txID.(string)
	if len(txID) == 0 || len(txID) > 1000 {
		t.Fatalf("invalid transaction id hash length %d", len(txID))
	}
}
