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

package consensus

import (
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	pb "github.com/openblockchain/obc-peer/protos"
)

// Consenter is used to receive messages from the network
// Every consensus plugin needs to implement this interface
type Consenter interface {
	RecvMsg(msg *pb.OpenchainMessage, senderHandle *pb.PeerID) error
}

// Inquirer is used to retrieve info about the validating network
type Inquirer interface {
	GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error)
	GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error)
}

// Communicator is used to send messages to other validators
type Communicator interface {
	Broadcast(msg *pb.OpenchainMessage, peerType pb.PeerEndpoint_Type) error
	Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error
}

// NetworkStack is used to retrieve network info and send messages
type NetworkStack interface {
	Communicator
	Inquirer
}

// SecurityUtils is used to access the sign/verify methods from the crypto package
type SecurityUtils interface {
	Sign(msg []byte) ([]byte, error)
	Verify(peerID *pb.PeerID, signature []byte, message []byte) error
}

// ReadOnlyLedger is used for interrogating the blockchain
type ReadOnlyLedger interface {
	GetBlock(id uint64) (block *pb.Block, err error)
	GetCurrentStateHash() (stateHash []byte, err error)
	GetBlockchainSize() (uint64, error)
}

// UtilLedger contains additional useful utility functions for interrogating the blockchain
type UtilLedger interface {
	HashBlock(block *pb.Block) ([]byte, error)
	VerifyBlockchain(start, finish uint64) (uint64, error)
}

// WritableLedger is useful for updating the blockchain during state transfer
type WritableLedger interface {
	PutBlock(blockNumber uint64, block *pb.Block) error
	ApplyStateDelta(id interface{}, delta *statemgmt.StateDelta) error
	CommitStateDelta(id interface{}) error
	RollbackStateDelta(id interface{}) error
	EmptyState() error
}

// Ledger is an unrestricted union of reads, utilities, and updates
type Ledger interface {
	ReadOnlyLedger
	UtilLedger
	WritableLedger
}

// Executor is used to invoke transactions, potentially modifying the backing ledger
type Executor interface {
	BeginTxBatch(id interface{}) error
	ExecTxs(id interface{}, txs []*pb.Transaction) ([]byte, error)
	CommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error)
	RollbackTxBatch(id interface{}) error
	PreviewCommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error)
}

// RemoteLedgers is used to interrogate the blockchain of other replicas
type RemoteLedgers interface {
	GetRemoteBlocks(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error)
	GetRemoteStateSnapshot(replicaID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error)
	GetRemoteStateDeltas(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error)
}

// LedgerStack serves as interface to the blockchain-oriented activities, such as executing transactions, querying, and updating the ledger
type LedgerStack interface {
	Executor
	Ledger
	RemoteLedgers
}

// Stack is the set of stack-facing methods available to the consensus plugin
type Stack interface {
	NetworkStack
	SecurityUtils
	LedgerStack
}
