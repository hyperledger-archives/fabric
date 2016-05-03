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
	"math/rand"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/ledger/statemgmt"

	pb "github.com/hyperledger/fabric/protos"
	gp "google/protobuf"
)

type noopSecurity struct{}

func (ns *noopSecurity) Sign(msg []byte) ([]byte, error) {
	return nil, nil
}

func (ns *noopSecurity) Verify(peerID *pb.PeerID, signature []byte, message []byte) error {
	return nil
}

type mockPersist struct {
	store map[string][]byte
}

func (p *mockPersist) initialize() {
	if p.store == nil {
		p.store = make(map[string][]byte)
	}
}

func (p *mockPersist) ReadState(key string) ([]byte, error) {
	p.initialize()
	if val, ok := p.store[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("cannot find key %s", key)
}

func (p *mockPersist) ReadStateSet(prefix string) (map[string][]byte, error) {
	if p.store == nil {
		return nil, fmt.Errorf("no state yet")
	}
	ret := make(map[string][]byte)
	for k, v := range p.store {
		if len(k) >= len(prefix) && k[0:len(prefix)] == prefix {
			ret[k] = v
		}
	}
	return ret, nil
}

func (p *mockPersist) StoreState(key string, value []byte) error {
	p.initialize()
	p.store[key] = value
	return nil
}

func (p *mockPersist) DelState(key string) {
	p.initialize()
	delete(p.store, key)
}

// Create a message of type `Message_CHAIN_TRANSACTION`
func createOcMsgWithChainTx(iter int64) (msg *pb.Message) {
	txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_DEPLOY,
		Timestamp: txTime,
		Payload:   []byte(fmt.Sprint(iter)),
	}
	txPacked, _ := proto.Marshal(tx)
	msg = &pb.Message{
		Type:    pb.Message_CHAIN_TRANSACTION,
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

type omniProto struct {
	// Stack methods
	GetNetworkInfoImpl         func() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error)
	GetNetworkHandlesImpl      func() (self *pb.PeerID, network []*pb.PeerID, err error)
	BroadcastImpl              func(msg *pb.Message, peerType pb.PeerEndpoint_Type) error
	UnicastImpl                func(msg *pb.Message, receiverHandle *pb.PeerID) error
	SignImpl                   func(msg []byte) ([]byte, error)
	VerifyImpl                 func(peerID *pb.PeerID, signature []byte, message []byte) error
	GetBlockImpl               func(id uint64) (block *pb.Block, err error)
	GetCurrentStateHashImpl    func() (stateHash []byte, err error)
	GetBlockchainSizeImpl      func() (uint64, error)
	HashBlockImpl              func(block *pb.Block) ([]byte, error)
	VerifyBlockchainImpl       func(start, finish uint64) (uint64, error)
	PutBlockImpl               func(blockNumber uint64, block *pb.Block) error
	ApplyStateDeltaImpl        func(id interface{}, delta *statemgmt.StateDelta) error
	CommitStateDeltaImpl       func(id interface{}) error
	RollbackStateDeltaImpl     func(id interface{}) error
	EmptyStateImpl             func() error
	BeginTxBatchImpl           func(id interface{}) error
	ExecTxsImpl                func(id interface{}, txs []*pb.Transaction) ([]byte, error)
	CommitTxBatchImpl          func(id interface{}, metadata []byte) (*pb.Block, error)
	RollbackTxBatchImpl        func(id interface{}) error
	PreviewCommitTxBatchImpl   func(id interface{}, metadata []byte) ([]byte, error)
	GetRemoteBlocksImpl        func(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error)
	GetRemoteStateSnapshotImpl func(replicaID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error)
	GetRemoteStateDeltasImpl   func(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error)
	ReadStateImpl              func(key string) ([]byte, error)
	ReadStateSetImpl           func(prefix string) (map[string][]byte, error)
	StoreStateImpl             func(key string, value []byte) error
	DelStateImpl               func(key string)

	// Inner Stack methods
	broadcastImpl    func(msgPayload []byte)
	unicastImpl      func(msgPayload []byte, receiverID uint64) (err error)
	executeImpl      func(seqNo uint64, txRaw []byte)
	getStateImpl     func() []byte
	skipToImpl       func(seqNo uint64, snapshotID []byte, peers []uint64)
	validateImpl     func(txRaw []byte) error
	viewChangeImpl   func(curView uint64)
	signImpl         func(msg []byte) ([]byte, error)
	verifyImpl       func(senderID uint64, signature []byte, message []byte) error
	getLastSeqNoImpl func() (uint64, error)

	// Closable Consenter methods
	RecvMsgImpl func(ocMsg *pb.Message, senderHandle *pb.PeerID) error
	CloseImpl   func()
	deliverImpl func([]byte, *pb.PeerID)

	// Orderer methods
	ValidateImpl func(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID)
}

func (op *omniProto) GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
	if nil != op.GetNetworkInfoImpl {
		return op.GetNetworkInfoImpl()
	}

	panic("Unimplemented")
}
func (op *omniProto) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	if nil != op.GetNetworkHandlesImpl {
		return op.GetNetworkHandlesImpl()
	}

	panic("Unimplemented")
}
func (op *omniProto) Broadcast(msg *pb.Message, peerType pb.PeerEndpoint_Type) error {
	if nil != op.BroadcastImpl {
		return op.BroadcastImpl(msg, peerType)
	}

	panic("Unimplemented")
}
func (op *omniProto) Unicast(msg *pb.Message, receiverHandle *pb.PeerID) error {
	if nil != op.UnicastImpl {
		return op.UnicastImpl(msg, receiverHandle)
	}

	panic("Unimplemented")
}
func (op *omniProto) Sign(msg []byte) ([]byte, error) {
	if nil != op.SignImpl {
		return op.SignImpl(msg)
	}

	panic("Unimplemented")
}
func (op *omniProto) Verify(peerID *pb.PeerID, signature []byte, message []byte) error {
	if nil != op.VerifyImpl {
		return op.VerifyImpl(peerID, signature, message)
	}

	panic("Unimplemented")
}
func (op *omniProto) GetBlock(id uint64) (block *pb.Block, err error) {
	if nil != op.GetBlockImpl {
		return op.GetBlockImpl(id)
	}

	panic("Unimplemented")
}
func (op *omniProto) GetCurrentStateHash() (stateHash []byte, err error) {
	if nil != op.GetCurrentStateHashImpl {
		return op.GetCurrentStateHashImpl()
	}

	panic("Unimplemented")
}
func (op *omniProto) GetBlockchainSize() (uint64, error) {
	if nil != op.GetBlockchainSizeImpl {
		return op.GetBlockchainSizeImpl()
	}

	panic("Unimplemented")
}
func (op *omniProto) HashBlock(block *pb.Block) ([]byte, error) {
	if nil != op.HashBlockImpl {
		return op.HashBlockImpl(block)
	}

	panic("Unimplemented")
}
func (op *omniProto) VerifyBlockchain(start, finish uint64) (uint64, error) {
	if nil != op.VerifyBlockchainImpl {
		return op.VerifyBlockchainImpl(start, finish)
	}

	panic("Unimplemented")
}
func (op *omniProto) PutBlock(blockNumber uint64, block *pb.Block) error {
	if nil != op.PutBlockImpl {
		return op.PutBlockImpl(blockNumber, block)
	}

	panic("Unimplemented")
}
func (op *omniProto) ApplyStateDelta(id interface{}, delta *statemgmt.StateDelta) error {
	if nil != op.ApplyStateDeltaImpl {
		return op.ApplyStateDeltaImpl(id, delta)
	}

	panic("Unimplemented")
}
func (op *omniProto) CommitStateDelta(id interface{}) error {
	if nil != op.CommitStateDeltaImpl {
		return op.CommitStateDeltaImpl(id)
	}

	panic("Unimplemented")
}
func (op *omniProto) RollbackStateDelta(id interface{}) error {
	if nil != op.RollbackStateDeltaImpl {
		return op.RollbackStateDeltaImpl(id)
	}

	panic("Unimplemented")
}
func (op *omniProto) EmptyState() error {
	if nil != op.EmptyStateImpl {
		return op.EmptyStateImpl()
	}

	panic("Unimplemented")
}
func (op *omniProto) BeginTxBatch(id interface{}) error {
	if nil != op.BeginTxBatchImpl {
		return op.BeginTxBatchImpl(id)
	}

	panic("Unimplemented")
}
func (op *omniProto) ExecTxs(id interface{}, txs []*pb.Transaction) ([]byte, error) {
	if nil != op.ExecTxsImpl {
		return op.ExecTxsImpl(id, txs)
	}

	panic("Unimplemented")
}
func (op *omniProto) CommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error) {
	if nil != op.CommitTxBatchImpl {
		return op.CommitTxBatchImpl(id, metadata)
	}

	panic("Unimplemented")
}
func (op *omniProto) RollbackTxBatch(id interface{}) error {
	if nil != op.RollbackTxBatchImpl {
		return op.RollbackTxBatchImpl(id)
	}

	panic("Unimplemented")
}
func (op *omniProto) PreviewCommitTxBatch(id interface{}, metadata []byte) ([]byte, error) {
	if nil != op.PreviewCommitTxBatchImpl {
		return op.PreviewCommitTxBatchImpl(id, metadata)
	}

	panic("Unimplemented")
}
func (op *omniProto) GetRemoteBlocks(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
	if nil != op.GetRemoteBlocksImpl {
		return op.GetRemoteBlocksImpl(replicaID, start, finish)
	}

	panic("Unimplemented")
}
func (op *omniProto) GetRemoteStateSnapshot(replicaID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error) {
	if nil != op.GetRemoteStateSnapshotImpl {
		return op.GetRemoteStateSnapshotImpl(replicaID)
	}

	panic("Unimplemented")
}
func (op *omniProto) GetRemoteStateDeltas(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	if nil != op.GetRemoteStateDeltasImpl {
		return op.GetRemoteStateDeltasImpl(replicaID, start, finish)
	}

	panic("Unimplemented")
}

