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

	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
)

// Package-level logger.
var Logger *logging.Logger

func init() {
	Logger = logging.MustGetLogger("plugin")
}

// =============================================================================
// Structure definitions go here.
// =============================================================================

// This structure should implement the `ConsensusLayer` interface (defined
// below). This is where you define peer-related variables.
type layerMember struct {
	leader bool
}

// This structure should implement the `StreamReceiver` interface (defined
// below). This is where you define stream-related variables.
type receiverMember struct {
	fsm      *fsm.FSM
	msgStore map[string]*Unpack
}

// =============================================================================
// Interface definitions go here.
// =============================================================================

// ConsensusLayer is the interface that the plugin's `layerMember` should
// implement. As the name implies, this structure is a member of the
// `consensusLayer` of the validating peer.
type ConsensusLayer interface {
	isLeader() bool
	setLeader(flag bool) bool
}

// StreamReceiver is the interface that the plugin's `receiverMember` should
// implement. As the name implies, this structure is a member of the
// `streamReceiver` of the validating peer's `consensusLayer`
type StreamReceiver interface {
	getFSM() *fsm.FSM
	getMsgStore() map[string]*Unpack
}

// =============================================================================
// Interface implementations go here.
// =============================================================================

// isLeader allows to check whether a validating peer is the current leader.
func (lm *layerMember) isLeader() bool {

	return lm.leader
}

// setLeader flags a validating peer as leader. This is a temporary state.
func (lm *layerMember) setLeader(flag bool) bool {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Setting the leader flag.")
	}

	lm.leader = flag

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Leader flag set.")
	}

	return lm.leader
}

// getFSM returns a `streamReceiver`'s FSM.
func (rm *receiverMember) getFSM() *fsm.FSM {

	return rm.fsm
}

// getMsgStore returns a `streamReceiver`'s message store for incoming messages.
func (rm *receiverMember) getMsgStore() map[string]*Unpack {

	return rm.msgStore
}

// =============================================================================
// Constructors go here.
// =============================================================================

// NewLayerMember creates the plugin-specific fields/structures that related to
// the validator (not the stream).
func NewLayerMember() ConsensusLayer {

	return &layerMember{leader: false}
}

// NewReceiverMember creates the plugin-specific fields/structures that relate
// to the stream. For example, the FSM and data stores for a stream's messages
// belong here.
func NewReceiverMember() StreamReceiver {

	rm := receiverMember{
		fsm:      newFSM(),
		msgStore: make(map[string]*Unpack),
	}

	return &rm
}

// newFSM defines the possible states of the algorithm, the allowed transitions,
// and the callbacks that should be executed upon each transition. TODO: Replace
// with actual FSM. Remember that this is the FSM that sits on the receiving
// side of a stream.
func newFSM() (f *fsm.FSM) {

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
func HandleMessage(receiver StreamReceiver, msg *pb.OpenchainMessage) error {

	// Declare so that you can filter it later.
	var err error

	// Get reference to receiver's FSM.
	fsm := receiver.getFSM()

	if msg.Type == pb.OpenchainMessage_CONSENSUS {

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Unpacking CONSENSUS message in plugin's message handler.")
		}

		// Unpack to the common message template.
		u := &Unpack{}

		err = proto.Unmarshal(msg.Payload, u)
		if err != nil {
			return fmt.Errorf("Error unpacking payload from message: %s", err)
		}

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Message unpacked.")
		}

		if fsm.Cannot(u.Type.String()) {
			return fmt.Errorf("Receiver's FSM cannot handle message type %s while in state %s.", u.Type.String(), fsm.Current())
		}

		// If the message type is allowed in that state, trigger the respective event in the FSM.
		err = fsm.Event(u.Type.String(), u)

		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Processed message of type %s, current state is: %s", u.Type, fsm.Current())
		}

	} else {

		if fsm.Cannot(msg.Type.String()) {
			return fmt.Errorf("Receiver's FSM cannot handle message type %s while in state %s.", msg.Type.String(), fsm.Current())
		}

		// Attempt to trigger an event based on the incoming message's type.
		err = fsm.Event(msg.Type.String(), msg)
	}

	return filterError(err)
}

// =============================================================================
// Everything else goes here.
// =============================================================================

// filterError filters the FSM errors to allow NoTransitionError and
// CanceledError to not propogate for cases where embedded err == nil.
// This should be called by the plugin developer whenever FSM state transitions
// are attempted. TODO: Maybe move to the `utils` package.
func filterError(fsmError error) error {

	if fsmError != nil {

		if noTransitionErr, ok := fsmError.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Only allow `NoTransitionError` errors, all others are considered true error.
				return fsmError
			}
			Logger.Debug("Ignoring NoTransitionError: %s", noTransitionErr)
		}
		if canceledErr, ok := fsmError.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return canceledErr
			}
			Logger.Debug("Ignoring CanceledError: %s", canceledErr)
		}
	}

	return nil
}
