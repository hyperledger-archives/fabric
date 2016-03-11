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
	gp "google/protobuf"
	"math/rand"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	pb "github.com/openblockchain/obc-peer/protos"
)

type closableConsenter interface {
	consensus.Consenter
	Close()
	idleChan() <-chan struct{}
}

type taggedMsg struct {
	src int
	dst int
	msg []byte
}

type testnet struct {
	N        int
	f        int
	closed   bool // TODO remove
	replicas []*instance
	msgs     chan taggedMsg
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
	execTxResult func([]*pb.Transaction) ([]byte, error)
}

func (inst *instance) Sign(msg []byte) ([]byte, error) {
	return msg, nil
}
func (inst *instance) Verify(peerID *pb.PeerID, signature []byte, message []byte) error {
	return nil
}

func (inst *instance) GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
	oSelf, oNetwork, _ := inst.GetNetworkHandles()
	self = &pb.PeerEndpoint{
		ID:   oSelf,
		Type: pb.PeerEndpoint_VALIDATOR,
	}

	network = make([]*pb.PeerEndpoint, len(oNetwork))
	for i, id := range oNetwork {
		network[i] = &pb.PeerEndpoint{
			ID:   id,
			Type: pb.PeerEndpoint_VALIDATOR,
		}
	}
	return
}

func (inst *instance) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	self = inst.handle
	if nil == inst.net {
		err = fmt.Errorf("Network not initialized")
		return
	}
	network = inst.net.handles
	return
}

// Broadcast delivers to all replicas.  In contrast to the stack
// Broadcast, this will also deliver back to the replica.  We keep
// this behavior, because it exposes subtle bugs in the
// implementation.
func (inst *instance) Broadcast(msg *pb.OpenchainMessage, peerType pb.PeerEndpoint_Type) error {
	logger.Debug("TEST: forwarding request payload %p to broadcast filter", msg.Payload)
	inst.net.broadcastFilter(inst, msg.Payload)
	return nil
}

func (inst *instance) Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error {
	receiverID, err := getValidatorID(receiverHandle)
	if err != nil {
		return fmt.Errorf("Couldn't unicast message to %s: %v", receiverHandle.Name, err)
	}
	inst.net.msgs <- taggedMsg{inst.id, int(receiverID), msg.Payload}
	return nil
}

func (inst *instance) BeginTxBatch(id interface{}) error {
	return inst.ledger.BeginTxBatch(id)
}

func (inst *instance) ExecTxs(id interface{}, txs []*pb.Transaction) ([]byte, error) {
	return inst.ledger.ExecTxs(id, txs)
}

func (inst *instance) CommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error) {
	return inst.ledger.CommitTxBatch(id, metadata)
}

func (inst *instance) PreviewCommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error) {
	return inst.ledger.PreviewCommitTxBatch(id, metadata)
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
	if net.closed {
		logger.Error("WARNING! Attempted to send a request to a closed network, ignoring")
		return
	}
	if net.filterFn != nil {
		tmp := payload
		payload = net.filterFn(inst.id, -1, payload)
		logger.Debug("TEST: filtered message %p to %p", tmp, payload)
	}
	if payload != nil {
		/* msg := &Message{}
		_ = proto.Unmarshal(payload, msg)
		if fr := msg.GetFetchRequest(); fr != nil {
			// treat fetch-request as a high-priority message that needs to be processed ASAP
			fmt.Printf("Debug: replica %v broadcastFilter for fetch-request\n", inst.id)
			net.deliverFilter(taggedMsg{inst.id, -1, payload})
		} else { */
		logger.Debug("TEST: attempting to queue message %p", payload)
		net.msgs <- taggedMsg{inst.id, -1, payload}
		logger.Debug("TEST: message queued successfully %p", payload)
	}
}

func (net *testnet) deliverFilter(msg taggedMsg, senderID int) {
	senderHandle := net.handles[senderID]
	if msg.dst == -1 {
		for id, inst := range net.replicas {
			if msg.src == id {
				// do not deliver to local replica
				continue
			}
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

func (net *testnet) idleFan() <-chan struct{} {
	res := make(chan struct{})

	go func() {
		for _, inst := range net.replicas {
			if inst.consenter != nil {
				<-inst.consenter.idleChan()
			}
		}
		logger.Debug("TEST: closing idleChan")
		// Only close to the channel after all the consenters have written to us
		close(res)
	}()

	return res
}

func (net *testnet) processMessageFromChannel(msg taggedMsg, ok bool) bool {
	if !ok {
		logger.Debug("TEST: message channel closed, exiting\n")
		return false
	}
	logger.Debug("TEST: new message, delivering\n")
	net.deliverFilter(msg, msg.src)
	return true
}

func (net *testnet) process() error {
	for {
		logger.Debug("TEST: process looping")
		select {
		case msg, ok := <-net.msgs:
			logger.Debug("TEST: processing message without testing for idle")
			if !net.processMessageFromChannel(msg, ok) {
				return nil
			}
		default:
			logger.Debug("TEST: processing message or testing for idle")
			select {
			case <-net.idleFan():
				logger.Debug("TEST: exiting process loop because of idleness")
				return nil
			case msg, ok := <-net.msgs:
				if !net.processMessageFromChannel(msg, ok) {
					return nil
				}
			}
		}
	}

	return nil
}

func (net *testnet) processContinually() {
	for {
		msg, ok := <-net.msgs
		if !net.processMessageFromChannel(msg, ok) {
			return
		}
	}
}

func makeTestnet(N int, initFn ...func(*instance)) *testnet {
	f := N / 3
	net := &testnet{f: f, N: N}
	net.msgs = make(chan taggedMsg, 100)

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

func (net *testnet) clearMessages() {
	for {
		select {
		case <-net.msgs:
		default:
			return
		}
	}
}

func (net *testnet) close() {
	net.closed = true
	close(net.msgs)
}

// Create a message of type `OpenchainMessage_CHAIN_TRANSACTION`
func createOcMsgWithChainTx(iter int64) (msg *pb.OpenchainMessage) {
	txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW,
		Timestamp: txTime,
		Payload:   []byte(fmt.Sprint(iter)),
	}
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
