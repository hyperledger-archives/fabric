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
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

type mockCPI struct {
	broadcasted [][]byte
	*instance
}

func newMock() *mockCPI {
	mock := &mockCPI{
		make([][]byte, 0),
		&instance{},
	}
	mock.instance.ledger = NewMockLedger(nil, nil)
	mock.instance.ledger.PutBlock(0, SimpleGetBlock(0))
	return mock
}

func (mock *mockCPI) broadcast(msg []byte) {
	mock.broadcasted = append(mock.broadcasted, msg)
}

func (mock *mockCPI) unicast(msg []byte, receiverID uint64) (err error) {
	panic("not implemented")
}

// =============================================================================
// Fake network structures
// =============================================================================

type closableConsenter interface {
	consensus.Consenter
	Close()
}

type taggedMsg struct {
	src int
	dst int
	msg []byte
}

type testnet struct {
	f        int
	cond     *sync.Cond
	closed   bool
	replicas []*instance
	msgs     []taggedMsg
	handles  []string
	filterFn func(int, int, []byte) []byte
}

type instance struct {
	id        int
	handle    string
	pbft      *pbftCore
	consenter closableConsenter
	net       *testnet
	ledger    consensus.Ledger

	deliver      func([]byte)
	execTxResult func([]*pb.Transaction) ([]byte, []error)
}

func (inst *instance) broadcast(payload []byte) {
	net := inst.net
	net.cond.L.Lock()
	/* msg := &Message{}
	_ = proto.Unmarshal(payload, msg)
	if fr := msg.GetFetchRequest(); fr != nil {
		fmt.Printf("Debug: replica %v broadcast fetch-request\n", inst.id)
	} */
	net.broadcastFilter(inst, payload)
	net.cond.Signal()
	net.cond.L.Unlock()
}

func (inst *instance) unicast(payload []byte, receiverID uint64) error {
	net := inst.net
	/* msg := &Message{}
	_ = proto.Unmarshal(payload, msg)
	if rr := msg.GetReturnRequest(); rr != nil {
		fmt.Printf("Debug: replica %v unicast return-request to %v\n", inst.id, receiverID)
		net.replicas[int(receiverID)].deliver(payload)
	} else {
		fmt.Printf("Debug: replica %v unicast (non return-request) to %v\n", inst.id, receiverID) */
	net.cond.L.Lock()
	net.msgs = append(net.msgs, taggedMsg{inst.id, int(receiverID), payload})
	net.cond.Signal()
	net.cond.L.Unlock()
	return nil
}

func (inst *instance) verify(payload []byte) error {
	return nil
}

func (inst *instance) execute(payload []byte) {

	tx := &pb.Transaction{
		Payload: payload,
	}

	txs := []*pb.Transaction{tx}
	txBatchID := base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(payload))

	if err := inst.BeginTxBatch(txBatchID); err != nil {
		fmt.Printf("Failed to begin transaction %s: %v", txBatchID, err)
		return
	}

	_, errs := inst.ExecTXs(txs)

	if errs[len(txs)] != nil {
		fmt.Printf("Fail to execute transaction %s: %v", txBatchID, errs)
		if err := inst.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

	if err := inst.CommitTxBatch(txBatchID, txs, nil); err != nil {
		fmt.Printf("Failed to commit transaction %s to the ledger: %v", txBatchID, err)
		if err = inst.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

}

func (inst *instance) viewChange(uint64) {
}

func (inst *instance) GetNetworkHandles() (self string, network []string, err error) {
	return inst.handle, inst.net.handles, nil
}

func (inst *instance) GetReplicaHandle(id uint64) (handle string, err error) {
	_, network, _ := inst.GetNetworkHandles()
	if int(id) > (len(network) - 1) {
		return handle, fmt.Errorf("Replica ID is out of bounds")
	}
	return network[int(id)], nil
}

func (inst *instance) GetReplicaID(handle string) (id uint64, err error) {
	_, network, _ := inst.GetNetworkHandles()
	for i, v := range network {
		if v == handle {
			return uint64(i), nil
		}
	}
	err = fmt.Errorf("Couldn't find handle in list of handles in testnet")
	return
}

// Broadcast delivers to all replicas.  In contrast to the stack
// Broadcast, this will also deliver back to the replica.  We keep
// this behavior, because it exposes subtle bugs in the
// implementation.
func (inst *instance) Broadcast(msg *pb.OpenchainMessage) error {
	net := inst.net
	net.cond.L.Lock()
	net.broadcastFilter(inst, msg.Payload)
	net.cond.Signal()
	net.cond.L.Unlock()
	return nil
}

func (inst *instance) Unicast(msg *pb.OpenchainMessage, receiver string) error {
	net := inst.net
	net.cond.L.Lock()
	receiverID, err := inst.GetReplicaID(receiver)
	if err != nil {
		return fmt.Errorf("Couldn't unicast message to %s: %v", receiver, err)
	}
	net.msgs = append(net.msgs, taggedMsg{inst.id, int(receiverID), msg.Payload})
	net.cond.Signal()
	net.cond.L.Unlock()
	return nil
}

func (inst *instance) BeginTxBatch(id interface{}) error {
	return inst.ledger.BeginTxBatch(id)
}

func (inst *instance) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	return inst.ledger.ExecTXs(txs)
}

