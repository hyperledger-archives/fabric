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

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/consensus/helper/persist"
	"github.com/hyperledger/fabric/consensus/statetransfer"
	"github.com/hyperledger/fabric/core/chaincode"
	crypto "github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/statemgmt"
	"github.com/hyperledger/fabric/core/peer"
	pb "github.com/hyperledger/fabric/protos"
)

// Helper contains the reference to the peer's MessageHandlerCoordinator
type Helper struct {
	consenter   consensus.Consenter
	coordinator peer.MessageHandlerCoordinator
	secOn       bool
	secHelper   crypto.Peer
	curBatch    []*pb.Transaction // TODO, remove after issue 579
	persist.PersistHelper

	sts *statetransfer.StateTransferState
}

// NewHelper constructs the consensus helper object
func NewHelper(mhc peer.MessageHandlerCoordinator) *Helper {
	h := &Helper{
		coordinator: mhc,
		secOn:       viper.GetBool("security.enabled"),
		secHelper:   mhc.GetSecHelper(),
	}
	h.sts = statetransfer.NewStateTransferState(h)
	h.sts.Initiate(nil)
	h.sts.RegisterListener(h)
	return h
}

func (h *Helper) setConsenter(c consensus.Consenter) {
	h.consenter = c
}

// GetNetworkInfo returns the PeerEndpoints of the current validator and the entire validating network
func (h *Helper) GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
	ep, err := h.coordinator.GetPeerEndpoint()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve own endpoint: %v", err)
	}
	self = ep

	peersMsg, err := h.coordinator.GetPeers()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve list of peers: %v", err)
	}
	peers := peersMsg.GetPeers()
	for _, endpoint := range peers {
		if endpoint.Type == pb.PeerEndpoint_VALIDATOR {
			network = append(network, endpoint)
		}
	}
	network = append(network, self)

	return
}

// GetNetworkHandles returns the PeerIDs of the current validator and the entire validating network
func (h *Helper) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	selfEP, networkEP, err := h.GetNetworkInfo()
	if err != nil {
		return self, network, fmt.Errorf("Couldn't retrieve validating network's endpoints: %v", err)
	}

	self = selfEP.ID

	for _, endpoint := range networkEP {
		network = append(network, endpoint.ID)
	}
	network = append(network, self)

	return
}

// Broadcast sends a message to all validating peers
func (h *Helper) Broadcast(msg *pb.Message, peerType pb.PeerEndpoint_Type) error {
	errors := h.coordinator.Broadcast(msg, peerType)
	if len(errors) > 0 {
		return fmt.Errorf("Couldn't broadcast successfully")
	}
	return nil
}

// Unicast sends a message to a specified receiver
func (h *Helper) Unicast(msg *pb.Message, receiverHandle *pb.PeerID) error {
	return h.coordinator.Unicast(msg, receiverHandle)
}

// Sign a message with this validator's signing key
func (h *Helper) Sign(msg []byte) ([]byte, error) {
	if h.secOn {
		return h.secHelper.Sign(msg)
	}
	logger.Debug("Security is disabled")
	return msg, nil
}

// Verify that the given signature is valid under the given replicaID's verification key
// If replicaID is nil, use this validator's verification key
// If the signature is valid, the function should return nil
func (h *Helper) Verify(replicaID *pb.PeerID, signature []byte, message []byte) error {
	if !h.secOn {
		logger.Debug("Security is disabled")
		return nil
	}

	logger.Debug("Verify message from: %v", replicaID.Name)
	_, network, err := h.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Couldn't retrieve validating network's endpoints: %v", err)
	}

	// check that the sender is a valid replica
	// if so, call crypto verify() with that endpoint's pkiID
	for _, endpoint := range network {
		logger.Debug("Endpoint name: %v", endpoint.ID.Name)
		if *replicaID == *endpoint.ID {
			cryptoID := endpoint.PkiID
			return h.secHelper.Verify(cryptoID, signature, message)
		}
	}
	return fmt.Errorf("Could not verify message from %s (unknown peer)", replicaID.Name)
}

// BeginTxBatch gets invoked when the next round
// of transaction-batch execution begins
func (h *Helper) BeginTxBatch(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.BeginTxBatch(id); err != nil {
		return fmt.Errorf("Failed to begin transaction with the ledger: %v", err)
	}
	h.curBatch = nil // TODO, remove after issue 579
	return nil
}

// ExecTxs executes all the transactions listed in the txs array
// one-by-one. If all the executions are successful, it returns
// the candidate global state hash, and nil error array.
func (h *Helper) ExecTxs(id interface{}, txs []*pb.Transaction) ([]byte, error) {
	// TODO id is currently ignored, fix once the underlying implementation accepts id

	// The secHelper is set during creat ChaincodeSupport, so we don't need this step
	// cxt := context.WithValue(context.Background(), "security", h.coordinator.GetSecHelper())
	// TODO return directly once underlying implementation no longer returns []error
	res, _ := chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)
	h.curBatch = append(h.curBatch, txs...) // TODO, remove after issue 579
	return res, nil
}

