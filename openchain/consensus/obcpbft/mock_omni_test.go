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
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"

	pb "github.com/openblockchain/obc-peer/protos"
)

type omniProto struct {
	// Stack methods
	GetNetworkInfoImpl         func() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error)
	GetNetworkHandlesImpl      func() (self *pb.PeerID, network []*pb.PeerID, err error)
	BroadcastImpl              func(msg *pb.OpenchainMessage, peerType pb.PeerEndpoint_Type) error
	UnicastImpl                func(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error
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
	PreviewCommitTxBatchImpl   func(id interface{}, metadata []byte) (*pb.Block, error)
	GetRemoteBlocksImpl        func(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error)
	GetRemoteStateSnapshotImpl func(replicaID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error)
	GetRemoteStateDeltasImpl   func(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error)

	// Inner Stack methods
	broadcastImpl  func(msgPayload []byte)
	unicastImpl    func(msgPayload []byte, receiverID uint64) (err error)
	executeImpl    func(seqNo uint64, txRaw []byte, execInfo *ExecutionInfo)
	skipToImpl     func(seqNo uint64, snapshotID []byte, peers []uint64)
	validStateImpl func(seqNo uint64, id []byte, peers []uint64)
	validateImpl   func(txRaw []byte) error
	viewChangeImpl func(curView uint64)
	signImpl       func(msg []byte) ([]byte, error)
	verifyImpl     func(senderID uint64, signature []byte, message []byte) error

	// Closable Consenter methods
	RecvMsgImpl        func(ocMsg *pb.OpenchainMessage, senderHandle *pb.PeerID) error
	CloseImpl          func()
	blockUntilIdleImpl func()

	// Orderer methods
	CheckpointImpl func(seqNo uint64, id []byte)
	ValidateImpl   func(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID)
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
func (op *omniProto) Broadcast(msg *pb.OpenchainMessage, peerType pb.PeerEndpoint_Type) error {
	if nil != op.BroadcastImpl {
		return op.BroadcastImpl(msg, peerType)
	}

	panic("Unimplemented")
}
func (op *omniProto) Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error {
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
func (op *omniProto) PreviewCommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error) {
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
func (op *omniProto) execute(seqNo uint64, txRaw []byte, execInfo *ExecutionInfo) {
	if nil != op.executeImpl {
		op.executeImpl(seqNo, txRaw, execInfo)
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
func (op *omniProto) validState(seqNo uint64, id []byte, peers []uint64) {
	if nil != op.validStateImpl {
		op.validStateImpl(seqNo, id, peers)
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

func (op *omniProto) RecvMsg(ocMsg *pb.OpenchainMessage, senderHandle *pb.PeerID) error {
	if nil != op.RecvMsgImpl {
		return op.RecvMsgImpl(ocMsg, senderHandle)
	}

	panic("Unimplemented")
}

func (op *omniProto) Close() {
	if nil != op.CloseImpl {
		op.CloseImpl()
		return
	}

	panic("Unimplemented")
}

func (op *omniProto) blockUntilIdle() {
	if nil != op.blockUntilIdleImpl {
		op.blockUntilIdleImpl()
		return
	}

	panic("Unimplemented")
}

func (op *omniProto) Checkpoint(seqNo uint64, id []byte) {
	if nil != op.CheckpointImpl {
		op.CheckpointImpl(seqNo, id)
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
