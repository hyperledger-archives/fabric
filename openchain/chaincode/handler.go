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
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	pb "github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/ledger"
)

const (
	createdstate     = "created"     //start state
	establishedstate = "established" //in: CREATED, rcv:  REGISTER, send: REGISTERED, INIT
	initstate        = "init"        //in:ESTABLISHED, rcv:-, send: INIT
	readystate       = "ready"       //in:ESTABLISHED,TRANSACTION, rcv:COMPLETED
	transactionstate = "transaction" //in:READY, rcv: xact from consensus, send: TRANSACTION
	busyinitstate    = "busyinit"    //in:INIT, rcv: PUT_STATE, DEL_STATE, INVOKE_CHAINCODE
	busyxactstate    = "busyxact"    //in:TRANSACION, rcv: PUT_STATE, DEL_STATE, INVOKE_CHAINCODE
	endstate         = "end"         //in:INIT,ESTABLISHED, rcv: error, terminate container

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
	SendMessage(msg *pb.ChaincodeMessage) error
}

type transactionContext struct {
	transactionSecContext *pb.Transaction
	responseNotifier      chan *pb.ChaincodeMessage
}

type nextStateInfo struct {
	msg      *pb.ChaincodeMessage
	sendToCC bool
}

// Handler responsbile for managment of Peer's side of chaincode stream
type Handler struct {
	sync.RWMutex
	ChatStream  PeerChaincodeStream
	FSM         *fsm.FSM
	ChaincodeID *pb.ChaincodeID

	// A copy of decrypted deploy tx this handler manages, no code
	deployTXSecContext *pb.Transaction

	chaincodeSupport *ChaincodeSupport
	registered       bool
	readyNotify      chan bool
	// Map of tx uuid to either invoke or query tx (decrypted). Each tx will be
	// added prior to execute and remove when done execute
	txCtxs map[string]*transactionContext

	uuidMap map[string]bool

	// Track which UUIDs are queries; Although the shim maintains this, it cannot be trusted.
	isTransaction map[string]bool
	nextState     chan *nextStateInfo
}

func (handler *Handler) createTxContext(uuid string, tx *pb.Transaction) (*transactionContext, error) {
	if handler.txCtxs == nil {
		return nil, fmt.Errorf("cannot create notifier for Uuid:%s", uuid)
	}
	handler.Lock()
	defer handler.Unlock()
	if handler.txCtxs[uuid] != nil {
		return nil, fmt.Errorf("Uuid:%s exists", uuid)
	}
	txctx := &transactionContext{transactionSecContext: tx, responseNotifier: make(chan *pb.ChaincodeMessage, 1)}
	handler.txCtxs[uuid] = txctx
	return txctx, nil
}

func (handler *Handler) getTxContext(uuid string) *transactionContext {
	handler.Lock()
	defer handler.Unlock()
	return handler.txCtxs[uuid]
}

func (handler *Handler) deleteTxContext(uuid string) {
	handler.Lock()
	if handler.txCtxs != nil {
		delete(handler.txCtxs, uuid)
	}
	handler.Unlock()
}

func (handler *Handler) encryptOrDecrypt(encrypt bool, uuid string, payload []byte) ([]byte, error) {
	secHelper := handler.chaincodeSupport.getSecHelper()
	if secHelper == nil {
		return payload, nil
	}

	txctx := handler.getTxContext(uuid)
	if txctx == nil {
		return nil, fmt.Errorf("No context for uuid %s", uuid)
	}
	if txctx.transactionSecContext == nil {
		return nil, fmt.Errorf("transaction context is nil for uuid %s", uuid)
	}

	var enc crypto.StateEncryptor
	var err error
	if txctx.transactionSecContext.Type == pb.Transaction_CHAINCODE_NEW {
		if enc, err = secHelper.GetStateEncryptor(handler.deployTXSecContext, handler.deployTXSecContext); err != nil {
			return nil, fmt.Errorf("error getting crypto encryptor for deploy tx :%s", err)
		}
	} else if txctx.transactionSecContext.Type == pb.Transaction_CHAINCODE_EXECUTE || txctx.transactionSecContext.Type == pb.Transaction_CHAINCODE_QUERY {
		if enc, err = secHelper.GetStateEncryptor(handler.deployTXSecContext, txctx.transactionSecContext); err != nil {
			return nil, fmt.Errorf("error getting crypto encryptor %s", err)
		}
	} else {
		return nil, fmt.Errorf("invalid transaction type %s", txctx.transactionSecContext.Type.String())
	}
	if enc == nil {
		return nil, fmt.Errorf("secure context returns nil encryptor for tx %s", uuid)
	}
	if chaincodeLogger.IsEnabledFor(logging.DEBUG) {
		chaincodeLogger.Debug("Payload before encrypt/decrypt: %v", payload)
	}
	if encrypt {
		payload, err = enc.Encrypt(payload)
	} else {
		payload, err = enc.Decrypt(payload)
	}
	if chaincodeLogger.IsEnabledFor(logging.DEBUG) {
		chaincodeLogger.Debug("Payload after encrypt/decrypt: %v", payload)
	}

	return payload, err
}

