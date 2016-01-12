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

	"github.com/spf13/viper"
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

// GetReplicaHash returns the crypto IDs of the current replica and the whole network
// TODO func (h *Helper) GetReplicaHash() (self []byte, network [][]byte, err error) {
// ATTN: Until the crypto package is integrated, this functions returns
// the <IP:port>s of the current replica and the whole network instead
func (h *Helper) GetReplicaHash() (self string, network []string, err error) {
	// v, _ := h.coordinator.GetValidator()
	// self = v.GetID()
	peer, err := peer.GetPeerEndpoint()
	if err != nil {
		return "", nil, err
	}
	self = peer.Address

	config := viper.New()
	config.SetConfigName("openchain")
	config.AddConfigPath("./")
	err = config.ReadInConfig()
	if err != nil {
		err = fmt.Errorf("Fatal error reading root config: %s", err)
		return self, nil, err
	}

	// encodedHashes := config.GetStringSlice("peer.validator.replicas.hashes")
	/* network = make([][]byte, len(encodedHashes))
	for i, v := range encodedHashes {
		network[i], _ = base64.StdEncoding.DecodeString(v)
	} */
	network = config.GetStringSlice("peer.validator.replicas.ips")

	return self, network, nil
}

// GetReplicaID returns the uint handle corresponding to a replica address
// TODO func (h *Helper) GetReplicaID(hash []byte) (id uint64, err error) {
func (h *Helper) GetReplicaID(addr string) (id uint64, err error) {
	_, network, err := h.GetReplicaHash()
	if err != nil {
		return uint64(0), err
	}
	for i, v := range network {
		// if bytes.Equal(v, hash) {
		if v == addr {
			return uint64(i), nil
		}
	}

	//err = fmt.Errorf("Couldn't find crypto ID in list of VP IDs given in config")
	err = fmt.Errorf("Couldn't find IP:port in list of VP addresses given in config")
	return uint64(0), err
}

// Broadcast sends a message to all validating peers.
func (h *Helper) Broadcast(msg *pb.OpenchainMessage) error {
	_ = h.coordinator.Broadcast(msg) // TODO process the errors
	return nil
}

// Unicast sends a message to a specified receiver.
func (h *Helper) Unicast(msgPayload []byte, receiver string) error {
	// TODO Call a function in the comms layer; wait for Jeff's implementation.
	return nil
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
func (h *Helper) CommitTxBatch(id interface{}, transactions []*pb.Transaction, proof []byte) error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Failed to get the ledger: %v", err)
	}
	if err := ledger.CommitTxBatch(id, transactions, proof); err != nil {
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
