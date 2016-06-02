/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consensus

import (
	pb "github.com/hyperledger/fabric/protos"
)

// Consenter is used to receive messages from the network
// Every consensus plugin needs to implement this interface
type Consenter interface {
	RecvMsg(msg *pb.Message, senderHandle *pb.PeerID) error // Called serially with incoming messages from gRPC
	StateUpdated(tag uint64, id []byte)                     // Called when state transfer completes, serial with StateUpdating
	StateUpdating(tag uint64, id []byte)                    // Called when SkipTo causes state transfer to start serial with StateUpdated
}

// Inquirer is used to retrieve info about the validating network
type Inquirer interface {
	GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error)
	GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error)
}

// Communicator is used to send messages to other validators
type Communicator interface {
	Broadcast(msg *pb.Message, peerType pb.PeerEndpoint_Type) error
	Unicast(msg *pb.Message, receiverHandle *pb.PeerID) error
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
	GetBlockchainSize() uint64
	GetBlockchainInfoBlob() []byte
	GetBlockHeadMetadata() ([]byte, error)
}

// Executor is used to invoke transactions, potentially modifying the backing ledger
type Executor interface {
	BeginTxBatch(id interface{}) error
	ExecTxs(id interface{}, txs []*pb.Transaction) ([]byte, error)
	CommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error)
	RollbackTxBatch(id interface{}) error
	PreviewCommitTxBatch(id interface{}, metadata []byte) ([]byte, error)
}

// LedgerManager is used to manipulate the state of the ledger
type LedgerManager interface {
	SkipTo(tag uint64, id []byte, peers []*pb.PeerID) // SkipTo tells state transfer to bring the ledger to a particular state, it should generally be preceeded/proceeded by Invalidate/Validate
	InvalidateState()                                 // Invalidate informs the ledger that it is out of date and should reject queries
	ValidateState()                                   // Validate informs the ledger that it is back up to date and should resume replying to queries
}

// StatePersistor is used to store consensus state which should survive a process crash
type StatePersistor interface {
	StoreState(key string, value []byte) error
	ReadState(key string) ([]byte, error)
	ReadStateSet(prefix string) (map[string][]byte, error)
	DelState(key string)
}

// Stack is the set of stack-facing methods available to the consensus plugin
type Stack interface {
	NetworkStack
	SecurityUtils
	Executor
	LedgerManager
	ReadOnlyLedger
	StatePersistor
}