func (handler *Handler) decrypt(uuid string, payload []byte) ([]byte, error) {
	return handler.encryptOrDecrypt(false, uuid, payload)
}

func (handler *Handler) encrypt(uuid string, payload []byte) ([]byte, error) {
	return handler.encryptOrDecrypt(true, uuid, payload)
}

func (handler *Handler) deregister() error {
	if handler.registered {
		handler.chaincodeSupport.deregisterHandler(handler)
	}
	return nil
}

func (handler *Handler) triggerNextState(msg *pb.ChaincodeMessage, send bool) {
	handler.nextState <- &nextStateInfo{msg, send}
}

func (handler *Handler) processStream() error {
	defer handler.deregister()
	msgAvail := make(chan *pb.ChaincodeMessage)
	var nsInfo *nextStateInfo
	var in *pb.ChaincodeMessage
	var err error
	for {
		in = nil
		err = nil
		nsInfo = nil
		go func() {
			var in2 *pb.ChaincodeMessage
			in2, err = handler.ChatStream.Recv()
			msgAvail <- in2
		}()
		select {
		case in = <-msgAvail:
			// Defer the deregistering of the this handler.
			if in == nil || err == io.EOF {
				if err == nil {
					err = fmt.Errorf("Received nil message, ending chaincode support stream")
					chaincodeLogger.Debug("Received nil message, ending chaincode support stream")
				} else {
					chaincodeLogger.Debug("Received EOF, ending chaincode support stream")
				}
				return err
			}
			if err != nil {
				chaincodeLog.Error(fmt.Sprintf("Error handling chaincode support stream: %s", err))
				return err
			}
		case nsInfo = <-handler.nextState:
			in = nsInfo.msg
		}
		err = handler.HandleMessage(in)
		if err != nil {
			chaincodeLog.Error(fmt.Sprintf("Error handling message, ending stream: %s", err))
			return fmt.Errorf("Error handling message, ending stream: %s", err)
		}
		if nsInfo != nil && nsInfo.sendToCC {
			if err = handler.ChatStream.Send(in); err != nil {
				chaincodeLog.Error(fmt.Sprintf("Error sending %s: %s", in.Type.String(), err))
				return fmt.Errorf("Error sending %s: %s", in.Type.String(), err)
			}
		}
	}
}

// HandleChaincodeStream Main loop for handling the associated Chaincode stream
func HandleChaincodeStream(chaincodeSupport *ChaincodeSupport, stream pb.ChaincodeSupport_RegisterServer) error {
	deadline, ok := stream.Context().Deadline()
	chaincodeLogger.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	handler := newChaincodeSupportHandler(chaincodeSupport, stream)
	return handler.processStream()
}