func (op *omniProto) broadcast(msgPayload []byte) {
	if nil != op.broadcastImpl {
		op.broadcastImpl(msgPayload)
		return
	}

	panic("Unimplemented")
}
func (op *omniProto) unicast(msgPayload []byte, receiverID uint64) (err error) {
	if nil != op.unicastImpl {
		return op.unicastImpl(msgPayload, receiverID)
	}

	panic("Unimplemented")
}
func (op *omniProto) execute(seqNo uint64, txRaw []byte) {
	if nil != op.executeImpl {
		op.executeImpl(seqNo, txRaw)
		return
	}

	panic("Unimplemented")
}
func (op *omniProto) skipTo(seqNo uint64, snapshotID []byte, peers []uint64) {
	if nil != op.skipToImpl {
		op.skipToImpl(seqNo, snapshotID, peers)
		return
	}

	panic("Unimplemented")
}
func (op *omniProto) validate(txRaw []byte) error {
	if nil != op.validateImpl {
		return op.validateImpl(txRaw)
	}

	panic("Unimplemented")
}
func (op *omniProto) viewChange(curView uint64) {
	if nil != op.viewChangeImpl {
		op.viewChangeImpl(curView)
		return
	}

	panic("Unimplemented")
}
func (op *omniProto) sign(msg []byte) ([]byte, error) {
	if nil != op.signImpl {
		return op.signImpl(msg)
	}

	panic("Unimplemented")
}
func (op *omniProto) verify(senderID uint64, signature []byte, message []byte) error {
	if nil != op.verifyImpl {
		return op.verifyImpl(senderID, signature, message)
	}

	panic("Unimplemented")
}

