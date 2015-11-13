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
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	//"github.com/openblockchain/obc-peer/openchain/chaincode"
	pb "github.com/openblockchain/obc-peer/protos"
)

// PeerChaincodeStream interface for stream between Peer and chaincode instance.
type PeerChaincodeStream interface {
        Send(*pb.ChaincodeMessage) error
        Recv() (*pb.ChaincodeMessage, error)
}


// Handler handler implementation for shim side of chaincode.
type Handler struct {
	sync.RWMutex	
	To         string
	ChatStream PeerChaincodeStream
	FSM        *fsm.FSM
	cc         Chaincode  
	// Multiple queries (and one transaction) with different Uuids can be executing in parallel for this chaincode
	// responseChannel is the channel on which responses are communicated by the shim to the chaincodeStub.
	responseChannel map[string] chan pb.ChaincodeMessage
}

func (handler *Handler) createChannel(uuid string) error {
	if handler.responseChannel == nil {
		return fmt.Errorf("Cannot create response channel for Uuid:%s", uuid)
	}
	handler.Lock()
	defer handler.Unlock()
	if handler.responseChannel[uuid] != nil {
		return fmt.Errorf("Channel for Uuid:%s exists", uuid)
	}
	handler.responseChannel[uuid] = make(chan pb.ChaincodeMessage)
	return nil
}

func (handler *Handler) deleteChannel(uuid string) {
	handler.Lock()
	if handler.responseChannel != nil {
		delete(handler.responseChannel,uuid)
	}
	handler.Unlock()
}

// NewChaincodeHandler returns a new instance of the shim side handler.
func newChaincodeHandler(to string, peerChatStream chaincode.PeerChaincodeStream, chaincode Chaincode) *Handler {
	v := &Handler{
		To:         to,
		ChatStream: peerChatStream,
		cc:         chaincode,
	}
	v.responseChannel = make(map[string] chan pb.ChaincodeMessage)

	// Create the shim side FSM
	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.ChaincodeMessage_REGISTERED.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.ChaincodeMessage_INIT.String(), Src: []string{"established"}, Dst: "init"},
			{Name: pb.ChaincodeMessage_READY.String(), Src: []string{"established"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{"init"}, Dst: "established"},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{"init"}, Dst: "init"},
			{Name: pb.ChaincodeMessage_COMPLETED.String(), Src: []string{"init"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_TRANSACTION.String(), Src: []string{"ready"}, Dst: "transaction"},
			{Name: pb.ChaincodeMessage_COMPLETED.String(), Src: []string{"transaction"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{"transaction"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{"transaction"}, Dst: "transaction"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTERED.String(): func(e *fsm.Event) { v.beforeRegistered(e) },
			//"after_" + pb.ChaincodeMessage_INIT.String(): func(e *fsm.Event) { v.beforeInit(e) },
			//"after_" + pb.ChaincodeMessage_TRANSACTION.String(): func(e *fsm.Event) { v.beforeTransaction(e) },
			"before_" + pb.ChaincodeMessage_RESPONSE.String(): func(e *fsm.Event) { v.beforeResponse(e) },
			"before_" + pb.ChaincodeMessage_ERROR.String(): func(e *fsm.Event) { v.beforeResponse(e) },
			"enter_init": func(e *fsm.Event) { v.enterInitState(e) },
			"enter_transaction": func(e *fsm.Event) { v.enterTransactionState(e) },
		},
	)
	return v
}

// beforeRegistered is called to handle the REGISTERED message.
func (handler *Handler) beforeRegistered(e *fsm.Event) {
	if _, ok := e.Args[0].(*pb.ChaincodeMessage); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, ready for invocations", pb.ChaincodeMessage_REGISTERED)
}

// Handles request to initialize chaincode
func (handler *Handler) handleInit(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the beforeInit function is exited. Interesting bug fix!!
	go func() {
		// Get the function and args from Payload
		input := &pb.ChaincodeInput{}
		unmarshalErr := proto.Unmarshal(msg.Payload, input)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Incorrect payload format. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			handler.ChatStream.Send(errMsg)
			return
		}	

		// Call chaincode's Run
		// Create the ChaincodeStub which the chaincode can use to callback
		stub := new(ChaincodeStub)
		stub.Uuid = msg.Uuid
		res, err := handler.cc.Run(stub, input.Function, input.Args)
		if err != nil {
			payload := []byte(err.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Init failed. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			handler.ChatStream.Send(errMsg)
			return
		} 

		// Send COMPLETED message to chaincode support and change state
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid} 
		chaincodeLogger.Debug("Init succeeded. Sending %s(%s)", pb.ChaincodeMessage_COMPLETED, completedMsg.Uuid)
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
		handler.ChatStream.Send(completedMsg)
	}()
}

// enterInitState will initialize the chaincode if entering init from established
func (handler *Handler) enterInitState(e *fsm.Event) {
	chaincodeLogger.Debug("(enterInitState)Entered state %s", handler.FSM.Current())
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s(uuid:%s), initializing chaincode", pb.ChaincodeMessage_INIT, msg.Uuid)
	if msg.Type.String() == pb.ChaincodeMessage_INIT.String() {
		// Call the chaincode's Run function to initialize
		handler.handleInit(msg)
	}
}

