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
	"strconv"
	"sync"
)

type mockCPI struct {
	broadcasted [][]byte
	executed    [][]byte
}

func newMock() *mockCPI {
	mock := &mockCPI{
		make([][]byte, 0),
		make([][]byte, 0),
	}
	return mock
}

func (mock *mockCPI) viewChange(uint64) {
	//
}

func (mock *mockCPI) broadcast(msg []byte) {
	mock.broadcasted = append(mock.broadcasted, msg)
}

func (mock *mockCPI) execute(tx []byte) {
	mock.executed = append(mock.executed, tx)
}

// =============================================================================
// Fake network structures
// =============================================================================

type taggedMsg struct {
	id  int
	msg []byte
}

type testnet struct {
	cond      *sync.Cond
	closed    bool
	replicas  []*instance
	msgs      []taggedMsg
	addresses []string
}

type instance struct {
	address  string
	id       int
	pbft     *pbftCore
	net      *testnet
	executed [][]byte
}

func (inst *instance) GetReplicaAddress(self bool) (addresses []string, err error) {
	if self {
		addresses = append(addresses, inst.address)
		return addresses, nil
	}
	return inst.net.addresses, nil
}

func (inst *instance) GetReplicaID(address string) (id uint64, err error) {
	for i, v := range inst.net.addresses {
		if v == address {
			return uint64(i), nil
		}
	}
	err = fmt.Errorf("Couldn't find address in list of addresses in testnet")
	return uint64(0), err
}

func (inst *instance) broadcast(payload []byte) {
	net := inst.net
	net.cond.L.Lock()
	net.msgs = append(net.msgs, taggedMsg{inst.id, payload})
	net.cond.Signal()
	net.cond.L.Unlock()
}

func (*instance) Unicast(msgPayload []byte, receiver string) error {
	panic("not implemented yet")
}

func (inst *instance) execute(payload []byte) {
	inst.executed = append(inst.executed, payload)
}

func (inst *instance) viewChange(uint64) {
	//
}

func (net *testnet) filterMsg(outMsg taggedMsg, filterfns ...func(bool, int, []byte) []byte) (msgs []taggedMsg) {
	msg := outMsg.msg
	for _, f := range filterfns {
		msg = f(true, outMsg.id, msg)
		if msg == nil {
			break
		}
	}

	for i := range net.replicas {
		if i == outMsg.id {
			continue
		}

		msg := msg
		for _, f := range filterfns {
			msg = f(false, i, msg)
			if msg == nil {
				break
			}
		}

		if msg == nil {
			continue
		}

		msgs = append(msgs, taggedMsg{i, msg})
	}

	return msgs
}

func (net *testnet) process(filterfns ...func(bool, int, []byte) []byte) error {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()

	for len(net.msgs) > 0 {
		msgs := net.msgs
		net.msgs = nil

		for _, taggedMsg := range msgs {
			for _, msg := range net.filterMsg(taggedMsg, filterfns...) {
				net.cond.L.Unlock()
				net.replicas[msg.id].pbft.receive(msg.msg)
				net.cond.L.Lock()
			}
		}
	}

	return nil
}

func (net *testnet) processContinually(filterfns ...func(bool, int, []byte) []byte) {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()
	for {
		if net.closed {
			break
		}
		if len(net.msgs) == 0 {
			net.cond.Wait()
		}
		for len(net.msgs) > 0 {
			msgs := net.msgs
			net.msgs = nil

			for _, taggedMsg := range msgs {
				for _, msg := range net.filterMsg(taggedMsg, filterfns...) {
					net.cond.L.Unlock()
					net.replicas[msg.id].pbft.receive(msg.msg)
					net.cond.L.Lock()
				}
			}
		}
	}
}

func makeTestnet(f int, initFn ...func(*pbftCore)) *testnet {
	replicaCount := 3*f + 1
	net := &testnet{}
	net.cond = sync.NewCond(&sync.Mutex{})
	for i := 0; i < replicaCount; i++ {
		inst := &instance{address: strconv.Itoa(i), id: i, net: net}
		net.replicas = append(net.replicas, inst)
		net.addresses = append(net.addresses, inst.address)
	}
	config := readConfig()
	for i, inst := range net.replicas {
		inst.pbft = newPbftCore(uint64(i), config, inst)
		inst.pbft.replicaCount = replicaCount
		inst.pbft.f = f
		for _, fn := range initFn {
			fn(inst.pbft)
		}
	}

	return net
}

func (net *testnet) Close() {
	if net.closed {
		return
	}
	for _, inst := range net.replicas {
		inst.pbft.close()
	}
	net.cond.L.Lock()
	net.closed = true
	net.cond.Signal()
	net.cond.L.Unlock()
}