func newChaincodeSupportHandler(chaincodeSupport *ChaincodeSupport, peerChatStream PeerChaincodeStream) *Handler {
	v := &Handler{
		ChatStream: peerChatStream,
	}
	v.chaincodeSupport = chaincodeSupport
	//we want this to block
	v.nextState = make(chan *nextStateInfo)

	v.FSM = fsm.NewFSM(
		createdstate,
		fsm.Events{
			//Send REGISTERED, then, if deploy { trigger INIT(via INIT) } else { trigger READY(via COMPLETED) }
			{Name: pb.ChaincodeMessage_REGISTER.String(), Src: []string{createdstate}, Dst: establishedstate},
			{Name: pb.ChaincodeMessage_INIT.String(), Src: []string{establishedstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_READY.String(), Src: []string{establishedstate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_TRANSACTION.String(), Src: []string{readystate}, Dst: transactionstate},
			{Name: pb.ChaincodeMessage_PUT_STATE.String(), Src: []string{transactionstate}, Dst: busyxactstate},
			{Name: pb.ChaincodeMessage_DEL_STATE.String(), Src: []string{transactionstate}, Dst: busyxactstate},
			{Name: pb.ChaincodeMessage_INVOKE_CHAINCODE.String(), Src: []string{transactionstate}, Dst: busyxactstate},
			{Name: pb.ChaincodeMessage_PUT_STATE.String(), Src: []string{initstate}, Dst: busyinitstate},
			{Name: pb.ChaincodeMessage_DEL_STATE.String(), Src: []string{initstate}, Dst: busyinitstate},
			{Name: pb.ChaincodeMessage_INVOKE_CHAINCODE.String(), Src: []string{initstate}, Dst: busyinitstate},
			{Name: pb.ChaincodeMessage_COMPLETED.String(), Src: []string{initstate, readystate, transactionstate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_GET_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_GET_STATE.String(), Src: []string{initstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_GET_STATE.String(), Src: []string{busyinitstate}, Dst: busyinitstate},
			{Name: pb.ChaincodeMessage_GET_STATE.String(), Src: []string{transactionstate}, Dst: transactionstate},
			{Name: pb.ChaincodeMessage_GET_STATE.String(), Src: []string{busyxactstate}, Dst: busyxactstate},
			{Name: pb.ChaincodeMessage_RANGE_QUERY_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_RANGE_QUERY_STATE.String(), Src: []string{initstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_RANGE_QUERY_STATE.String(), Src: []string{busyinitstate}, Dst: busyinitstate},
			{Name: pb.ChaincodeMessage_RANGE_QUERY_STATE.String(), Src: []string{transactionstate}, Dst: transactionstate},
			{Name: pb.ChaincodeMessage_RANGE_QUERY_STATE.String(), Src: []string{busyxactstate}, Dst: busyxactstate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{initstate}, Dst: endstate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{transactionstate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{busyinitstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{busyxactstate}, Dst: transactionstate},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{busyinitstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{busyxactstate}, Dst: transactionstate},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTER.String():         func(e *fsm.Event) { v.beforeRegisterEvent(e, v.FSM.Current()) },
			"before_" + pb.ChaincodeMessage_COMPLETED.String():        func(e *fsm.Event) { v.beforeCompletedEvent(e, v.FSM.Current()) },
			"before_" + pb.ChaincodeMessage_INIT.String():             func(e *fsm.Event) { v.beforeInitState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_GET_STATE.String():         func(e *fsm.Event) { v.afterGetState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_RANGE_QUERY_STATE.String(): func(e *fsm.Event) { v.afterRangeQueryState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_PUT_STATE.String():         func(e *fsm.Event) { v.afterPutState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_DEL_STATE.String():         func(e *fsm.Event) { v.afterDelState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_INVOKE_CHAINCODE.String():  func(e *fsm.Event) { v.afterInvokeChaincode(e, v.FSM.Current()) },
			"enter_" + establishedstate:                               func(e *fsm.Event) { v.enterEstablishedState(e, v.FSM.Current()) },
			"enter_" + initstate:                                      func(e *fsm.Event) { v.enterInitState(e, v.FSM.Current()) },
			"enter_" + readystate:                                     func(e *fsm.Event) { v.enterReadyState(e, v.FSM.Current()) },
			"enter_" + busyinitstate:                                  func(e *fsm.Event) { v.enterBusyState(e, v.FSM.Current()) },
			"enter_" + busyxactstate:                                  func(e *fsm.Event) { v.enterBusyState(e, v.FSM.Current()) },
			"enter_" + transactionstate:                               func(e *fsm.Event) { v.enterTransactionState(e, v.FSM.Current()) },
			"enter_" + endstate:                                       func(e *fsm.Event) { v.enterEndState(e, v.FSM.Current()) },
		},
	)

	return v
}

func (handler *Handler) createUUIDEntry(uuid string) bool {
	if handler.uuidMap == nil {
		return false
	}
	handler.Lock()
	defer handler.Unlock()
	if handler.uuidMap[uuid] {
		return false
	}
	handler.uuidMap[uuid] = true
	return handler.uuidMap[uuid]
}

func (handler *Handler) deleteUUIDEntry(uuid string) {
	handler.Lock()
	defer handler.Unlock()
	if handler.uuidMap != nil {
		delete(handler.uuidMap, uuid)
	} else {
		chaincodeLogger.Warning("UUID %s not found!", uuid)
	}
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

func (handler *Handler) notifyDuringStartup(val bool) {
	//if USER_RUNS_CC readyNotify will be nil
	if handler.readyNotify != nil {
		chaincodeLogger.Debug("Notifying during startup")
		handler.readyNotify <- val
	} else {
		chaincodeLogger.Debug("nothing to notify (dev mode ?)")
	}
}

// beforeRegisterEvent is invoked when chaincode tries to register.
func (handler *Handler) beforeRegisterEvent(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Received %s in state %s", e.Event, state)
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeID := &pb.ChaincodeID{}
	err := proto.Unmarshal(msg.Payload, chaincodeID)
	if err != nil {
		e.Cancel(fmt.Errorf("Error in received %s, could NOT unmarshal registration info: %s", pb.ChaincodeMessage_REGISTER, err))
		return
	}

	// Now register with the chaincodeSupport
	handler.ChaincodeID = chaincodeID
	err = handler.chaincodeSupport.registerHandler(handler)
	if err != nil {
		e.Cancel(err)
		handler.notifyDuringStartup(false)
		return
	}

	chaincodeLogger.Debug("Got %s for chaincodeID = %s, sending back %s", e.Event, chaincodeID, pb.ChaincodeMessage_REGISTERED)
	if err := handler.ChatStream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTERED}); err != nil {
		e.Cancel(fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_REGISTERED, err))
		handler.notifyDuringStartup(false)
		return
	}
}

func (handler *Handler) notify(msg *pb.ChaincodeMessage) {
	handler.Lock()
	defer handler.Unlock()
	tctx := handler.txCtxs[msg.Uuid]
	if tctx == nil {
		chaincodeLogger.Debug("notifier Uuid:%s does not exist", msg.Uuid)
	} else {
		chaincodeLogger.Debug("notifying Uuid:%s", msg.Uuid)
		tctx.responseNotifier <- msg
	}
}

// beforeCompletedEvent is invoked when chaincode has completed execution of init, invoke or query.
func (handler *Handler) beforeCompletedEvent(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Received %s in state %s", e.Event, state)
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Notify on channel once into READY state
	chaincodeLogger.Debug("beforeCompleted uuid:%s - not in ready state will notify when in readystate", msg.Uuid)
	return
}

// beforeInitState is invoked before an init message is sent to the chaincode.
func (handler *Handler) beforeInitState(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Before state %s.. notifying waiter that we are up", state)
	handler.notifyDuringStartup(true)
}

// afterGetState handles a GET_STATE request from the chaincode.
func (handler *Handler) afterGetState(e *fsm.Event, state string) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking get state from ledger", pb.ChaincodeMessage_GET_STATE)

	// Query ledger for state
	defer handler.handleGetState(msg)
	chaincodeLogger.Debug("Exiting GET_STATE")
}

// Handles query to ledger to get state
func (handler *Handler) handleGetState(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode uuid
		uniqueReq := handler.createUUIDEntry(msg.Uuid)
		if !uniqueReq {
			// Drop this request
			chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
			return
		}

		key := string(msg.Payload)
		ledgerObj, ledgerErr := ledger.GetLedger()
		if ledgerErr != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(ledgerErr.Error())
			chaincodeLogger.Debug("Failed to get chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		// Invoke ledger to get state
		chaincodeID := handler.ChaincodeID.Name
		// ToDo: Eventually, once consensus is plugged in, we need to set bool to true for ledger
		res, err := ledgerObj.GetState(chaincodeID, key, false)
		chaincodeLogger.Debug("Got state %s from ledger", string(res))
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed to get chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
		} else {
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)

			// Decrypt the data if the confidential is enabled
			if res, err = handler.decrypt(msg.Uuid, res); err == nil {
				// Send response msg back to chaincode. GetState will not trigger event
				chaincodeLogger.Debug("Got state. Sending %s", pb.ChaincodeMessage_RESPONSE)
				responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: res, Uuid: msg.Uuid}
				handler.ChatStream.Send(responseMsg)
			} else {
				// Send err msg back to chaincode.
				chaincodeLogger.Debug("Got error while decrypting. Sending %s", pb.ChaincodeMessage_ERROR)
				errBytes := []byte(err.Error())
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: errBytes, Uuid: msg.Uuid}
				handler.ChatStream.Send(errMsg)
			}

		}

	}()
}

const maxRangeQueryStateLimit = 100

// afterRangeQueryState handles a RANGE_QUERY_STATE request from the chaincode.
func (handler *Handler) afterRangeQueryState(e *fsm.Event, state string) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking get state from ledger", pb.ChaincodeMessage_RANGE_QUERY_STATE)

	// Query ledger for state
	defer handler.handleRangeQueryState(msg)
	chaincodeLogger.Debug("Exiting GET_STATE")
}

// Handles query to ledger to rage query state
func (handler *Handler) handleRangeQueryState(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterRangeQueryState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode uuid
		uniqueReq := handler.createUUIDEntry(msg.Uuid)
		if !uniqueReq {
			// Drop this request
			chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
			return
		}

		rangeQueryStateInfo := &pb.RangeQueryStateInfo{}
		unmarshalErr := proto.Unmarshal(msg.Payload, rangeQueryStateInfo)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			chaincodeLogger.Debug("Failed to unmarshall range query request. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		ledger, ledgerErr := ledger.GetLedger()
		if ledgerErr != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(ledgerErr.Error())
			chaincodeLogger.Debug("Failed to get ledger. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		chaincodeID := handler.ChaincodeID.Name
		rangeIter, err := ledger.GetStateRangeScanIterator(chaincodeID, rangeQueryStateInfo.StartKey, rangeQueryStateInfo.EndKey)
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed to get ledger scan iterator. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}
		defer rangeIter.Close()

		var limit uint32
		if rangeQueryStateInfo.Limit <= maxRangeQueryStateLimit {
			limit = rangeQueryStateInfo.Limit
		} else {
			limit = maxRangeQueryStateLimit
		}

		var keysAndValues []*pb.RangeQueryStateKeyValue
		var i = uint32(0)
		for ; rangeIter.Next() && i < limit; i++ {
			key, value := rangeIter.GetKeyValue()
			// Decrypt the data if the confidential is enabled
			decryptedValue, err := handler.decrypt(msg.Uuid, value)
			if err != nil {
				payload := []byte(unmarshalErr.Error())
				chaincodeLogger.Debug("Failed decrypt value. Sending %s", pb.ChaincodeMessage_ERROR)
				// Remove uuid from current set
				handler.deleteUUIDEntry(msg.Uuid)
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
				handler.ChatStream.Send(errMsg)
				return
			}
			keyAndValue := pb.RangeQueryStateKeyValue{Key: key, Value: decryptedValue}
			keysAndValues = append(keysAndValues, &keyAndValue)
		}

		hasNext := i == limit && rangeIter.Next()

		payload := &pb.RangeQueryStateResponse{KeysAndValues: keysAndValues, HasMore: hasNext}
		payloadBytes, err := proto.Marshal(payload)
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed marshall resopnse. Sending %s", pb.ChaincodeMessage_ERROR)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		// Remove uuid from current set
		handler.deleteUUIDEntry(msg.Uuid)

		chaincodeLogger.Debug("Got keys and values. Sending %s", pb.ChaincodeMessage_RESPONSE)
		responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: payloadBytes, Uuid: msg.Uuid}
		handler.ChatStream.Send(responseMsg)

	}()
}

// afterPutState handles a PUT_STATE request from the chaincode.
func (handler *Handler) afterPutState(e *fsm.Event, state string) {
	_, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s in state %s, invoking put state to ledger", pb.ChaincodeMessage_PUT_STATE, state)

	// Put state into ledger handled within enterBusyState
}

// afterDelState handles a DEL_STATE request from the chaincode.
func (handler *Handler) afterDelState(e *fsm.Event, state string) {
	_, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking delete state from ledger", pb.ChaincodeMessage_DEL_STATE)

	// Delete state from ledger handled within enterBusyState
}

// afterInvokeChaincode handles an INVOKE_CHAINCODE request from the chaincode.
func (handler *Handler) afterInvokeChaincode(e *fsm.Event, state string) {
	_, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s in state %s, invoking another chaincode", pb.ChaincodeMessage_INVOKE_CHAINCODE, state)

	// Invoke another chaincode handled within enterBusyState
}

// Handles request to ledger to put state
func (handler *Handler) enterBusyState(e *fsm.Event, state string) {
	go func() {
		chaincodeLogger.Debug("state is %s", state)
		msg, _ := e.Args[0].(*pb.ChaincodeMessage)
		// First check if this UUID is a transaction; error otherwise
		if !handler.isTransaction[msg.Uuid] {
			payload := []byte(fmt.Sprintf("Cannot handle %s in query context", msg.Type.String()))
			chaincodeLogger.Debug("Cannot handle %s in query context. Sending %s", msg.Type.String(), pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.triggerNextState(errMsg, true)
			return
		}

		// Check if this is the unique request from this chaincode uuid
		uniqueReq := handler.createUUIDEntry(msg.Uuid)
		if !uniqueReq {
			// Drop this request
			chaincodeLogger.Debug("Another request pending for this Uuid. Cannot process.")
			return
		}

		ledgerObj, ledgerErr := ledger.GetLedger()
		if ledgerErr != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(ledgerErr.Error())
			chaincodeLogger.Debug("Failed to handle %s. Sending %s", msg.Type.String(), pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.triggerNextState(errMsg, true)
			return
		}

		chaincodeID := handler.ChaincodeID.Name
		var err error
		var res []byte

		if msg.Type.String() == pb.ChaincodeMessage_PUT_STATE.String() {
			putStateInfo := &pb.PutStateInfo{}
			unmarshalErr := proto.Unmarshal(msg.Payload, putStateInfo)
			if unmarshalErr != nil {
				payload := []byte(unmarshalErr.Error())
				chaincodeLogger.Debug("Unable to decipher payload. Sending %s", pb.ChaincodeMessage_ERROR)
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
				// Remove uuid from current set
				handler.deleteUUIDEntry(msg.Uuid)
				handler.triggerNextState(errMsg, true)
				return
			}

			var pVal []byte
			// Encrypt the data if the confidential is enabled
			if pVal, err = handler.encrypt(msg.Uuid, putStateInfo.Value); err == nil {
				// Invoke ledger to put state
				err = ledgerObj.SetState(chaincodeID, putStateInfo.Key, pVal)
			}
		} else if msg.Type.String() == pb.ChaincodeMessage_DEL_STATE.String() {
			// Invoke ledger to delete state
			key := string(msg.Payload)
			err = ledgerObj.DeleteState(chaincodeID, key)
		} else if msg.Type.String() == pb.ChaincodeMessage_INVOKE_CHAINCODE.String() {
			chaincodeSpec := &pb.ChaincodeSpec{}
			unmarshalErr := proto.Unmarshal(msg.Payload, chaincodeSpec)
			if unmarshalErr != nil {
				payload := []byte(unmarshalErr.Error())
				chaincodeLogger.Debug("Unable to decipher payload. Sending %s", pb.ChaincodeMessage_ERROR)
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
				// Remove uuid from current set
				handler.deleteUUIDEntry(msg.Uuid)
				handler.triggerNextState(errMsg, true)
				return
			}

			// Get the chaincodeID to invoke
			newChaincodeID := chaincodeSpec.ChaincodeID.Name

			// Create the transaction object
			chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: chaincodeSpec}
			transaction, _ := pb.NewChaincodeExecute(chaincodeInvocationSpec, msg.Uuid, pb.Transaction_CHAINCODE_EXECUTE)

			// Launch the new chaincode if not already running
			_, chaincodeInput, launchErr := handler.chaincodeSupport.LaunchChaincode(context.Background(), transaction)
			if launchErr != nil {
				payload := []byte(launchErr.Error())
				chaincodeLogger.Debug("Failed to launch invoked chaincode. Sending %s", pb.ChaincodeMessage_ERROR)
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
				// Remove uuid from current set
				handler.deleteUUIDEntry(msg.Uuid)
				handler.triggerNextState(errMsg, true)
				return
			}

			// TODO: Need to handle timeout correctly
			timeout := time.Duration(30000) * time.Millisecond

			ccMsg, _ := createTransactionMessage(transaction.Uuid, chaincodeInput)

			// Execute the chaincode
			//TODOOOOOOOOOOOOOOOOOOOOOOOOO - pass transaction to Execute
			response, execErr := handler.chaincodeSupport.Execute(context.Background(), newChaincodeID, ccMsg, timeout, nil)
			err = execErr
			res = response.Payload
		}

		if err != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed to handle %s. Sending %s", msg.Type.String(), pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.triggerNextState(errMsg, true)
			return
		}

		// Send response msg back to chaincode.
		chaincodeLogger.Debug("Completed %s. Sending %s", msg.Type.String(), pb.ChaincodeMessage_RESPONSE)
		responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: res, Uuid: msg.Uuid}
		// Remove uuid from current set
		handler.deleteUUIDEntry(msg.Uuid)
		handler.triggerNextState(responseMsg, true)
	}()
}

func (handler *Handler) enterEstablishedState(e *fsm.Event, state string) {
	handler.notifyDuringStartup(true)
	chaincodeLogger.Debug("Entered state; notified %s", state)
}

func (handler *Handler) enterInitState(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Entered state %s", state)
	ccMsg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	//very first time entering init state from established, send message to chaincode
	if ccMsg.Type == pb.ChaincodeMessage_INIT {
		// Mark isTransaction to allow put/del state and invoke other chaincodes
		handler.markIsTransaction(ccMsg.Uuid, true)
		if err := handler.ChatStream.Send(ccMsg); err != nil {
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: []byte(fmt.Sprintf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)), Uuid: ccMsg.Uuid}
			handler.notify(errMsg)
		}
	}
}

