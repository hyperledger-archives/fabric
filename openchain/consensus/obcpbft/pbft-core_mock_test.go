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
	"strconv"
	"sync"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
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

func (mock *mockCPI) broadcast(msg []byte) {
	mock.broadcasted = append(mock.broadcasted, msg)
}

func (mock *mockCPI) verify(tx []byte) error {
	return nil
}

func (mock *mockCPI) execute(tx []byte) {
	mock.executed = append(mock.executed, tx)
}

func (mock *mockCPI) viewChange(uint64) {
}

// =============================================================================
// Fake network structures
// =============================================================================

type closableConsenter interface {
	consensus.Consenter
	Close()
}

type taggedMsg struct {
	id  int
	msg []byte
}

type testnet struct {
	f         int
	cond      *sync.Cond
	closed    bool
	replicas  []*instance
	msgs      []taggedMsg
	addresses []string
}

type instance struct {
	id        int
	addr      string
	pbft      *pbftCore
	consenter closableConsenter
	net       *testnet
	executed  [][]byte

	txId     interface{}
	curBatch []*pb.Transaction
	blocks   [][]*pb.Transaction

	deliver func([]byte)
}

func (inst *instance) broadcast(payload []byte) {
	net := inst.net
	net.cond.L.Lock()
	net.msgs = append(net.msgs, taggedMsg{inst.id, payload})
	net.cond.Signal()
	net.cond.L.Unlock()
}

func (inst *instance) verify(payload []byte) error {
	return nil
}

func (inst *instance) execute(payload []byte) {
	inst.executed = append(inst.executed, payload)
}

func (inst *instance) viewChange(uint64) {
}

func (inst *instance) GetReplicaHash() (self string, network []string, err error) {
	return inst.addr, inst.net.addresses, nil
}

func (inst *instance) GetReplicaID(addr string) (id uint64, err error) {
	for i, v := range inst.net.addresses {
		if v == addr {
			return uint64(i), nil
		}
	}
	err = fmt.Errorf("Couldn't find address in list of addresses in testnet")
	return uint64(0), err
}

func (inst *instance) Broadcast(msg *pb.OpenchainMessage) error {
	net := inst.net
	net.cond.L.Lock()
	net.msgs = append(net.msgs, taggedMsg{inst.id, msg.Payload})
	net.cond.Signal()
	net.cond.L.Unlock()
	return nil
}

func (inst *instance) Unicast(msgPayload []byte, receiver string) error {
	panic("not implemented yet")
}

func (inst *instance) BeginTxBatch(id interface{}) error {
	if inst.txId != nil {
		return fmt.Errorf("Tx batch is already active")
	}
	inst.txId = id
	inst.curBatch = nil
	return nil
}

func (inst *instance) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	inst.curBatch = append(inst.curBatch, txs...)
	errs := make([]error, len(txs)+1)
	return nil, errs
}

func (inst *instance) CommitTxBatch(id interface{}, txs []*pb.Transaction, proof []byte) error {
	if !reflect.DeepEqual(inst.txId, id) {
		return fmt.Errorf("Invalid batch ID")
	}
	if !reflect.DeepEqual(txs, inst.curBatch) {
		return fmt.Errorf("Tx list does not match executed Tx batch")
	}
	inst.txId = nil
	inst.blocks = append(inst.blocks, inst.curBatch)
	inst.curBatch = nil
	return nil
}

func (inst *instance) RollbackTxBatch(id interface{}) error {
	if !reflect.DeepEqual(inst.txId, id) {
		return fmt.Errorf("Invalid batch ID")
	}
	inst.curBatch = nil
	inst.txId = nil
	return nil
}

func (net *testnet) filterMsg(outMsg taggedMsg, filterFns ...func(bool, int, []byte) []byte) (msgs []taggedMsg) {
	msg := outMsg.msg
	for _, f := range filterFns {
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
		for _, f := range filterFns {
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

func (net *testnet) process(filterFns ...func(bool, int, []byte) []byte) error {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()

	for len(net.msgs) > 0 {
		msgs := net.msgs
		net.msgs = nil

		for _, taggedMsg := range msgs {
			for _, msg := range net.filterMsg(taggedMsg, filterFns...) {
				net.cond.L.Unlock()
				net.replicas[msg.id].deliver(msg.msg)
				net.cond.L.Lock()
			}
		}
	}

	return nil
}

func (net *testnet) processContinually(filterFns ...func(bool, int, []byte) []byte) {
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
				for _, msg := range net.filterMsg(taggedMsg, filterFns...) {
					net.cond.L.Unlock()
					net.replicas[msg.id].deliver(msg.msg)
					net.cond.L.Lock()
				}
			}
		}
	}
}

func makeTestnet(f int, initFn ...func(*instance)) *testnet {
	net := &testnet{f: f}
	net.cond = sync.NewCond(&sync.Mutex{})
	replicaCount := 3*f + 1
	for i := 0; i < replicaCount; i++ {
		inst := &instance{addr: strconv.Itoa(i), id: i, net: net} // XXX ugly hack
		net.replicas = append(net.replicas, inst)
		net.addresses = append(net.addresses, inst.addr)
	}

	for _, inst := range net.replicas {
		for _, fn := range initFn {
			fn(inst)
		}
	}

	return net
}

func (net *testnet) close() {
	if net.closed {
		return
	}
	for _, inst := range net.replicas {
		if inst.pbft != nil {
			inst.pbft.close()
		}
		if inst.consenter != nil {
			inst.consenter.Close()
		}
	}
	net.cond.L.Lock()
	net.closed = true
	net.cond.Signal()
	net.cond.L.Unlock()
}
