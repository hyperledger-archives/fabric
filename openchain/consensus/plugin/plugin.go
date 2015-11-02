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

package plugin

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/looplab/fsm"
)

// Package-level logger.
var Logger *logging.Logger

func init() {
	Logger = logging.MustGetLogger("plugin")
}

// =============================================================================
// Structure definitions go here.
// =============================================================================

// This structure should implement the `LayerMember` interface (defined below).
type layerMember struct {
	leader bool
}

// =============================================================================
// Interface definitions go here.
// =============================================================================

// Layer is the interface that the plugin's `layerMember` should implement. As
// the name implies, this structure is a member of the `layer` of the validating
// peer.
type Layer interface {
	isLeader() bool
	setLeader(flag bool) bool
}

// =============================================================================
// Interface implementations go here.
// =============================================================================

// IsLeader allows to check whether a validating peer is the current leader.
func (lm *layerMember) IsLeader() {

	return leader
}

// SetLeader flags a validating peer as leader. This is a temporary state.
func (lm *layerMember) SetLeader(flag bool) bool {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Setting the leader flag.")
	}

	layer.leader = flag

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Leader flag set.")
	}

	return layer.leader
}

// =============================================================================
// Constructors go here.
// =============================================================================

// NewLayerMember creates the plugin-specific fields/structures that related to
// the validator (not the stream).
func NewLayerMember() Layer {
	return &layerMember{leader: false}
}

// NewReceiverMember creates the plugin-specific fields/structures that relate
// to the stream. For example, data stores for a stream's messages belong here.
func NewReceiverMember() interface{} {
	msgStore := make(map[string]*Unpack)
	return msgStore
}

// NewFSM defines the possible states of the algorithm, the allowed transitions,
// and the callbacks that should be executed upon each transition. TODO: Replace
// with actual FSM. Remember that this is the FSM that sits on the receiving
// side of a stream.
func NewFSM() (f *fsm.FSM) {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Creating a new FSM.")
	}

	f = fsm.NewFSM(
		"closed",
		fsm.Events{
			{Name: "open", Src: []string{"closed"}, Dst: "open"},
			{Name: "close", Src: []string{"open"}, Dst: "closed"},
		},
		fsm.Callbacks{},
	)

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("FSM created.")
	}

	return

}

// =============================================================================
// The message handler goes here.
// =============================================================================

// HandleMessage allows a `streamReceiver` to process a message that is
// incoming to this stream. This is where you inspect the incoming message,
// unmarshal if needed, and trigger events on the `streamReceiver`'s FSM.
func HandleMessage(receiver *consensus.streamReceiver, msg *pb.OpenchainMessage) error {

	if msg.Type == pb.OpenchainMessage_CONSENSUS {

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Unpacking CONSENSUS message in plugin's message handler.")
		}

		// Unpack to the common message template.
		u := &Unpack{}

		err := proto.Unmarshal(msg.Payload, u)
		if err != nil {
			return fmt.Errorf("Error unpacking payload from message: %s", err)
		}

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Message unpacked.")
		}

		if receiver.fsm.Cannot(u.Type.String()) {
			return fmt.Errorf("Receiver's FSM cannot handle message type %s while in state %s.", u.Type.String(), receiver.fsm.Current())
		}

		// If the message type is allowed in that state, trigger the respective event in the FSM.
		err = receiver.fsm.Event(u.Type.String(), u)

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Processed message of type %s, current state is: %s", u.Type, receiver.fsm.Current())
		}

	} else {

		if receiver.fsm.Cannot(msg.Type.String()) {
			return fmt.Errorf("Receiver's FSM cannot handle message type %s while in state %s.", msg.Type.String(), receiver.fsm.Current())
		}

		// Attempt to trigger an event based on the incoming message's type.
		err := receiver.Event(msg.Type.String(), msg)
	}

	return consensus.filterError(err)
}

// =============================================================================
// Everything else goes here.
// =============================================================================