func (handler *Handler) enterReadyState(e *fsm.Event, state string) {
	// Now notify
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	handler.deleteIsTransaction(msg.Uuid)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	handler.notify(msg)

	chaincodeLogger.Debug("Entered state %s", state)
}

func (handler *Handler) enterTransactionState(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Entered state %s", state)
}

func (handler *Handler) enterEndState(e *fsm.Event, state string) {
	defer handler.deregister()
	// Now notify
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	handler.deleteIsTransaction(msg.Uuid)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	handler.notify(msg)
	chaincodeLogger.Debug("Entered state %s", state)
	e.Cancel(fmt.Errorf("Entered end state"))
}

//if initArgs is set (should be for "deploy" only) move to Init
//else move to ready
func (handler *Handler) initOrReady(uuid string, f *string, initArgs []string, tx *pb.Transaction) (chan *pb.ChaincodeMessage, error) {
	var event string
	var ccMsg *pb.ChaincodeMessage
	var send bool

	txctx, funcErr := handler.createTxContext(uuid, tx)
	if funcErr != nil {
		return nil, funcErr
	}

	notfy := txctx.responseNotifier

	if f != nil || initArgs != nil {
		chaincodeLogger.Debug("sending INIT")
		var f2 string
		if f != nil {
			f2 = *f
		}
		funcArgsMsg := &pb.ChaincodeInput{Function: f2, Args: initArgs}
		var payload []byte
		if payload, funcErr = proto.Marshal(funcArgsMsg); funcErr != nil {
			handler.deleteTxContext(uuid)
			return nil, fmt.Errorf("Failed to marshall %s : %s\n", ccMsg.Type.String(), funcErr)
		}
		ccMsg = &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_INIT, Payload: payload, Uuid: uuid}
		event = pb.ChaincodeMessage_INIT.String()
		send = false
	} else {
		chaincodeLogger.Debug("sending READY")
		ccMsg = &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_READY, Uuid: uuid}
		event = pb.ChaincodeMessage_READY.String()
		send = true
	}

	funcErr = handler.FSM.Event(event, ccMsg)
	if funcErr != nil {
		handler.deleteTxContext(uuid)
		return nil, fmt.Errorf("Failed to trigger FSM event %s : %s\n", ccMsg.Type.String(), funcErr)
	}

	if send {
		if funcErr = handler.ChatStream.Send(ccMsg); funcErr != nil {
			handler.deleteTxContext(uuid)
			return nil, fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_READY, funcErr)
		}
	}

	//set deploy transaction on the handler
	handler.deployTXSecContext = tx

	return notfy, funcErr
}

