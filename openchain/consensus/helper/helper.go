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

// GetReplicaAddress returns the IP:Port for the current replica (self:true) or the whole network (self:false).
// Will be eventually return crypto IDs.
func (h *Helper) GetReplicaAddress(self bool) (addresses []string, err error) {
	if self {
		pe, err := peer.GetPeerEndpoint()
		if err != nil {
			err = fmt.Errorf("Couldn't get own peer endpoint: %s", err)
			return nil, err
		}
		addresses = append(addresses, pe.Address)
	} else {
		config := viper.New()
		config.SetConfigName("openchain")
		config.AddConfigPath("./")
		err = config.ReadInConfig()
		if err != nil {
			err = fmt.Errorf("Fatal error reading root config: %s", err)
			return nil, err
		}
		addresses = config.GetStringSlice("peer.validator.replicas")
	}
	return addresses, nil
}

// GetReplicaID returns the uint handle corresponding to a replica address.
func (h *Helper) GetReplicaID(address string) (id uint64, err error) {
	addresses, err := h.GetReplicaAddress(false)
	if err != nil {
		return uint64(0), err
	}
	for i, v := range addresses {
		if v == address {
			return uint64(i), nil
		}
	}
	err = fmt.Errorf("Couldn't find address in list of VP addresses given in config")
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

// ExecTXs executes all the transactions listed in the txs array one-by-one.
// If all the executions are successful, it returns the candidate global state hash, and nil error array.
func (h *Helper) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	return chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs, h.coordinator.GetSecHelper())
}
