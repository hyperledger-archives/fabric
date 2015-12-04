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
	"errors"
	"fmt"
	"sync"
	"time"

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
	responseChannel map[string]chan pb.ChaincodeMessage
	// Track which UUIDs are transactions and which are queries, to decide whether get/put state and invoke chaincode are allowed.
	isTransaction map[string]bool
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
		delete(handler.responseChannel, uuid)
	}
	handler.Unlock()
}

// markIsTransaction marks a UUID as a transaction or a query; true = transaction, false = query
func (handler *Handler) markIsTransaction(uuid string, isTrans bool) bool {
	if handler.isTransaction == nil {
		return false
	}
	handler.Lock()
	defer handler.Unlock()
	handler.isTransaction[uuid] = isTrans
	return true
}

func (handler *Handler) deleteIsTransaction(uuid string) {
	handler.Lock()
	if handler.isTransaction != nil {
		delete(handler.isTransaction, uuid)
	}
	handler.Unlock()
}

// NewChaincodeHandler returns a new instance of the shim side handler.
func newChaincodeHandler(to string, peerChatStream PeerChaincodeStream, chaincode Chaincode) *Handler {
	v := &Handler{
		To:         to,
		ChatStream: peerChatStream,
		cc:         chaincode,
	}
	v.responseChannel = make(map[string]chan pb.ChaincodeMessage)
	v.isTransaction = make(map[string]bool)

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
			{Name: pb.ChaincodeMessage_QUERY.String(), Src: []string{"transaction"}, Dst: "transaction"},
			{Name: pb.ChaincodeMessage_QUERY.String(), Src: []string{"ready"}, Dst: "ready"},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{"ready"}, Dst: "ready"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTERED.String(): func(e *fsm.Event) { v.beforeRegistered(e) },
			//"after_" + pb.ChaincodeMessage_INIT.String(): func(e *fsm.Event) { v.beforeInit(e) },
			//"after_" + pb.ChaincodeMessage_TRANSACTION.String(): func(e *fsm.Event) { v.beforeTransaction(e) },
			"after_" + pb.ChaincodeMessage_RESPONSE.String(): func(e *fsm.Event) { v.afterResponse(e) },
			"after_" + pb.ChaincodeMessage_ERROR.String():    func(e *fsm.Event) { v.afterError(e) },
			"enter_init":                                     func(e *fsm.Event) { v.enterInitState(e) },
			"enter_transaction":                              func(e *fsm.Event) { v.enterTransactionState(e) },
			//"enter_ready":                                     func(e *fsm.Event) { v.enterReadyState(e) },
			"after_" + pb.ChaincodeMessage_COMPLETED.String(): func(e *fsm.Event) { v.afterCompleted(e) },
			"before_" + pb.ChaincodeMessage_QUERY.String():    func(e *fsm.Event) { v.beforeQuery(e) }, //only checks for QUERY
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

// handleInit handles request to initialize chaincode.
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

		// Mark as a transaction (allow put/del state)
		handler.markIsTransaction(msg.Uuid, true)

		// Call chaincode's Run
		// Create the ChaincodeStub which the chaincode can use to callback
		stub := new(ChaincodeStub)
		stub.UUID = msg.Uuid
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

		// delete isTransaction entry
		handler.deleteIsTransaction(msg.Uuid)

		// Send COMPLETED message to chaincode support and change state
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid}
		chaincodeLogger.Debug("Init succeeded. Sending %s(%s)", pb.ChaincodeMessage_COMPLETED, completedMsg.Uuid)
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
		handler.ChatStream.Send(completedMsg)
	}()
}

// enterInitState will initialize the chaincode if entering init from established.
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

