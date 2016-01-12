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

package helper

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/peer"
	pb "github.com/openblockchain/obc-peer/protos"
)

// =============================================================================
// Structure definitions go here
// =============================================================================

// Helper contains the reference to coordinator for broadcasts/unicasts.
type Helper struct {
	coordinator peer.MessageHandlerCoordinator
}

// =============================================================================
// Constructors go here
// =============================================================================

// NewHelper constructs the consensus helper object.
func NewHelper(mhc peer.MessageHandlerCoordinator) consensus.CPI {
	return &Helper{coordinator: mhc}
}

// =============================================================================
// Stack-facing implementation goes here
// =============================================================================

// GetNetworkHandles returns the handles (MVP: hashed raw enrollment certificates) of the current replica and the whole network of VPs
func (h *Helper) GetNetworkHandles() (self string, network []string, err error) {
	ep, err := h.coordinator.GetPeerEndpoint()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve own endpoint: %v", err)
	}
	self = ep.ID.Name

	peersMsg, err := h.coordinator.GetPeers()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve list of peers: %v", err)
	}
	peers := peersMsg.GetPeers()
	for _, endpoint := range peers {
		if endpoint.Type == pb.PeerEndpoint_VALIDATOR {
			network = append(network, endpoint.ID.Name)
		}
	}
	network = append(network, self)
	// sort.Strings(network)

	return
}

// GetReplicaHandle returns the handle that corresponds to a replica ID (uin64 assigned to it for PBFT)
func (h *Helper) GetReplicaHandle(id uint64) (handle string, err error) {
	handle = "vp" + strconv.FormatUint(id, 10)
	return
}

// GetReplicaID returns the uint ID corresponding to a replica handle
func (h *Helper) GetReplicaID(handle string) (id uint64, err error) {
	// if the handle starts with "vp*", short-circuit the function
	// consider this our debugging mode for when we don't have a fixed VP list
	// and want to instantiate the Consenter with the proper ID
	if startsWith := strings.HasPrefix(handle, "vp"); startsWith {
		id, err = strconv.ParseUint(handle[2:], 10, 64)
		if err != nil {
			return id, fmt.Errorf("Error extracting ID from \"%s\" handle: %v", handle, err)
		}
		return
	}
	err = fmt.Errorf(`For MVP, set the VP's peer.id to vpX,
		where X is a unique integer between 0 and N-1
		(N being the maximum number of VPs in the network`)
	return
}

// Broadcast sends a message to all validating peers.
func (h *Helper) Broadcast(msg *pb.OpenchainMessage) error {
	errors := h.coordinator.Broadcast(msg)
	if len(errors) > 0 {
		return fmt.Errorf("Couldn't broadcast successfully")
	}
	return nil
}

// Unicast sends a message to a specified receiver.
func (h *Helper) Unicast(msg *pb.OpenchainMessage, receiverHandle string) error {
	return h.coordinator.Unicast(msg, receiverHandle)
}

// BeginTxBatch gets invoked when the next round of transaction-batch
// execution begins.
func (h *Helper) BeginTxBatch(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.BeginTxBatch(id); err != nil {
		return fmt.Errorf("Failed to begin transaction with the ledger: %v", err)
	}
	return nil
}

// ExecTXs executes all the transactions listed in the txs array
// one-by-one. If all the executions are successful, it returns
// the candidate global state hash, and nil error array.
func (h *Helper) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	return chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs, h.coordinator.GetSecHelper())
}

// CommitTxBatch gets invoked when the current transaction-batch needs
// to be committed. This function returns successfully iff the
// transactions details and state changes (that may have happened
// during execution of this transaction-batch) have been committed to
// permanent storage.
func (h *Helper) CommitTxBatch(id interface{}, transactions []*pb.Transaction, metadata []byte) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.CommitTxBatch(id, transactions, metadata); err != nil {
		return fmt.Errorf("Failed to commit transaction to the ledger: %v", err)
	}
	return nil
}

// RollbackTxBatch discards all the state changes that may have taken
// place during the execution of current transaction-batch.
func (h *Helper) RollbackTxBatch(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.RollbackTxBatch(id); err != nil {
		return fmt.Errorf("Failed to rollback transaction with the ledger: %v", err)
	}
	return nil
}

func (h *Helper) PreviewCommitTxBatchBlock(id interface{}) (*pb.Block, error) {
	// TODO
	return nil, fmt.Errorf("Unimplemented")
}

// GetBlock returns a block from the chain
func (h *Helper) GetBlock(blockNumber uint64) (block *pb.Block, err error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.GetBlockByNumber(blockNumber)
}

// GetCurrentStateHash returns the current/temporary state hash
func (h *Helper) GetCurrentStateHash() (stateHash []byte, err error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.GetTempStateHash()
}

// GetBlockchainSize returns the current size of the blockchain
func (h *Helper) GetBlockchainSize() (uint64, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return 0, fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.GetBlockchainSize(), nil
}

// GetBlockchainSize returns the hash of the included block, useful for mocking
func (h *Helper) HashBlock(block *pb.Block) ([]byte, error) {
	return block.GetHash()
}

// PutBlock inserts a raw block into the blockchain at the specified index, nearly no error checking is performed
func (h *Helper) PutBlock(blockNumber uint64, block *pb.Block) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.PutRawBlock(block, blockNumber)
}

// TODO, waiting to see the streaming implementation to define this API nicely
func (h *Helper) ApplyStateDelta(delta []byte, unapply bool) error {
	return // TODO implement
}

// EmptyState completely empties the state and prepares it to restore a snapshot
func (h *Helper) EmptyState() error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.DeleteALLStateKeysAndValues()
}

// VerifyBlockchain checks the integrity of the blockchain between indices start and finish,
// returning the first block who's PreviousBlockHash field does not match the hash of the previous block
func (h *Helper) VerifyBlockchain(start, finish uint64) (uint64, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return finish, fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.VerifyChain(start, finish)
}

// GetRemoteBlocks will return a channel to stream blocks from the desired replicaId
func (h *Helper) GetRemoteBlocks(replicaId uint64, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
	return nil, fmt.Errorf("Unimplemented")
}

// GetRemoteStateSnapshot will return a channel to stream a state snapshot from the desired replicaId
func (h *Helper) GetRemoteStateSnapshot(replicaId uint64) (<-chan *pb.SyncStateSnapshot, error) {
	return nil, fmt.Errorf("Unimplemented")
}

// GetRemoteStateDeltas  will return a channel to stream a state snapshot deltas from the desired replicaId
func (h *Helper) GetRemoteStateDeltas(replicaId uint64, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	return nil, fmt.Errorf("Unimplemented")
}