// Handles request to execute a transaction
func (handler *Handler) handleTransaction(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the beforeInit function is exited. Interesting bug fix!!
	go func() {
		// Get the function and args from Payload
		input := &pb.ChaincodeInput{}
		unmarshalErr := proto.Unmarshal(msg.Payload, input)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Incorrect payload format. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.FSM.Event(errMsg.Type.String(), errMsg)
			handler.ChatStream.Send(errMsg)
			return
		}	

		// Call chaincode's Run
		// Create the ChaincodeStub which the chaincode can use to callback
		stub := new(ChaincodeStub)
		stub.Uuid = msg.Uuid
		res, err := handler.cc.Run(stub, input.Function, input.Args)
		if err != nil {
			payload := []byte(err.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Transaction execution failed. Sending %s", pb.ChaincodeMessage_ERROR)
			errorMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
			handler.FSM.Event(errorMsg.Type.String(), errorMsg)
			handler.ChatStream.Send(errorMsg)
			return
		} 

		// Send COMPLETED message to chaincode support and change state
		chaincodeLogger.Debug("Transaction completed. Sending %s", pb.ChaincodeMessage_COMPLETED)
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid} 
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
		handler.ChatStream.Send(completedMsg)
	}()
}

// enterTransactionState will execute chaincode's Run if coming from a TRANSACTION event
func (handler *Handler) enterTransactionState(e *fsm.Event) {
	chaincodeLogger.Debug("(enterTransactionState)Entered state %s", handler.FSM.Current())
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking transaction on chaincode", pb.ChaincodeMessage_TRANSACTION)
	if msg.Type.String() == pb.ChaincodeMessage_TRANSACTION.String() {
		// Call the chaincode's Run function to invoke transaction
		handler.handleTransaction(msg)
	}
}

// BeforeResponse is called to deliver a response or error to the chaincode stub.
func (handler *Handler) beforeResponse(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, communicating on responseChannel", msg.Type)

	handler.responseChannel[msg.Uuid] <- *msg
}

// TODO: Implement method to get and put entire state map and not one key at a time?
// HandleGetState communicates with the validator to fetch the requested state information from the ledger.
func (handler *Handler) handleGetState(key string, uuid string) ([]byte, error) {
	// Create the channel on which to communicate the response from validating peer
	uniqueReqErr := handler.createChannel(uuid)
	if uniqueReqErr != nil {
		chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
		return nil, uniqueReqErr
	}
	
	// Send GET_STATE message to validator chaincode support
	payload := []byte(key)
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_GET_STATE, Payload: payload, Uuid: uuid}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_GET_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return nil, errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Payload %s, Uuid %s", pb.ChaincodeMessage_RESPONSE, responseMsg.Payload, responseMsg.Uuid)
		handler.deleteChannel(uuid)
		return responseMsg.Payload, nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s, Uuid %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload, responseMsg.Uuid)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		handler.deleteChannel(uuid)
		return nil, errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return nil, errors.New("Incorrect chaincode message received")
}

// HandlePutState communicates with the validator to put state information into the ledger.
func (handler *Handler) handlePutState(key string, value []byte, uuid string) error {
	payload := &pb.PutStateInfo{Key: key, Value: value}
	payloadBytes, err := proto.Marshal(payload) 
	if err != nil {
		return errors.New("Failed to process put state request")
	}

	// Create the channel on which to communicate the response from validating peer
	uniqueReqErr := handler.createChannel(uuid)
	if uniqueReqErr != nil {
		chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
		return uniqueReqErr
	}

	// Send PUT_STATE message to validator chaincode support
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_PUT_STATE, Payload: payloadBytes, Uuid: uuid}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_PUT_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully updated state", pb.ChaincodeMessage_RESPONSE)
		handler.deleteChannel(uuid)
		return nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		handler.deleteChannel(uuid)
		return errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return errors.New("Incorrect chaincode message received")
}

// HandleDelState communicates with the validator to delete a key from the state in the ledger.
func (handler *Handler) handleDelState(key string, uuid string) error {
	// Create the channel on which to communicate the response from validating peer
	uniqueReqErr := handler.createChannel(uuid)
	if uniqueReqErr != nil {
		chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
		return uniqueReqErr
	}

	// Send DEL_STATE message to validator chaincode support
	payload := []byte(key)
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_DEL_STATE, Payload: payload, Uuid: uuid}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_DEL_STATE)

	// Wait on responseChannel for response
	responseMsg, ok := <- handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully deleted state", pb.ChaincodeMessage_RESPONSE)
		handler.deleteChannel(uuid)
		return nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		n := bytes.IndexByte(responseMsg.Payload, 0)
		handler.deleteChannel(uuid)
		return errors.New(string(responseMsg.Payload[:n]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return errors.New("Incorrect chaincode message received")
}

// HandleInvokeChaincode communicates with the validator to invoke another chaincode.
func (handler *Handler) handleInvokeChaincode(chaincodeID string, args []byte, uuid string) ([]byte, error) {
	// TODO: To be implemented
	return nil, nil
}

// HandleMessage message handles loop for shim side of chaincode/validator stream.
func (handler *Handler) handleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s ", msg.Type)
	if handler.FSM.Cannot(msg.Type.String()) {
		errStr := fmt.Sprintf("Chaincode handler FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), handler.FSM.Current())
		err := errors.New(errStr)
		payload := []byte(err.Error())
		errorMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid} 
		handler.ChatStream.Send(errorMsg)
		return err
	}
	err := handler.FSM.Event(msg.Type.String(), msg)
	return filterError(err)
}

// filterError filters the errors to allow NoTransitionError and CanceledError to not propogate for cases where embedded Err == nil.
func filterError(errFromFSMEvent error) error {
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
