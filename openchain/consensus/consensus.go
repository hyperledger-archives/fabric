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
	RecvMsg(msg *pb.OpenchainMessage) error
}

// Communicator is used to send messages to other validators
type Communicator interface {
	GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error)  //TODO: should network be a map rather than an array ?
	Broadcast(msg *pb.OpenchainMessage) error
	Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error
}

//TTD
type SecurityUtils interface {
   Sign(msg []byte) ([]byte, error)                                    // sign a msg with this replica's signing key.
   Verify(peerID *pb.PeerID, signature []byte, message []byte) error   // verify that given signature is valid under the given replicaID's verification key. If replicaID is nil,
                                                                       //  use this replica's verification key. If signature is valid, function return nil
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
	ExecTXs(txs []*pb.Transaction) ([]byte, []error)
	CommitTxBatch(id interface{}, transactions []*pb.Transaction, transactionsResults []*pb.TransactionResult, metadata []byte) error
	RollbackTxBatch(id interface{}) error
	PreviewCommitTxBatchBlock(id interface{}, transactions []*pb.Transaction, metadata []byte) (*pb.Block, error)
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

// CPI (Consensus Programming Interface) is the set of stack-facing methods available to the consensus plugin
type CPI interface {
	Communicator
   SecurityUtils //TTD
	BlockchainPackage
	LedgerStack
}
