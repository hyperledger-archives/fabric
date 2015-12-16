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

	net.replicas[1].consenter.RecvMsg(createExternalRequest(1))

	go net.processContinually(func(out bool, id int, raw []byte) []byte {
		if out && id == 0 {
			sieve := &SieveMessage{}
			proto.Unmarshal(raw, sieve)
			if sieve.GetPbftMessage() != nil {
				return nil
			}
		}
		return raw
	})
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
