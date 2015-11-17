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
	"time"

	"github.com/op/go-logging"
	"golang.org/x/net/context"
	gp "google/protobuf"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
)

// =============================================================================
// Init.
// =============================================================================

// Package-level logger.
var helperLogger *logging.Logger

func init() {

	helperLogger = logging.MustGetLogger("helper")
}

// =============================================================================
// Structure definitions go here.
// =============================================================================

// Helper data structure.
type Helper struct {
	coordinator MessageHandlerCoordinator
}

// =============================================================================
// Constructs go here.
// =============================================================================

// NewHelper constructs the consensus helper object.
func NewHelper(mhc MessageHandlerCoordinator) consensus.CPI {
	return &Helper{coordinator: mhc}
}

// =============================================================================
// Interface implementations go here.
// =============================================================================

// Broadcast sends a message to all validating peers. during a consensus round.
// The argument is the serialized message that is specific to the consensus
// algorithm implementation. It is wrapped into a CONSENSUS message.
func (h *Helper) Broadcast(msgPayload []byte) error {

	if helperLogger.IsEnabledFor(logging.DEBUG) {
		helperLogger.Debug("Broadcasting a message.")
	}

	// Wrap as message of type OpenchainMessage_CONSENSUS.
	msgTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	msg := &pb.OpenchainMessage{
		Type:      pb.OpenchainMessage_CONSENSUS,
		Timestamp: msgTime,
		Payload:   msgPayload,
	}

	return h.coordinator.Broadcast(msg)
}

// Unicast is called by the validating peer to send a CONSENSUS message to a
// specified receiver. As is the case with `Broadcast()`, the argument is the
// serialized payload of an implementation-specific message.
func (h *Helper) Unicast(msgPayload []byte, receiver string) error {

	// Wrap as message of type OpenchainMessage_CONSENSUS.
	msgTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	msg := &pb.OpenchainMessage{
		Type:      pb.OpenchainMessage_CONSENSUS,
		Timestamp: msgTime,
		Payload:   msgPayload,
	}

	// TODO: Call a function in the comms layer.
	// Waiting for Jeff's implementation.
	var _ = msg // Just to silence the compiler error.

	return nil
}

// ExecTXs will execute all the transactions listed in the `txs` array
// one-by-one. If all the executions are successful, it returns the candidate
// global state hash, and nil error array.
func (h *Helper) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {

	if helperLogger.IsEnabledFor(logging.DEBUG) {
		helperLogger.Debug("Executing the transactions.")
	}
	return chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)

}
