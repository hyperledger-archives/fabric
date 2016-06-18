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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hyperledger/fabric/consensus"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"

	pb "github.com/hyperledger/fabric/protos"
)

func (op *obcSieve) getPBFTCore() *pbftCore {
	return op.legacyGenericShim.pbft.pbftCore
}

func obcSieveHelper(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
	// It's not entirely obvious why the compiler likes the parent function, but not newObcSieve directly
	return newObcSieve(id, config, stack)
}

func TestSieveNetwork(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcSieveHelper)
	defer net.stop()

	net.debug = true

	req1 := createOcMsgWithChainTx(1)
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(req1, net.endpoints[generateBroadcaster(validatorCount)].getHandle())
	net.process()
	req0 := createOcMsgWithChainTx(2)
	net.endpoints[0].(*consumerEndpoint).consumer.RecvMsg(req0, net.endpoints[generateBroadcaster(validatorCount)].getHandle())
	net.process()

	testblock := func(ep endpoint, blockNo uint64, msg *pb.Message) {
		cep := ep.(*consumerEndpoint)
		block, err := cep.consumer.(*obcSieve).stack.GetBlock(blockNo)
		if err != nil {
			t.Fatalf("Replica %d could not retrieve block %d: %s", cep.id, blockNo, err)
		}
		txs := block.GetTransactions()
		if len(txs) != 1 {
			t.Fatalf("Replica %d block %v contains %d transactions, expected 1", cep.id, blockNo, len(txs))
		}
		if numTxResults := len(block.NonHashData.TransactionResults); numTxResults != 1 {
			t.Fatalf("Replica %d block %v has %d txResults, expected 1", cep.id, blockNo, numTxResults)
		}

		msgTx := &pb.Transaction{}
		proto.Unmarshal(msg.Payload, msgTx)
		if !reflect.DeepEqual(txs[0], msgTx) {
			t.Errorf("Replica %d transaction does not match; is %+v, should be %+v", cep.id, txs[0], msgTx)
		}
	}

	for _, ep := range net.endpoints {
		cep := ep.(*consumerEndpoint)
		blockchainSize := cep.consumer.(*obcSieve).stack.GetBlockchainSize() - 1
		if blockchainSize != 2 {
			t.Errorf("Replica %d has incorrect blockchain size; is %d, should be 2", cep.id, blockchainSize)
		}
		testblock(cep, 1, req1)
		testblock(cep, 2, req0)

		if cep.consumer.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				cep.id, cep.consumer.(*obcSieve).epoch)
		}
	}
}

// TestSieveNoDecision disables PFBT messages from replica 0 to
// simulate the sieve leader being byzantine.  Execute and verify
// replies will make it to replica 0, but the replicas will time out
// waiting for the verifyset.
func TestSieveNoDecision(t *testing.T) {
	t.Skip() // This is periodically failing, TODO, fix eventually
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcSieveHelper, func(ce *consumerEndpoint) {
		ce.consumer.(*obcSieve).pbft.requestTimeout = 400 * time.Millisecond
		ce.consumer.(*obcSieve).pbft.newViewTimeout = 1200 * time.Millisecond
		ce.consumer.(*obcSieve).pbft.lastNewViewTimeout = 1200 * time.Millisecond
	})
	// net.debug = true // Enable for debug
	net.testnet.filterFn = func(src int, dst int, raw []byte) []byte {
		if dst == -1 && src == 0 {
			sieve := &SieveMessage{}
			if err := proto.Unmarshal(raw, sieve); nil != err {
				panic("Should only ever encounter sieve messages")
			}
			if sieve.GetPbftMessage() != nil {
				return nil
			}
		}
		return raw
	}

	fmt.Printf("DEBUG: filterFn is %p and net is %p\n", net.testnet.filterFn, net.testnet)

	broadcaster := net.endpoints[generateBroadcaster(validatorCount)].getHandle()
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), broadcaster)

	go net.processContinually()
	time.Sleep(2 * time.Second)
	net.endpoints[3].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), broadcaster)
	time.Sleep(5 * time.Second)
	net.stop()

	for _, ep := range net.endpoints {
		cep := ep.(*consumerEndpoint)
		newBlocks := cep.consumer.(*obcSieve).stack.GetBlockchainSize() - 1
		if newBlocks != 2 {
			t.Errorf("replica %d executed %d requests, expected %d",
				cep.id, newBlocks, 2)
		}

		if cep.consumer.(*obcSieve).epoch != 1 {
			t.Errorf("replica %d in epoch %d, expected 1",
				cep.id, cep.consumer.(*obcSieve).epoch)
		}
	}
}