// Handles request to query another chaincode
func (handler *Handler) handleQueryChaincode(msg *pb.ChaincodeMessage) {
	go func() {
		// Check if this is the unique request from this chaincode uuid
		uniqueReq := handler.createUUIDEntry(msg.Uuid)
		if !uniqueReq {
			// Drop this request
			chaincodeLogger.Debug("Another request pending for this Uuid. Cannot process.")
			return
		}

		chaincodeSpec := &pb.ChaincodeSpec{}
		unmarshalErr := proto.Unmarshal(msg.Payload, chaincodeSpec)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			chaincodeLogger.Debug("Unable to decipher payload. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(errMsg)
			return
		}

		// Get the chaincodeID to invoke
		newChaincodeID := chaincodeSpec.ChaincodeID.Name

		// Create the transaction object
		chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: chaincodeSpec}
		transaction, _ := pb.NewChaincodeExecute(chaincodeInvocationSpec, msg.Uuid, pb.Transaction_CHAINCODE_QUERY)

		// Launch the new chaincode if not already running
		_, chaincodeInput, launchErr := handler.chaincodeSupport.LaunchChaincode(context.Background(), transaction)
		if launchErr != nil {
			payload := []byte(launchErr.Error())
			chaincodeLogger.Debug("Failed to launch invoked chaincode. Sending %s", pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(errMsg)
			return
		}

		// TODO: Need to handle timeout correctly
		timeout := time.Duration(30000) * time.Millisecond

		ccMsg, _ := createQueryMessage(transaction.Uuid, chaincodeInput)

		// Query the chaincode
		//TODOOOOOOOOOOOOOOOOOOOOOOOOO - pass transaction to Execute
		response, execErr := handler.chaincodeSupport.Execute(context.Background(), newChaincodeID, ccMsg, timeout, nil)

		if execErr != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(execErr.Error())
			chaincodeLogger.Debug("Failed to handle %s. Sending %s", msg.Type.String(), pb.ChaincodeMessage_ERROR)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(errMsg)
			return
		}

		// Send response msg back to chaincode.
		chaincodeLogger.Debug("Completed %s. Sending %s", msg.Type.String(), pb.ChaincodeMessage_RESPONSE)
		responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: response.Payload, Uuid: msg.Uuid}
		// Remove uuid from current set
		handler.deleteUUIDEntry(msg.Uuid)
		handler.ChatStream.Send(responseMsg)
	}()
}