// handleTransaction Handles request to execute a transaction.
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

		// Mark as a transaction (allow put/del state)
		handler.markIsTransaction(msg.Uuid, true)

		// Call chaincode's Run
		// Create the ChaincodeStub which the chaincode can use to callback
		stub := new(ChaincodeStub)
		stub.UUID = msg.Uuid
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

		// delete isTransaction entry
		handler.deleteIsTransaction(msg.Uuid)

		// Send COMPLETED message to chaincode support and change state
		chaincodeLogger.Debug("Transaction completed. Sending %s", pb.ChaincodeMessage_COMPLETED)
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_COMPLETED, Payload: res, Uuid: msg.Uuid}
		handler.FSM.Event(completedMsg.Type.String(), completedMsg)
		//there's still a timing window .... user could send another transaction
		//before the state transitions to ready.... so we really have to do the send
		//in the readystate (see afterCompleted (previously enterReadyState))
		//handler.ChatStream.Send(completedMsg)
	}()
}

// handleQuery handles request to execute a query.
func (handler *Handler) handleQuery(msg *pb.ChaincodeMessage) {
	// Query does not transition state. It can happen anytime after Ready
	go func() {
		// Get the function and args from Payload
		input := &pb.ChaincodeInput{}
		unmarshalErr := proto.Unmarshal(msg.Payload, input)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Incorrect payload format. Sending %s", pb.ChaincodeMessage_QUERY_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_QUERY_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		// Mark as a query (do not allow put/del state)
		handler.markIsTransaction(msg.Uuid, false)

		// Call chaincode's Query
		// Create the ChaincodeStub which the chaincode can use to callback
		stub := new(ChaincodeStub)
		stub.UUID = msg.Uuid
		res, err := handler.cc.Query(stub, input.Function, input.Args)
		if err != nil {
			payload := []byte(err.Error())
			// Send ERROR message to chaincode support and change state
			chaincodeLogger.Debug("Query execution failed. Sending %s", pb.ChaincodeMessage_QUERY_ERROR)
			errorMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_QUERY_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errorMsg)
			return
		}

		// delete isTransaction entry
		handler.deleteIsTransaction(msg.Uuid)

		// Send COMPLETED message to chaincode support
		chaincodeLogger.Debug("Query completed. Sending %s", pb.ChaincodeMessage_QUERY_COMPLETED)
		completedMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_QUERY_COMPLETED, Payload: res, Uuid: msg.Uuid}
		handler.ChatStream.Send(completedMsg)
	}()
}

// enterTransactionState will execute chaincode's Run if coming from a TRANSACTION event.
func (handler *Handler) enterTransactionState(e *fsm.Event) {
	chaincodeLogger.Debug("(enterTransactionState)Entered state %s", handler.FSM.Current())
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking transaction on chaincode(Src:%s, Dst:%s)", pb.ChaincodeMessage_TRANSACTION, e.Src, e.Dst)
	if msg.Type.String() == pb.ChaincodeMessage_TRANSACTION.String() {
		// Call the chaincode's Run function to invoke transaction
		handler.handleTransaction(msg)
	}
	/* This is being called from beforeQuery()
	else if msg.Type.String() == pb.ChaincodeMessage_QUERY.String() {
		handler.handleQuery(msg)
	}
	*/
}

// enterReadyState will need to handle COMPLETED event by sending message to the peer
//func (handler *Handler) enterReadyState(e *fsm.Event) {

// afterCompleted will need to handle COMPLETED event by sending message to the peer
func (handler *Handler) afterCompleted(e *fsm.Event) {
	chaincodeLogger.Debug("(afterCompleted)Completed in state %s", handler.FSM.Current())
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	//no need, now I AM in completed state if msg.Type.String() == pb.ChaincodeMessage_COMPLETED.String() {
	// now that we are comfortably in READY, send message to peer side
	chaincodeLogger.Debug("sending COMPLETED to validator for tid %s", msg.Uuid)
	handler.ChatStream.Send(msg)
	//}
}

// beforeQuery is invoked when a query message is received from the validator
func (handler *Handler) beforeQuery(e *fsm.Event) {
	chaincodeLogger.Debug("(beforeQuery)in state %s", handler.FSM.Current())
	if e.Args != nil {
		msg, ok := e.Args[0].(*pb.ChaincodeMessage)
		if !ok {
			e.Cancel(fmt.Errorf("Received unexpected message type"))
			return
		}
		handler.handleQuery(msg)
	}
}