func (inst *instance) CommitTxBatch(id interface{}, txs []*pb.Transaction, proof []byte) error {
	return inst.ledger.CommitTxBatch(id, txs, proof)
}

func (inst *instance) PreviewCommitTxBatchBlock(id interface{}, txs []*pb.Transaction, proof []byte) (*pb.Block, error) {
	return inst.ledger.PreviewCommitTxBatchBlock(id, txs, proof)
}

func (inst *instance) RollbackTxBatch(id interface{}) error {
	return inst.ledger.RollbackTxBatch(id)
}

func (inst *instance) GetBlock(id uint64) (block *pb.Block, err error) {
	return inst.ledger.GetBlock(id)
}
func (inst *instance) GetCurrentStateHash() (stateHash []byte, err error) {
	return inst.ledger.GetCurrentStateHash()
}
func (inst *instance) GetBlockchainSize() (uint64, error) {
	return inst.ledger.GetBlockchainSize()
}
func (inst *instance) HashBlock(block *pb.Block) ([]byte, error) {
	return inst.ledger.HashBlock(block)
}
func (inst *instance) PutBlock(blockNumber uint64, block *pb.Block) error {
	return inst.ledger.PutBlock(blockNumber, block)
}
func (inst *instance) ApplyStateDelta(delta []byte, unapply bool) error {
	return inst.ledger.ApplyStateDelta(delta, unapply)
}
func (inst *instance) EmptyState() error {
	return inst.ledger.EmptyState()
}
func (inst *instance) VerifyBlockchain(start, finish uint64) (uint64, error) {
	return inst.ledger.VerifyBlockchain(start, finish)
}
func (inst *instance) GetRemoteBlocks(replicaId uint64, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
	return inst.ledger.GetRemoteBlocks(replicaId, start, finish)
}
func (inst *instance) GetRemoteStateSnapshot(replicaId uint64) (<-chan *pb.SyncStateSnapshot, error) {
	return inst.ledger.GetRemoteStateSnapshot(replicaId)
}
func (inst *instance) GetRemoteStateDeltas(replicaId uint64, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	return inst.ledger.GetRemoteStateDeltas(replicaId, start, finish)
}

func (net *testnet) broadcastFilter(inst *instance, payload []byte) {
	if net.filterFn != nil {
		payload = net.filterFn(inst.id, -1, payload)
	}
	if payload != nil {
		/* msg := &Message{}
		_ = proto.Unmarshal(payload, msg)
		if fr := msg.GetFetchRequest(); fr != nil {
			// treat fetch-request as a high-priority message that needs to be processed ASAP
			fmt.Printf("Debug: replica %v broadcastFilter for fetch-request\n", inst.id)
			net.deliverFilter(taggedMsg{inst.id, -1, payload})
		} else { */
		net.msgs = append(net.msgs, taggedMsg{inst.id, -1, payload})
	}
}

func (net *testnet) deliverFilter(msg taggedMsg) {
	if msg.dst == -1 {
		for id, inst := range net.replicas {
			payload := msg.msg
			if net.filterFn != nil {
				payload = net.filterFn(msg.src, id, payload)
			}
			if payload != nil {
				inst.deliver(msg.msg)
			}
		}
	} else {
		net.replicas[msg.dst].deliver(msg.msg)
	}
}

func (net *testnet) process() error {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()

	for len(net.msgs) > 0 {
		msg := net.msgs[0]
		fmt.Printf("Debug: process iteration (%d messages to go, delivering now to destination %v)\n", len(net.msgs), msg.dst)
		net.msgs = net.msgs[1:]
		net.cond.L.Unlock()
		net.deliverFilter(msg)
		net.cond.L.Lock()
	}

	return nil
}

func (net *testnet) processContinually() {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()
	for {
		if net.closed {
			break
		}
		if len(net.msgs) == 0 {
			net.cond.Wait()
		}
		net.cond.L.Unlock()
		net.process()
		net.cond.L.Lock()
	}
}

func makeTestnet(f int, initFn ...func(*instance)) *testnet {
	net := &testnet{f: f}
	net.cond = sync.NewCond(&sync.Mutex{})
	replicaCount := 3*f + 1

	for i := uint64(0); i < uint64(replicaCount); i++ {
	}

	ledgers := make(map[uint64]ReadOnlyLedger, replicaCount)
	for i := 0; i < replicaCount; i++ {
		inst := &instance{handle: "vp" + strconv.Itoa(i), id: i, net: net}
		ml := NewMockLedger(&ledgers, nil)
		ml.inst = inst
		ml.PutBlock(0, SimpleGetBlock(0))
		ledgers[uint64(i)] = ml
		inst.ledger = ml
		net.replicas = append(net.replicas, inst)
		net.handles = append(net.handles, inst.handle)
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