// HandleMessage implementation of MessageHandler interface.  Peer's handling of Chaincode messages.
func (handler *Handler) HandleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s in state %s", msg.Type, handler.FSM.Current())

	//QUERY_COMPLETED message can happen ONLY for Transaction_QUERY (stateless)
	if msg.Type == pb.ChaincodeMessage_QUERY_COMPLETED {
		chaincodeLogger.Debug("HandleMessage- QUERY_COMPLETED for uuid:%s. Notify", msg.Uuid)
		handler.deleteIsTransaction(msg.Uuid)
		handler.notify(msg)
		return nil
	} else if msg.Type == pb.ChaincodeMessage_QUERY_ERROR {
		chaincodeLogger.Debug("HandleMessage- QUERY_ERROR (%s) for uuid:%s. Notify", string(msg.Payload), msg.Uuid)
		handler.deleteIsTransaction(msg.Uuid)
		handler.notify(msg)
		return nil
	} else if msg.Type == pb.ChaincodeMessage_INVOKE_QUERY {
		// Received request to query another chaincode from shim
		chaincodeLogger.Debug("HandleMessage- Received request to query another chaincode")
		handler.handleQueryChaincode(msg)
		return nil
	}
	if handler.FSM.Cannot(msg.Type.String()) {
		// Check if this is a request from validator in query context
		if msg.Type.String() == pb.ChaincodeMessage_PUT_STATE.String() || msg.Type.String() == pb.ChaincodeMessage_DEL_STATE.String() || msg.Type.String() == pb.ChaincodeMessage_INVOKE_CHAINCODE.String() {
			// Check if this UUID is a transaction
			if !handler.isTransaction[msg.Uuid] {
				payload := []byte(fmt.Sprintf("Cannot handle %s in query context", msg.Type.String()))
				chaincodeLogger.Debug("Cannot handle %s in query context. Sending %s", msg.Type.String(), pb.ChaincodeMessage_ERROR)
				errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
				handler.ChatStream.Send(errMsg)
				return fmt.Errorf("Cannot handle %s in query context", msg.Type.String())
			}
		}

		// Other errors
		return fmt.Errorf("Chaincode handler validator FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), handler.FSM.Current())
	}
	chaincodeLogger.Debug("Received message %s from shim", msg.Type.String())
	if msg.Type.String() == pb.ChaincodeMessage_ERROR.String() {
		chaincodeLogger.Debug("Got error: %s", string(msg.Payload))
	}
	eventErr := handler.FSM.Event(msg.Type.String(), msg)
	filteredErr := filterError(eventErr)
	if filteredErr != nil {
		chaincodeLogger.Debug("Failed to trigger FSM event %s: %s", msg.Type.String(), filteredErr)
	}

	return filteredErr
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

func (handler *Handler) sendExecuteMessage(msg *pb.ChaincodeMessage, tx *pb.Transaction) (chan *pb.ChaincodeMessage, error) {
	txctx, err := handler.createTxContext(msg.Uuid, tx)
	if err != nil {
		return nil, err
	}

	// Mark UUID as either transaction or query
	chaincodeLogger.Debug("Inside sendExecuteMessage. Message %s, Uuid %s", msg.Type.String(), msg.Uuid)
	if msg.Type.String() == pb.ChaincodeMessage_QUERY.String() {
		handler.markIsTransaction(msg.Uuid, false)
	} else {
		handler.markIsTransaction(msg.Uuid, true)
	}

	// Trigger FSM event if it is a transaction
	if msg.Type.String() == pb.ChaincodeMessage_TRANSACTION.String() {
		err = handler.FSM.Event(msg.Type.String(), msg)
		if err != nil {
			handler.deleteTxContext(msg.Uuid)
			chaincodeLogger.Debug("Failed to trigger FSM event TRANSACTION: %s", err)
			return nil, fmt.Errorf("Failed to trigger FSM event TRANSACTION: %s", err)
		}
	}

	// Send the message to shim
	if err = handler.ChatStream.Send(msg); err != nil {
		handler.deleteTxContext(msg.Uuid)
		return nil, fmt.Errorf("SendMessage error sending %s(%s)", msg.Uuid, err)
	}

	return txctx.responseNotifier, nil
}

func (handler *Handler) isRunning() bool {
	switch handler.FSM.Current() {
	case createdstate:
		fallthrough
	case establishedstate:
		fallthrough
	case initstate:
		return false
	default:
		return true
	}
}

/****************
func (handler *Handler) initEvent() (chan *pb.ChaincodeMessage, error) {
	if handler.responseNotifiers == nil {
		return nil,fmt.Errorf("SendMessage called before registration for Uuid:%s", msg.Uuid)
	}
	var notfy chan *pb.ChaincodeMessage
	handler.Lock()
	if handler.responseNotifiers[msg.Uuid] != nil {
		handler.Unlock()
		return nil, fmt.Errorf("SendMessage Uuid:%s exists", msg.Uuid)
	}
	//note the explicit use of buffer 1. We won't block if the receiver times outi and does not wait
	//for our response
	handler.responseNotifiers[msg.Uuid] = make(chan *pb.ChaincodeMessage, 1)
	handler.Unlock()

	if err := c.ChatStream.Send(msg); err != nil {
		deleteNotifier(msg.Uuid)
		return nil, fmt.Errorf("SendMessage error sending %s(%s)", msg.Uuid, err)
	}
	return notfy, nil
}
*******************/
