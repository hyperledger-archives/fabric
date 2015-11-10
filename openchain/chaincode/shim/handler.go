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

package shim

import (
	"fmt"
	"errors"
	"bytes"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	pb "github.com/openblockchain/obc-peer/protos"
)

// Handler handler implementation for shim side of chaincode.
type Handler struct {
	To         string
	ChatStream chaincode.PeerChaincodeStream
	FSM        *fsm.FSM
	cc         Chaincode  
	ccStub     *ChaincodeStub
}

// responseChannel is the channel on which responses are communicated by the shim to the chaincodeStub.
var responseChannel chan pb.ChaincodeMessage

func init() {
	responseChannel = make(chan pb.ChaincodeMessage)
}

// NewChaincodeHandler returns a new instance of the shim side handler.
func NewChaincodeHandler(to string, peerChatStream chaincode.PeerChaincodeStream, chaincode Chaincode, stub *ChaincodeStub) *Handler {
	v := &Handler{
		To:         to,
		ChatStream: peerChatStream,
		cc:         chaincode,
		ccStub:     stub,	
	}

	// Create the shim side FSM
	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.ChaincodeMessage_REGISTERED.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.ChaincodeMessage_INIT.String(), Src: []string{"established"}, Dst: "init"},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{"init"}, Dst: "established"},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{"init"}, Dst: "init"},
			{Name: pb.ChaincodeMessage_COMPLETED.String(), Src: []string{"init"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_TRANSACTION.String(), Src: []string{"ready"}, Dst: "transaction"},
			{Name: pb.ChaincodeMessage_COMPLETED.String(), Src: []string{"transaction"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{"transaction"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{"transaction"}, Dst: "transaction"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTERED.String(): func(e *fsm.Event) { v.BeforeRegistered(e) },
			"before_" + pb.ChaincodeMessage_INIT.String(): func(e *fsm.Event) { v.BeforeInit(e) },
			"before_" + pb.ChaincodeMessage_TRANSACTION.String(): func(e *fsm.Event) { v.BeforeInit(e) },
			"before_" + pb.ChaincodeMessage_RESPONSE.String(): func(e *fsm.Event) { v.BeforeResponse(e) },
			"before_" + pb.ChaincodeMessage_ERROR.String(): func(e *fsm.Event) { v.BeforeResponse(e) },
		},
	)
	return v
}

// BeforeRegistered is called to handle the REGISTERED message.
func (handler *Handler) BeforeRegistered(e *fsm.Event) {
	if _, ok := e.Args[0].(*pb.ChaincodeMessage); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, ready for invocations", pb.ChaincodeMessage_REGISTERED)
}

// BeforeInit is called to initialize the chaincode.
func (handler *Handler) BeforeInit(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s(uuid:%s), initializing chaincode", pb.ChaincodeMessage_INIT, msg.Uuid)
	
	// Call the chaincode's Run function to initialize
	go func() {
		// Get the function and args from Payload
		input := &pb.ChaincodeInput{}
		unmarshalErr := proto.Unmarshal(msg.Payload, input)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Incorrect payload format. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.ChatStream.Send(errMsg)
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			return
		}	

		// Call chaincode's Run
		res, err := handler.cc.Run(handler.ccStub, input.Function, input.Args)
		if err != nil {
			payload := []byte(err.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Init failed. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.ChatStream.Send(errMsg)
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			return
		} 

		// Send COMPLETED message to chaincode support and change state
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid} 
		chaincodeLogger.Debug("Init succeeded. Sending %s(%s)", pb.ChaincodeMessage_COMPLETED, completedMsg.Uuid)
		handler.ChatStream.Send(completedMsg)
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
	}()
	return
}

// BeforeTransaction is called to invoke a transaction on the chaincode.
func (handler *Handler) BeforeTransaction(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking transaction on chaincode", pb.ChaincodeMessage_TRANSACTION)
	
	// Call the chaincode's Run function
	go func() {
		// Get the function and args from Payload
		input := &pb.ChaincodeInput{}
		unmarshalErr := proto.Unmarshal(msg.Payload, input)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Incorrect payload format. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.ChatStream.Send(errMsg)
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			return
		}	

		// Call chaincode's Run
		res, err := handler.cc.Run(handler.ccStub, input.Function, input.Args)
		if err != nil {
			payload := []byte(err.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Transaction execution failed. Sending %s", pb.ChaincodeMessage_ERROR)
			errorMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.ChatStream.Send(errorMsg)
			handler.FSM.Event(errorMsg.Type.String(), errorMsg)
			return
		} 

		// Send COMPLETED message to chaincode support and change state
		chaincodeLogger.Debug("Transaction completed. Sending %s", pb.ChaincodeMessage_COMPLETED)
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid} 
		handler.ChatStream.Send(completedMsg)
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
	}()
	return
}

// BeforeResponse is called to deliver a response or error to the chaincode stub.
func (handler *Handler) BeforeResponse(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, communicating on responseChannel", msg.Type)

	responseChannel <- *msg
}

// TODO: Implement method to get and put entire state map and not one key at a time?
// HandleGetState communicates with the validator to fetch the requested state information from the ledger.
func (handler *Handler) HandleGetState(key string) ([]byte, error) {
	// Send GET_STATE message to validator chaincode support
	payload := []byte(key)
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_GET_STATE, Payload: payload}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_GET_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- responseChannel
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		return nil, errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_RESPONSE, responseMsg.Payload)
		return responseMsg.Payload, nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		return nil, errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	return nil, errors.New("Incorrect chaincode message received")
}

// HandlePutState communicates with the validator to put state information into the ledger.
func (handler *Handler) HandlePutState(key string, value []byte) error {
	// Send PUT_STATE message to validator chaincode support
	// TODO: Need protobuf here to merge key and value?
	payload := []byte(key)
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_PUT_STATE, Payload: payload}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_PUT_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- responseChannel
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		return errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully updated state", pb.ChaincodeMessage_RESPONSE)
		return nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		return errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	return errors.New("Incorrect chaincode message received")
}

// HandleDelState communicates with the validator to delete a key from the state in the ledger.
func (handler *Handler) HandleDelState(key string) error {
	// Send DEL_STATE message to validator chaincode support
	payload := []byte(key)
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_DEL_STATE, Payload: payload}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_DEL_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- responseChannel
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		return errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully deleted state", pb.ChaincodeMessage_RESPONSE)
		return nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		return errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	return errors.New("Incorrect chaincode message received")
}

// HandleInvokeChaincode communicates with the validator to invoke another chaincode.
func (handler *Handler) HandleInvokeChaincode(chaincodeID string, args []byte) ([]byte, error) {
	// TODO: To be implemented
	return nil, nil
}

// HandleMessage message handles loop for shim side of chaincode/validator stream.
func (handler *Handler) HandleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s ", msg.Type)
	if handler.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Chaincode handler FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), handler.FSM.Current())
	}
	err := handler.FSM.Event(msg.Type.String(), msg)
	return FilterError(err)
}

// FilterError filters the errors to allow NoTransitionError and CanceledError to not propogate for cases where embedded Err == nil.
func FilterError(errFromFSMEvent error) error {
	if errFromFSMEvent != nil {
		if noTransitionErr, ok := errFromFSMEvent.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return errFromFSMEvent
				//t.Error("expected only 'NoTransitionError'")
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
