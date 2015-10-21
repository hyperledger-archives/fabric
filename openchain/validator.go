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

package openchain

import (
	"fmt"

	"github.com/looplab/fsm"
	"github.com/op/go-logging"

	pb "github.com/openblockchain/obc-peer/protos"
)

var validatorLogger = logging.MustGetLogger("validator")

type Validator interface {
	Broadcast(*pb.OpenchainMessage) error
	GetHandler(stream PeerChatStream) MessageHandler
}

type SimpleValidator struct {
	validatorStreams map[string]MessageHandler
	peerStreams      map[string]MessageHandler
}

func (v *SimpleValidator) Broadcast(*pb.OpenchainMessage) error {
	return nil
}

func (v *SimpleValidator) GetHandler(stream PeerChatStream) MessageHandler {
	return NewValidatorFSM(v, "", stream)
}

func NewSimpleValidator() Validator {
	validator := &SimpleValidator{}
	return validator
}

type ValidatorFSM struct {
	To         string
	ChatStream PeerChatStream
	FSM        *fsm.FSM
	PeerFSM    *PeerFSM
	validator  Validator
}

func NewValidatorFSM(parent Validator, to string, peerChatStream PeerChatStream) *ValidatorFSM {
	v := &ValidatorFSM{
		To:         to,
		ChatStream: peerChatStream,
		validator:  parent,
	}

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
		},
		fsm.Callbacks{
			"enter_state":                                               func(e *fsm.Event) { v.enterState(e) },
			"before_" + pb.OpenchainMessage_DISC_HELLO.String():         func(e *fsm.Event) { v.beforeHello(e) },
			"before_" + pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(): func(e *fsm.Event) { v.beforeChainTransactions(e) },
		},
	)

	return v
}

func (v *ValidatorFSM) enterState(e *fsm.Event) {
	validatorLogger.Debug("The Validators's bi-directional stream to %s is %s, from event %s\n", v.To, e.Dst, e.Event)
}

func (v *ValidatorFSM) beforeHello(e *fsm.Event) {
	validatorLogger.Debug("Sending back %s", pb.OpenchainMessage_DISC_HELLO.String())
	if err := v.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
		e.Cancel(err)
	}
}

func (v *ValidatorFSM) beforeChainTransactions(e *fsm.Event) {
	validatorLogger.Debug("Sending broadcast to all validators upon receipt of %s", pb.OpenchainMessage_DISC_HELLO.String())
	// v.validator.Broadcast(pb.OpenchainMessage_VALIDATOR_TRANSACTIONS)
	if err := v.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
		e.Cancel(err)
	}
}

func (v *ValidatorFSM) when(stateToCheck string) bool {
	return v.FSM.Is(stateToCheck)
}

func (v *ValidatorFSM) HandleMessage(msg *pb.OpenchainMessage) error {
	validatorLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
	if v.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Validator FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), v.FSM.Current())
	}
	err := v.FSM.Event(msg.Type.String())
	if err != nil {
		if _, ok := err.(*fsm.NoTransitionError); !ok {
			// Only allow NoTransitionError's, all others are considered true error.
			return fmt.Errorf("Peer FSM failed while handling message (%s): current state: %s, error: %s", msg.Type.String(), v.FSM.Current(), err)
			//t.Error("expected only 'NoTransitionError'")
		}
	}
	return nil
}
