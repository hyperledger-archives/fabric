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
	gp "google/protobuf"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
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

func (mock *mockCPI) sign(msg []byte) ([]byte, error) {
	return msg, nil
}

func (mock *mockCPI) verify(senderID uint64, signature []byte, message []byte) error {
	return nil
}

func (mock *mockCPI) broadcast(msg []byte) {
	mock.broadcasted = append(mock.broadcasted, msg)
}

func (mock *mockCPI) unicast(msg []byte, receiverID uint64) (err error) {
	panic("not implemented")
}

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
	N        int
	f        int
	cond     *sync.Cond
	closed   bool
	replicas []*instance
	msgs     []taggedMsg
	handles  []*pb.PeerID
	filterFn func(int, int, []byte) []byte
}

type instance struct {
	id        int
	handle    *pb.PeerID
	pbft      *pbftCore
	consenter closableConsenter
	net       *testnet
	ledger    consensus.LedgerStack

	deliver      func([]byte, *pb.PeerID)
	execTxResult func([]*pb.Transaction) ([]byte, []error)
}

func (inst *instance) Sign(msg []byte) ([]byte, error) {
	return msg, nil
}
func (inst *instance) Verify(peerID *pb.PeerID, signature []byte, message []byte) error {
	return nil
}

func (inst *instance) sign(msg []byte) ([]byte, error) {
	return msg, nil
}

func (inst *instance) verify(replicaID uint64, signature []byte, message []byte) error {
	return nil
}

func (inst *instance) broadcast(payload []byte) {
	net := inst.net
	net.cond.L.Lock()
	net.broadcastFilter(inst, payload)
	net.cond.Signal()
	net.cond.L.Unlock()
}

func (inst *instance) unicast(payload []byte, receiverID uint64) error {
	net := inst.net
	net.cond.L.Lock()
	net.msgs = append(net.msgs, taggedMsg{inst.id, int(receiverID), payload})
	net.cond.Signal()
	net.cond.L.Unlock()
	return nil
}

func (inst *instance) validate(payload []byte) error {
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

	result, errs := inst.ExecTXs(txs)

	if errs[len(txs)] != nil {
		fmt.Printf("Fail to execute transaction %s: %v", txBatchID, errs)
		if err := inst.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

	txResult := []*pb.TransactionResult{
		&pb.TransactionResult{
			Result: result,
		},
	}

	if err := inst.CommitTxBatch(txBatchID, txs, txResult, nil); err != nil {
		fmt.Printf("Failed to commit transaction %s to the ledger: %v", txBatchID, err)
		if err = inst.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

}

func (inst *instance) viewChange(uint64) {
}

func (inst *instance) GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
	panic("Not implemented yet")
}

func (inst *instance) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	self = inst.handle
	network = inst.net.handles
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

func (inst *instance) Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error {
	net := inst.net
	net.cond.L.Lock()
	receiverID, err := getValidatorID(receiverHandle)
	if err != nil {
		return fmt.Errorf("Couldn't unicast message to %s: %v", receiverHandle.Name, err)
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

func (inst *instance) CommitTxBatch(id interface{}, txs []*pb.Transaction, txResults []*pb.TransactionResult, metadata []byte) error {
	return inst.ledger.CommitTxBatch(id, txs, txResults, metadata)
}

func (inst *instance) PreviewCommitTxBatchBlock(id interface{}, txs []*pb.Transaction, metadata []byte) (*pb.Block, error) {
	return inst.ledger.PreviewCommitTxBatchBlock(id, txs, metadata)
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
func (inst *instance) ApplyStateDelta(id interface{}, delta *statemgmt.StateDelta) error {
	return inst.ledger.ApplyStateDelta(id, delta)
}
func (inst *instance) CommitStateDelta(id interface{}) error {
	return inst.ledger.CommitStateDelta(id)
}
func (inst *instance) RollbackStateDelta(id interface{}) error {
	return inst.ledger.RollbackStateDelta(id)
}
func (inst *instance) EmptyState() error {
	return inst.ledger.EmptyState()
}
func (inst *instance) VerifyBlockchain(start, finish uint64) (uint64, error) {
	return inst.ledger.VerifyBlockchain(start, finish)
}
func (inst *instance) GetRemoteBlocks(peerID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
	return inst.ledger.GetRemoteBlocks(peerID, start, finish)
}
func (inst *instance) GetRemoteStateSnapshot(peerID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error) {
	return inst.ledger.GetRemoteStateSnapshot(peerID)
}
func (inst *instance) GetRemoteStateDeltas(peerID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	return inst.ledger.GetRemoteStateDeltas(peerID, start, finish)
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

func (net *testnet) deliverFilter(msg taggedMsg, senderID int) {
	senderHandle := net.handles[senderID]
	if msg.dst == -1 {
		for id, inst := range net.replicas {
			payload := msg.msg
			if net.filterFn != nil {
				payload = net.filterFn(msg.src, id, payload)
			}
			if payload != nil {
				inst.deliver(msg.msg, senderHandle)
			}
		}
	} else {
		net.replicas[msg.dst].deliver(msg.msg, senderHandle)
	}
}

func (net *testnet) process() error {
	net.cond.L.Lock()
	defer net.cond.L.Unlock()

	for len(net.msgs) > 0 {
		msg := net.msgs[0]
		net.msgs = net.msgs[1:]
		net.cond.L.Unlock()
		net.deliverFilter(msg, msg.src)
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

func makeTestnet(N int, initFn ...func(*instance)) *testnet {
	f := N / 3
	net := &testnet{f: f, N: N}
	net.cond = sync.NewCond(&sync.Mutex{})

	for i := uint64(0); i < uint64(N); i++ {
	}

	ledgers := make(map[pb.PeerID]consensus.ReadOnlyLedger, N)
	for i := 0; i < N; i++ {
		inst := &instance{handle: &pb.PeerID{Name: "vp" + strconv.Itoa(i)}, id: i, net: net}
		ml := NewMockLedger(&ledgers, nil)
		ml.inst = inst
		ml.PutBlock(0, SimpleGetBlock(0))
		handle, _ := getValidatorHandle(uint64(i))
		ledgers[*handle] = ml
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

// Create a message of type `OpenchainMessage_CHAIN_TRANSACTION`
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

func generateBroadcaster(validatorCount int) (requestBroadcaster int) {
	seed := rand.NewSource(time.Now().UnixNano())
	rndm := rand.New(seed)
	requestBroadcaster = rndm.Intn(validatorCount)
	return
}