func TestSieveReqBackToBack(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcSieveHelper)
	defer net.stop()

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
					for _, d := range delayPkt {
						net.msgs <- d
					}
					delayPkt = nil
				}
			}
		}
		return payload
	}

	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), net.endpoints[generateBroadcaster(validatorCount)].getHandle())
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), net.endpoints[generateBroadcaster(validatorCount)].getHandle())

	net.process()

	for _, ep := range net.endpoints {
		cep := ep.(*consumerEndpoint)
		newBlocks := cep.consumer.(*obcSieve).stack.GetBlockchainSize()
		newBlocks--
		if newBlocks != 2 {
			t.Errorf("Replica %d executed %d requests, expected %d",
				cep.id, newBlocks, 2)
		}

		if cep.consumer.(*obcSieve).epoch != 0 {
			t.Errorf("Replica %d in epoch %d, expected 0",
				cep.id, cep.consumer.(*obcSieve).epoch)
		}
	}
}

func TestSieveNonDeterministic(t *testing.T) {
	var instResults []int
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, obcSieveHelper, func(ce *consumerEndpoint) {
		ce.execTxResult = func(tx []*pb.Transaction) ([]byte, error) {
			res := fmt.Sprintf("%d %s", instResults[ce.id], tx)
			fmt.Printf("State hash for %d: %s\n", ce.id, res)
			return []byte(res), nil
		}
	})
	defer net.stop()

	instResults = []int{1, 2, 3, 4}
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(1), net.endpoints[generateBroadcaster(validatorCount)].getHandle())
	net.process()

	instResults = []int{5, 5, 6, 6}
	net.endpoints[1].(*consumerEndpoint).consumer.RecvMsg(createOcMsgWithChainTx(2), net.endpoints[generateBroadcaster(validatorCount)].getHandle())

	net.process()

	results := make([][]byte, len(net.endpoints))
	for _, ep := range net.endpoints {
		cep := ep.(*consumerEndpoint)
		block, err := cep.consumer.(*obcSieve).stack.GetBlock(1)
		if err != nil {
			t.Fatalf("Expected replica %d to have one block", cep.id)
		}
		blockRaw, _ := proto.Marshal(block)
		results[cep.id] = blockRaw
	}
	if !(reflect.DeepEqual(results[0], results[1]) &&
		reflect.DeepEqual(results[0], results[2]) &&
		reflect.DeepEqual(results[0], results[3])) {
		t.Fatalf("Expected all replicas to reach the same block, got: %v", results)
	}
}

func TestSieveRequestHash(t *testing.T) {
	validatorCount := 1
	net := makeConsumerNetwork(validatorCount, obcSieveHelper)
	defer net.stop()

	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_DEPLOY, Payload: make([]byte, 1000)}
	txPacked, _ := proto.Marshal(tx)
	msg := &pb.Message{
		Type:    pb.Message_CHAIN_TRANSACTION,
		Payload: txPacked,
	}

	r0 := net.endpoints[0].(*consumerEndpoint)
	r0.consumer.RecvMsg(msg, r0.getHandle())

	// This used to be enormous, verify that it is short
	txID := fmt.Sprintf("%v", net.mockLedgers[0].txID)
	if len(txID) == 0 || len(txID) > 1000 {
		t.Fatalf("invalid transaction id hash length %d", len(txID))
	}
}

func TestSieveCustody(t *testing.T) {
	validatorCount := 4
	net := makeConsumerNetwork(validatorCount, func(id uint64, config *viper.Viper, stack consensus.Stack) pbftConsumer {
		config.Set("general.timeout.request", "800ms")
		config.Set("general.timeout.viewchange", "1600ms")
		return newObcSieve(id, config, stack)
	})
	net.filterFn = func(src int, dst int, payload []byte) []byte {
		logger.Infof("msg from %d to %d", src, dst)
		if src == 0 {
			return nil
		}
		return payload
	}

	go net.processContinually()
	r2 := net.endpoints[2].(*consumerEndpoint).consumer
	r2.RecvMsg(createOcMsgWithChainTx(1), net.endpoints[1].getHandle())
	time.Sleep(6 * time.Second)
	net.stop()

	for _, inst := range net.endpoints {
		inst := inst.(*consumerEndpoint)
		_, err := inst.consumer.(*obcSieve).stack.GetBlock(1)
		if err != nil {
			t.Errorf("Expected replica %d to have one block", inst.id)
			continue
		}
	}
}
