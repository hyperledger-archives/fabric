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

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/peer"
	pb "github.com/openblockchain/obc-peer/protos"
)

// =============================================================================
// Init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/helper")
}

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

// GetReplicas returns the IP:Ports for all replicas in the network.
// The replica handles are the indexes of the return slice.
func (h *Helper) GetReplicas() (replicas []string, err error) {
	config := viper.New()
	config.SetConfigName("openchain")
	config.AddConfigPath("./")
	err = config.ReadInConfig()
	if err != nil {
		err = fmt.Errorf("Fatal error reading root config: %s", err)
		return nil, err
	}
	replicas = config.GetStringSlice("peer.validator.replicas")
	return replicas, nil
}

// GetReplicaID returns our own replica handle.
func (h *Helper) GetReplicaID() (id uint64, err error) {
	replicas, err := h.GetReplicas()
	if err != nil {
		return uint64(0), err
	}
	pe, _ := peer.GetPeerEndpoint()
	for i, v := range replicas {
		if v == pe.Address {
			fmt.Printf("\nID: %v\n", i)
			return uint64(i), nil
		}
	}
	err = fmt.Errorf("Couldn't find own IP in list of VP IDs given in config")
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
	return chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)
}