// afterResponse is called to deliver a response or error to the chaincode stub.
func (handler *Handler) afterResponse(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}

	handler.responseChannel[msg.Uuid] <- *msg
	chaincodeLogger.Debug("Received %s, communicated on responseChannel(state:%s)", msg.Type, handler.FSM.Current())
}

func (handler *Handler) afterError(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}

	/* There are two situations in which the ERROR event can be triggered:
	 * 1. When an error is encountered within handleInit or handleTransaction - some issue at the chaincode side; In this case there will be no responseChannel and the message has been sent to the validator.
	 * 2. The chaincode has initiated a request (get/put/del state) to the validator and is expecting a response on the responseChannel; If ERROR is received from validator, this needs to be notified on the responseChannel.
	 */
	if handler.responseChannel[msg.Uuid] != nil {
		chaincodeLogger.Debug("Error received from validator %s, communicating on responseChannel(state:%s)", msg.Type, handler.FSM.Current())
		handler.responseChannel[msg.Uuid] <- *msg
	}
}

// TODO: Implement method to get and put entire state map and not one key at a time?
// handleGetState communicates with the validator to fetch the requested state information from the ledger.
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
	responseMsg, ok := <-handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return nil, errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Payload: %s, Uuid %s", pb.ChaincodeMessage_RESPONSE, responseMsg.Payload, responseMsg.Uuid)
		handler.deleteChannel(uuid)
		return responseMsg.Payload, nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload: %s, Uuid %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload, responseMsg.Uuid)
		handler.deleteChannel(uuid)
		return nil, errors.New(string(responseMsg.Payload[:]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return nil, errors.New("Incorrect chaincode message received")
}

// handlePutState communicates with the validator to put state information into the ledger.
func (handler *Handler) handlePutState(key string, value []byte, uuid string) error {
	// Check if this is a transaction
	chaincodeLogger.Debug("Inside putstate, isTransaction = %t", handler.isTransaction[uuid])
	if !handler.isTransaction[uuid] {
		return errors.New("Cannot put state in query context")
	}

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
	responseMsg, ok := <-handler.responseChannel[uuid]
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
		chaincodeLogger.Debug("Received %s. Payload: %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		handler.deleteChannel(uuid)
		return errors.New(string(responseMsg.Payload[:]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return errors.New("Incorrect chaincode message received")
}

// handleDelState communicates with the validator to delete a key from the state in the ledger.
func (handler *Handler) handleDelState(key string, uuid string) error {
	// Check if this is a transaction
	if !handler.isTransaction[uuid] {
		return errors.New("Cannot del state in query context")
	}

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
	responseMsg, ok := <-handler.responseChannel[uuid]
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
		chaincodeLogger.Debug("Received %s. Payload: %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		handler.deleteChannel(uuid)
		return errors.New(string(responseMsg.Payload[:]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return errors.New("Incorrect chaincode message received")
}

// handleInvokeChaincode communicates with the validator to invoke another chaincode.
func (handler *Handler) handleInvokeChaincode(chaincodeURL string, chaincodeVersion string, function string, args []string, uuid string) ([]byte, error) {
	// Check if this is a transaction
	if !handler.isTransaction[uuid] {
		return nil, errors.New("Cannot invoke chaincode in query context")
	}

	chaincodeID := &pb.ChaincodeID{Url: chaincodeURL, Version: chaincodeVersion}
	input := &pb.ChaincodeInput{Function: function, Args: args}
	payload := &pb.ChaincodeSpec{ChaincodeID: chaincodeID, CtorMsg: input}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, errors.New("Failed to process invoke chaincode request")
	}

	// Create the channel on which to communicate the response from validating peer
	uniqueReqErr := handler.createChannel(uuid)
	if uniqueReqErr != nil {
		chaincodeLogger.Debug("Another request pending for this Uuid. Cannot process.")
		return nil, uniqueReqErr
	}

	// Send INVOKE_CHAINCODE message to validator chaincode support
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_INVOKE_CHAINCODE, Payload: payloadBytes, Uuid: uuid}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_INVOKE_CHAINCODE)

	// Wait on responseChannel for response
	responseMsg, ok := <-handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return nil, errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully invoked chaincode", pb.ChaincodeMessage_RESPONSE)
		handler.deleteChannel(uuid)
		return responseMsg.Payload, nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload: %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		handler.deleteChannel(uuid)
		return nil, errors.New(string(responseMsg.Payload[:]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return nil, errors.New("Incorrect chaincode message received")
}

// handleQueryChaincode communicates with the validator to query another chaincode.
func (handler *Handler) handleQueryChaincode(chaincodeURL string, chaincodeVersion string, function string, args []string, uuid string) ([]byte, error) {
	chaincodeID := &pb.ChaincodeID{Url: chaincodeURL, Version: chaincodeVersion}
	input := &pb.ChaincodeInput{Function: function, Args: args}
	payload := &pb.ChaincodeSpec{ChaincodeID: chaincodeID, CtorMsg: input}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, errors.New("Failed to process query chaincode request")
	}

	// Create the channel on which to communicate the response from validating peer
	uniqueReqErr := handler.createChannel(uuid)
	if uniqueReqErr != nil {
		chaincodeLogger.Debug("Another request pending for this Uuid. Cannot process.")
		return nil, uniqueReqErr
	}

	// Send INVOKE_QUERY message to validator chaincode support
	msg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_INVOKE_QUERY, Payload: payloadBytes, Uuid: uuid}
	handler.ChatStream.Send(msg)
	chaincodeLogger.Debug("Sending %s", pb.ChaincodeMessage_INVOKE_QUERY)

	// Wait on responseChannel for response
	responseMsg, ok := <-handler.responseChannel[uuid]
	if !ok {
		chaincodeLogger.Debug("Received unexpected message type")
		handler.deleteChannel(uuid)
		return nil, errors.New("Received unexpected message type")
	}

	if responseMsg.Type.String() == pb.ChaincodeMessage_RESPONSE.String() {
		// Success response
		chaincodeLogger.Debug("Received %s. Successfully queried chaincode", pb.ChaincodeMessage_RESPONSE)
		handler.deleteChannel(uuid)
		return responseMsg.Payload, nil
	}
	if responseMsg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		// Error response
		chaincodeLogger.Debug("Received %s. Payload: %s", pb.ChaincodeMessage_ERROR, responseMsg.Payload)
		handler.deleteChannel(uuid)
		return nil, errors.New(string(responseMsg.Payload[:]))
	}

	// Incorrect chaincode message received
	chaincodeLogger.Debug("Incorrect chaincode message %s recieved. Expecting %s or %s", responseMsg.Type, pb.ChaincodeMessage_RESPONSE, pb.ChaincodeMessage_ERROR)
	handler.deleteChannel(uuid)
	return nil, errors.New("Incorrect chaincode message received")
}

// handleMessage message handles loop for shim side of chaincode/validator stream.
func (handler *Handler) handleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s(state:%s)", msg.Type, handler.FSM.Current())
	if handler.FSM.Cannot(msg.Type.String()) {
		//TODO - this is a hack but it fixes an important hole that's
		//tricky to fix. "FSM.Cannot(..)" may fail not because we are in a bad state
		//but because FSM's transition object is not nilled. There is a tiny timing
		//window in looplab's fsm.go
		//	f.transition()
		//	f.transition = nil
		//before f.transition is set to nil where we could get a message causing Cannot() to return false.
		//We see this typically in ready state. Some rearranging might help us here
		//but for now, trying once more after a small sleep/seems to fix this and
		//helps the test cases run smoothly.
		//NEEDS REVISITING...

		time.Sleep(2 * time.Millisecond)

		//try again
		if handler.FSM.Cannot(msg.Type.String()) {
			errStr := fmt.Sprintf("Chaincode handler FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), handler.FSM.Current())
			err := errors.New(errStr)
			payload := []byte(err.Error())
			errorMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errorMsg)
			return err
		}
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