// CommitTxBatch gets invoked when the current transaction-batch needs
// to be committed. This function returns successfully iff the
// transactions details and state changes (that may have happened
// during execution of this transaction-batch) have been committed to
// permanent storage.
func (h *Helper) CommitTxBatch(id interface{}, metadata []byte) (*pb.Block, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger: %v", err)
	}
	// TODO fix this one the ledger has been fixed to implement
	if err := ledger.CommitTxBatch(id, h.curBatch, nil, metadata); err != nil {
		return nil, fmt.Errorf("Failed to commit transaction to the ledger: %v", err)
	}

	size := ledger.GetBlockchainSize()
	h.curBatch = nil // TODO, remove after issue 579

	block, err := ledger.GetBlockByNumber(size - 1)
	if err != nil {
		return nil, fmt.Errorf("Failed to get the block at the head of the chain: %v", err)
	}

	return block, nil
}

// RollbackTxBatch discards all the state changes that may have taken
// place during the execution of current transaction-batch
func (h *Helper) RollbackTxBatch(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.RollbackTxBatch(id); err != nil {
		return fmt.Errorf("Failed to rollback transaction with the ledger: %v", err)
	}
	h.curBatch = nil // TODO, remove after issue 579
	return nil
}

// PreviewCommitTxBatch retrieves a preview of the block info blob (as
// returned by GetBlockchainInfoBlob) that would describe the
// blockchain if CommitTxBatch were invoked.  The blockinfo will
// change if additional ExecTXs calls are invoked.
func (h *Helper) PreviewCommitTxBatch(id interface{}, metadata []byte) ([]byte, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger: %v", err)
	}
	// TODO fix this once the underlying API is fixed
	blockInfo, err := ledger.GetTXBatchPreviewBlockInfo(id, h.curBatch, metadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to preview commit: %v", err)
	}
	rawInfo, _ := proto.Marshal(blockInfo)
	return rawInfo, nil
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

// ApplyStateDelta applies a state delta to the current state
// The result of this function can be retrieved using GetCurrentStateDelta
// To commit the result, call CommitStateDelta, or to roll it back
// call RollbackStateDelta
func (h *Helper) ApplyStateDelta(id interface{}, delta *statemgmt.StateDelta) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.ApplyStateDelta(id, delta)
}

// CommitStateDelta makes the result of ApplyStateDelta permanent
// and releases the resources necessary to rollback the delta
func (h *Helper) CommitStateDelta(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.CommitStateDelta(id)
}

// RollbackStateDelta undoes the results of ApplyStateDelta to revert
// the current state back to the state before ApplyStateDelta was invoked
func (h *Helper) RollbackStateDelta(id interface{}) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger :%v", err)
	}
	return ledger.RollbackStateDelta(id)
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

func (h *Helper) getRemoteLedger(peerID *pb.PeerID) (peer.RemoteLedger, error) {
	remoteLedger, err := h.coordinator.GetRemoteLedger(peerID)
	if nil != err {
		return nil, fmt.Errorf("Error retrieving the remote ledger for the given handle '%s' : %s", peerID, err)
	}

	return remoteLedger, nil
}

// GetRemoteBlocks will return a channel to stream blocks from the desired replicaID
func (h *Helper) GetRemoteBlocks(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncBlocks, error) {
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
func (h *Helper) GetRemoteStateSnapshot(replicaID *pb.PeerID) (<-chan *pb.SyncStateSnapshot, error) {
	remoteLedger, err := h.getRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateSnapshot()
}

// GetRemoteStateDeltas will return a channel to stream a state snapshot deltas from the desired replicaID
func (h *Helper) GetRemoteStateDeltas(replicaID *pb.PeerID, start, finish uint64) (<-chan *pb.SyncStateDeltas, error) {
	remoteLedger, err := h.getRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateDeltas(&pb.SyncBlockRange{
		Start: start,
		End:   finish,
	})
}

func (h *Helper) GetBlockchainInfoBlob() []byte {
	ledger, _ := ledger.GetLedger()
	info, _ := ledger.GetBlockchainInfo()
	rawInfo, _ := proto.Marshal(info)
	return rawInfo
}

func (h *Helper) GetBlockHeadMetadata() ([]byte, error) {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, err
	}
	head := ledger.GetBlockchainSize()
	block, err := ledger.GetBlockByNumber(head - 1)
	if err != nil {
		return nil, err
	}
	return block.ConsensusMetadata, nil
}

func (h *Helper) SkipTo(tag uint64, id []byte, peers []*pb.PeerID) {
	info := &pb.BlockchainInfo{}
	proto.Unmarshal(id, info)
	h.sts.AddTarget(info.Height-1, info.CurrentBlockHash, peers, tag)
}

func (h *Helper) Initiated() {
}

func (h *Helper) Completed(bn uint64, bh []byte, pids []*pb.PeerID, m interface{}) {
	h.consenter.StateUpdate(m.(uint64), bh)
}

func (h *Helper) Errored(bn uint64, bh []byte, pids []*pb.PeerID, m interface{}, e error) {
	if seqNo, ok := m.(uint64); !ok {
		logger.Warning("state transfer reported error for block %d, seqNo %d: %s", bn, seqNo, e)
	} else {
		logger.Warning("state transfer reported error for block %d, %s", bn, e)
	}
}
