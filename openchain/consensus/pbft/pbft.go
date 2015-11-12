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

package pbft

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// =============================================================================
// Constants
// =============================================================================
const configPrefix = "OPENCHAIN_PBFT"

// =============================================================================
// Init.
// =============================================================================

// Package-level logger.
var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("plugin")
}

// =============================================================================
// Custom structure definitions go here.
// =============================================================================

// Plugin carries fields related to the consensus algorithm.
type Plugin struct {
	cpi      consensus.CPI      // The consensus programming interface
	config   *viper.Viper       // The link to the config file
	fsm      *fsm.FSM           // Holds the finite state machine.
	leader   bool               // Is this validating peer the current leader?
	msgStore map[string]*Unpack // Where we store incoming `REQUEST` messages.
}

// =============================================================================
// Custom interface definitions go here.
// =============================================================================

type validator interface {
	getParam(param string) (val string, err error)
	isLeader() bool
	setLeader(flag bool) bool
}

// =============================================================================
// Constructors go here.
// =============================================================================

// New creates an implementation-specific structure that will be held in the
// consensus `helper` object. (See `controller` and `helper` packages for more.)
func New(c consensus.CPI) *Plugin {

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Creating the consenter.")
	}
	instance := &Plugin{}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Setting the consenter's CPI.")
	}
	instance.cpi = c

	// TODO: Initialize the algorithm here.
	// You may want to set the fields of `instance` using `instance.GetParam()`.
	// e.g. instance.blockTimeOut = strconv.Atoi(instance.getParam("timeout.block"))

	// Create a link to the config file.
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Linking to the consenter's config file.")
	}
	instance.config = viper.New()

	// For environment variables.
	instance.config.SetEnvPrefix(configPrefix)
	instance.config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	instance.config.SetEnvKeyReplacer(replacer)

	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	instance.config.AddConfigPath("../pbft/") // For when you run a test from `controller`.
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// Create the FSM.
	instance.fsm = newFSM()

	// Create the data store for incoming messages.
	instance.msgStore = make(map[string]*Unpack)

	return instance
}

// newFSM defines the possible states of the algorithm, the allowed transitions,
// and the callbacks that should be executed upon each transition. TODO: Replace
// with actual FSM.
func newFSM() (f *fsm.FSM) {

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Creating a new FSM.")
	}

	f = fsm.NewFSM(
		"closed",
		fsm.Events{
			{Name: "open", Src: []string{"closed"}, Dst: "open"},
			{Name: "close", Src: []string{"open"}, Dst: "closed"},
		},
		fsm.Callbacks{},
	)

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("FSM created.")
	}

	return

}

// =============================================================================
// Consenter interface implementation goes here.
// =============================================================================

// RecvMsg allows the algorithm to receive and process a message. The message
// that reaches here is either `OpenchainMessage_REQUEST` or
// `OpenchainMessage_CONSENSUS`.
func (instance *Plugin) RecvMsg(msg *pb.OpenchainMessage) error {

	// Declare so that you can filter it later if need be.
	var err error

	if logger.IsEnabledFor(logging.INFO) {
		logger.Info("OpenchainMessage:%s received.", msg.Type)
	}

	if msg.Type == pb.OpenchainMessage_REQUEST {

		// Unmarshal `msg.Payload`.
		txBatch := &pb.TransactionBlock{}
		err = proto.Unmarshal(msg.Payload, txBatch)
		if err != nil {
			return fmt.Errorf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type)
		}

		numTx := len(txBatch.Transactions)

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Unmarshaled payload, number of transactions it carries: %d", numTx)
		}

		// Extract transaction.
		if numTx != 1 {
			return fmt.Errorf("OpenchainMessage:%s should carry 1 transaction instead of: %d", msg.Type, numTx)
		}

		tx := txBatch.Transactions[0]

		// Marshal transaction.
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			return fmt.Errorf("Error marshalling single transaction.")
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Marshaled single transaction.")
		}

		// Create new `Unpack_REQUEST2` message. TODO: Rename to `REQUEST`.

		reqMsg := &Request2{
			Timestamp: tx.Timestamp,
			Payload:   txPacked,
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Created REQUEST message.")
		}

		// Marshal this.
		reqMsgPacked, err := proto.Marshal(reqMsg)
		if err != nil {
			return fmt.Errorf("Error marshalling REQUEST message.")
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Marshalled REQUEST message.")
		}

		// Create new `Unpack` message.
		unpackMsg := &Unpack{
			Type:    Unpack_REQUEST,
			Payload: reqMsgPacked,
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Created Unpack:%s message.", unpackMsg.Type)
		}

		// Serialize it.
		newPayload, err := proto.Marshal(unpackMsg)
		if err != nil {
			return fmt.Errorf("Error marshalling Unpack:%s message.", unpackMsg.Type)
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Marshalled Unpack:%s message.", unpackMsg.Type)
		}

		// Broadcast this message to all the validating peers.
		return instance.cpi.Broadcast(newPayload)
	}

	// TODO: Message that reached here is `OpenchainMessage_CONSENSUS`.
	// Process it accordingly. You most likely want to pass it to
	// `instance.fsm`.

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Unpacking message.")
	}

	// Unpack to the common message template.
	extractedMsg := &Unpack{}

	err = proto.Unmarshal(msg.Payload, extractedMsg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Message unpacked.")
	}

	/* if instance.fsm.Cannot(extractedMsg.Type.String()) {
		return fmt.Errorf("FSM cannot handle message type %s while in state: %s", extractedMsg.Type.String(), instance.fsm.Current())
	}

	// If the message type is allowed in that state, trigger the respective event in the FSM.
	err = instance.fsm.Event(extractedMsg.Type.String(), extractedMsg)

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Processed message of type %s, current state is: %s", extractedMsg.Type, instance.fsm.Current())
	}

	return filterError(err) */

	return nil
}

// =============================================================================
// Custom interface implementation goes here.
// =============================================================================

// getParam is a getter for the values listed in `config.yaml`.
func (instance *Plugin) getParam(param string) (val string, err error) {
	if ok := instance.config.IsSet(param); !ok {
		err := fmt.Errorf("Key %s does not exist in algo config", param)
		return "nil", err
	}
	val = instance.config.GetString(param)
	return val, nil
}

// isLeader allows us to check whether a validating peer is the current leader.
func (instance *Plugin) isLeader() bool {

	return instance.leader
}

// setLeader flags a validating peer as the leader. This is a temporary state.
func (instance *Plugin) setLeader(flag bool) bool {

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Setting the leader flag.")
	}

	instance.leader = flag

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Leader flag set.")
	}

	return instance.leader
}

// =============================================================================
// Misc. helper functions go here.
// =============================================================================

// filterError filters the FSM errors to allow NoTransitionError and
// CanceledError to not propogate for cases where embedded err == nil.
// This should be called by the plugin developer whenever FSM state transitions
// are attempted.
func filterError(fsmError error) error {

	if fsmError != nil {

		if noTransitionErr, ok := fsmError.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Only allow `NoTransitionError` errors, all others are considered true error.
				return fsmError
			}
			logger.Debug("Ignoring NoTransitionError: %s", noTransitionErr)
		}
		if canceledErr, ok := fsmError.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return canceledErr
			}
			logger.Debug("Ignoring CanceledError: %s", canceledErr)
		}
	}

	return nil
}
