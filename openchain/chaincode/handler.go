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

package chaincode

import (
	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
)

var chaincodeLogger = logging.MustGetLogger("chaincode")

// PeerChaincodeStream interface for stream between Peer and chaincode instance.
type PeerChaincodeStream interface {
	Send(*pb.ChaincodeMessage) error
	Recv() (*pb.ChaincodeMessage, error)
}

// MessageHandler interface for handling chaincode messages (common between Peer chaincode support and chaincode)
type MessageHandler interface {
	HandleMessage(msg *pb.ChaincodeMessage) error
	//SendMessage(msg *pb.ChaincodeMessage) error
}

// Handler responsbile for managment of Peer's side of chaincode stream
type Handler struct {
	ChatStream      PeerChaincodeStream
	FSM             *fsm.FSM
	ChaincodeID     *pb.ChainletID
	chainletSupport *ChainletSupport
	registered      bool
}

func (c *Handler) deregister() error {
	if c.registered {
		c.chainletSupport.deregisterHandler(c)
	}
	return nil
}

func (c *Handler) processStream() error {
	for {
		in, err := c.ChatStream.Recv()
		// Defer the deregistering of the this handler.
		defer c.deregister()
		if err == io.EOF {
			chainletLog.Debug("Received EOF, ending chaincode support stream")
			return err
		}
		if err != nil {
			chainletLog.Error(fmt.Sprintf("Error handling chaincode support stream: %s", err))
			return err
		}
		err = c.HandleMessage(in)
		if err != nil {
			return fmt.Errorf("Error handling message, ending stream: %s", err)
		}
	}
}

// HandleChaincodeStream Main loop for handling the associated Chaincode stream
func HandleChaincodeStream(chainletSupport *ChainletSupport, stream pb.ChainletSupport_RegisterServer) error {
	deadline, ok := stream.Context().Deadline()
	chainletLog.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	handler := newChaincodeSupportHandler(chainletSupport, stream)
	return handler.processStream()
}

func newChaincodeSupportHandler(chainletSupport *ChainletSupport, peerChatStream PeerChaincodeStream) *Handler {
	v := &Handler{
		ChatStream: peerChatStream,
	}
	v.chainletSupport = chainletSupport

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.ChaincodeMessage_REGISTER.String(), Src: []string{"created"}, Dst: "established"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTER.String(): func(e *fsm.Event) { v.beforeRegister(e) },
		},
	)
	return v
}

func (c *Handler) beforeRegister(e *fsm.Event) {
	chaincodeLogger.Debug("Received %s", e.Event)
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chainletID := &pb.ChainletID{}
	err := proto.Unmarshal(msg.Payload, chainletID)
	if err != nil {
		e.Cancel(fmt.Errorf("Error in received %s, could NOT unmarshal registration info: %s", pb.ChaincodeMessage_REGISTER, err))
		return
	}

	// Now register with the chainletSupport
	c.ChaincodeID = chainletID
	err = c.chainletSupport.registerHandler(c)
	if err != nil {
		e.Cancel(err)
		return
	}

	chainletLog.Debug("Got %s for chainldetID = %s, sending back %s", e.Event, chainletID, pb.ChaincodeMessage_REGISTERED)
	if err := c.ChatStream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTERED}); err != nil {
		e.Cancel(fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_REGISTERED, err))
		return
	}

	// Mark as successfully registered
	c.registered = true
}

// HandleMessage implementation of MessageHandler interface.  Peer's handling of Chaincode messages.
func (c *Handler) HandleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s ", msg.Type)
	if c.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Chaincode handler FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), c.FSM.Current())
	}
	err := c.FSM.Event(msg.Type.String(), msg)
	return filterError(err)
}

// Filter the Errors to allow NoTransitionError and CanceledError to not propogate for cases where embedded Err == nil
func filterError(errFromFSMEvent error) error {
	if errFromFSMEvent != nil {
		if noTransitionErr, ok := errFromFSMEvent.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return errFromFSMEvent
			}
			chaincodeLogger.Debug("Ignoring NoTransitionError: %s", noTransitionErr)
		}
		if canceledErr, ok := errFromFSMEvent.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return canceledErr
				//t.Error("expected only 'NoTransitionError'")
			}
			chaincodeLogger.Debug("Ignoring CanceledError: %s", canceledErr)
		}
	}
	return nil
}