func (op *omniProto) RecvMsg(ocMsg *pb.Message, senderHandle *pb.PeerID) error {
	if nil != op.RecvMsgImpl {
		return op.RecvMsgImpl(ocMsg, senderHandle)
	}

	panic("Unimplemented")
}

func (op *omniProto) getLastSeqNo() (uint64, error) {
	if op.getLastSeqNoImpl != nil {
		return op.getLastSeqNoImpl()
	}

	return 0, fmt.Errorf("getLastSeqNo is not implemented")
}

func (op *omniProto) Close() {
	if nil != op.CloseImpl {
		op.CloseImpl()
		return
	}

	panic("Unimplemented")
}

func (op *omniProto) Validate(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID) {
	if nil != op.ValidateImpl {
		return op.ValidateImpl(seqNo, id)
	}

	panic("Unimplemented")

}

func (op *omniProto) deliver(msg []byte, target *pb.PeerID) {
	if nil != op.deliverImpl {
		op.deliverImpl(msg, target)
	}

	panic("Unimplemented")
}

func (op *omniProto) getState() []byte {
	if nil != op.getStateImpl {
		return op.getStateImpl()
	}

	panic("Unimplemented")
}

func (op *omniProto) ReadState(key string) ([]byte, error) {
	if nil != op.ReadStateImpl {
		return op.ReadStateImpl(key)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (op *omniProto) ReadStateSet(prefix string) (map[string][]byte, error) {
	if nil != op.ReadStateImpl {
		return op.ReadStateSetImpl(prefix)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (op *omniProto) DelState(key string) {
	if nil != op.DelStateImpl {
		op.DelStateImpl(key)
	}
}

func (op *omniProto) StoreState(key string, value []byte) error {
	if nil != op.ReadStateImpl {
		return op.StoreStateImpl(key, value)
	}
	return fmt.Errorf("unimplemented")
}

/*

	op := &omniProto{
		GetNetworkInfoImpl:         net.GetNetworkInfo,
		GetNetworkHandlesImpl:      net.GetNetworkHandles,
		BroadcastImpl:              net.Broadcast,
		UnicastImpl:                net.Unicast,
		SignImpl:                   security.Sign,
		VerifyImpl:                 security.Verify,
		GetBlockImpl:               ml.GetBlock,
		GetCurrentStateHashImpl:    ml.GetCurrentStateHash,
		GetBlockchainSizeImpl:      ml.GetBlockchainSize,
		HashBlockImpl:              ml.HashBlock,
		VerifyBlockchainImpl:       ml.VerifyBlockchain,
		PutBlockImpl:               ml.PutBlock,
		ApplyStateDeltaImpl:        ml.ApplyStateDelta,
		CommitStateDeltaImpl:       ml.CommitStateDelta,
		RollbackStateDeltaImpl:     ml.RollbackStateDelta,
		EmptyStateImpl:             ml.EmptyState,
		BeginTxBatchImpl:           ml.BeginTxBatch,
		ExecTxsImpl:                ml.ExecTxs,
		CommitTxBatchImpl:          ml.CommitTxBatch,
		RollbackTxBatchImpl:        ml.RollbackTxBatch,
		PreviewCommitTxBatchImpl:   ml.PreviewCommitTxBatch,
		GetRemoteBlocksImpl:        ml.GetRemoteBlocks,
		GetRemoteStateSnapshotImpl: ml.GetRemoteStateSnapshot,
		GetRemoteStateDeltasImpl:   ml.GetRemoteStateDeltas,
	}

*/
