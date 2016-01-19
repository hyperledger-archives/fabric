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

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/peer"
	pb "github.com/openblockchain/obc-peer/protos"
   crypto "github.com/openblockchain/obc-peer/openchain/crypto"   //TTD
)

// Helper contains the reference to the peer's MessageHandlerCoordinator
type Helper struct {
	coordinator peer.MessageHandlerCoordinator
   secHelper   crypto.Peer     //TTD
}

// NewHelper constructs the consensus helper object
func NewHelper(mhc peer.MessageHandlerCoordinator) consensus.CPI {
	return &Helper{coordinator: mhc, secHelper: mhc.GetSecHelper()} //TTD
}

// TTD
func (h *Helper) Sign(msg []byte) ([]byte, error) {
   return h.secHelper.Sign(msg)
}

// TTD
func (h *Helper) Verify(replicaID *pb.PeerID, signature []byte, message []byte) error {
   // check that sender is a valid replica
   _, peerIDs, err := h.GetNetworkHandles()
   if err != nil {
      return fmt.Errorf("Could not verify message from %v : %v", replicaID.Name, err)
   }

   for _, peerID := range peerIDs {
      if peerID.Name == replicaID.Name {
         // if it's a valid peer, let crypto do its function
         cryptoID := peerID.PkiID ;
         return h.secHelper.Verify(cryptoID, signature, message)
      }
   }
   return fmt.Errorf("Could not verify message from %s. Unknown peer.", replicaID.Name)
}

// GetNetworkHandles returns the peer handles of the current validator and VP network
func (h *Helper) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	ep, err := h.coordinator.GetPeerEndpoint()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve own endpoint: %v", err)
	}
	self = ep.ID

	peersMsg, err := h.coordinator.GetPeers()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve list of peers: %v", err)
	}
	peers := peersMsg.GetPeers()
	for _, endpoint := range peers {
		if endpoint.Type == pb.PeerEndpoint_VALIDATOR {
			network = append(network, endpoint.ID)
		}
	}
	network = append(network, self)

	return
}

// Broadcast sends a message to all validating peers
func (h *Helper) Broadcast(msg *pb.OpenchainMessage) error {
	errors := h.coordinator.Broadcast(msg)
	if len(errors) > 0 {
		return fmt.Errorf("Couldn't broadcast successfully")
	}
	return nil
}

// Unicast sends a message to a specified receiver
func (h *Helper) Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error {
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
	// The secHelper is set during creat ChaincodeSupport, so we don't need this step
	// cxt := context.WithValue(context.Background(), "security", h.coordinator.GetSecHelper())
	return chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)
}

// CommitTxBatch gets invoked when the current transaction-batch needs
// to be committed. This function returns successfully iff the
// transactions details and state changes (that may have happened
// during execution of this transaction-batch) have been committed to
// permanent storage.
func (h *Helper) CommitTxBatch(id interface{}, transactions []*pb.Transaction, transactionsResults []*pb.TransactionResult, metadata []byte) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.CommitTxBatch(id, transactions, nil, metadata); err != nil {
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

// PreviewCommitTxBatchBlock ...
func (h *Helper) PreviewCommitTxBatchBlock(id interface{}, txs []*pb.Transaction, metadata []byte) (*pb.Block, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if block, err := ledger.GetTXBatchPreviewBlock(id, txs, metadata); err != nil {
		return nil, fmt.Errorf("Failed to commit transaction to the ledger: %v", err)
	} else {
		return block, err
	}
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

// HashBlock returns the hash of the included block, useful for mocking
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

// ApplyStateDelta ....
// TODO, waiting to see the streaming implementation to define this API nicely
func (h *Helper) ApplyStateDelta(delta []byte, unapply bool) error {
	return nil // TODO implement
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

func (h *Helper) getRemoteLedger(replicaID uint64) (peer.RemoteLedger, error) {
	// TODO adding this so that the code compiles without errors
	var err error
	receiverHandle := &pb.PeerID{}
	// receiverHandle, err := h.GetReplicaHandle(replicaID)

	if nil != err {
		return nil, fmt.Errorf("Error retrieving handle for given replicaID %d : %s", replicaID, err)
	}

	remoteLedger, err := h.coordinator.GetRemoteLedger(receiverHandle)
	if nil != err {
		return nil, fmt.Errorf("Error retrieving the remote ledger for the given handle '%s' : %s", receiverHandle, err)
	}

	return remoteLedger, nil
}

// GetRemoteBlocks will return a channel to stream blocks from the desired replicaID
func (h *Helper) GetRemoteBlocks(replicaID uint64, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
	remoteLedger, err := h.getRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestBlocks(&pb.SyncBlockRange{
		Start: start,
		End:   finish,
	})
}

// GetRemoteStateSnapshot will return a channel to stream a state snapshot from the desired replicaID
func (h *Helper) GetRemoteStateSnapshot(replicaID uint64) (<-chan *pb.SyncStateSnapshot, error) {
	remoteLedger, err := h.getRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateSnapshot()
}

// GetRemoteStateDeltas will return a channel to stream a state snapshot deltas from the desired replicaID
func (h *Helper) GetRemoteStateDeltas(replicaID uint64, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	remoteLedger, err := h.getRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateDeltas(&pb.SyncBlockRange{
		Start: start,
		End:   finish,
	})
}
