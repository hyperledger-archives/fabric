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

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"

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

// Handler responsbile for managment of Peer's side of chaincode stream
type Handler struct {
	sync.RWMutex
	ChatStream        PeerChaincodeStream
	FSM               *fsm.FSM
	ChaincodeID       *pb.ChaincodeID
	chaincodeSupport  *ChaincodeSupport
	registered        bool
	readyNotify       chan bool
	responseNotifiers map[string]chan *pb.ChaincodeMessage
	// Uuids of all in-progress state invocations
	uuidMap map[string]bool
}

func (handler *Handler) deregister() error {
	if handler.registered {
		handler.chaincodeSupport.deregisterHandler(handler)
	}
	return nil
}

func (handler *Handler) processStream() error {
	defer handler.deregister()
	for {
		in, err := handler.ChatStream.Recv()
		// Defer the deregistering of the this handler.
		if err == io.EOF {
			chaincodeLogger.Debug("Received EOF, ending chaincode support stream")
			return err
		}
		if err != nil {
			chaincodeLog.Error(fmt.Sprintf("Error handling chaincode support stream: %s", err))
			return err
		}
		err = handler.HandleMessage(in)
		if err != nil {
			return fmt.Errorf("Error handling message, ending stream: %s", err)
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
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{initstate}, Dst: endstate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{transactionstate}, Dst: readystate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{busyinitstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_ERROR.String(), Src: []string{busyxactstate}, Dst: transactionstate},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{busyinitstate}, Dst: initstate},
			{Name: pb.ChaincodeMessage_RESPONSE.String(), Src: []string{busyxactstate}, Dst: transactionstate},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTER.String():  func(e *fsm.Event) { v.beforeRegisterEvent(e, v.FSM.Current()) },
			"before_" + pb.ChaincodeMessage_COMPLETED.String(): func(e *fsm.Event) { v.beforeCompletedEvent(e, v.FSM.Current()) },
			"before_" + pb.ChaincodeMessage_INIT.String():      func(e *fsm.Event) { v.beforeinitstate(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_GET_STATE.String():  func(e *fsm.Event) { v.beforeGetState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_PUT_STATE.String():  func(e *fsm.Event) { v.beforePutState(e, v.FSM.Current()) },
			"after_" + pb.ChaincodeMessage_DEL_STATE.String():  func(e *fsm.Event) { v.beforeDelState(e, v.FSM.Current()) },
			"enter_" + establishedstate:                        func(e *fsm.Event) { v.enterestablishedstate(e, v.FSM.Current()) },
			"enter_" + initstate:                               func(e *fsm.Event) { v.enterinitstate(e, v.FSM.Current()) },
			"enter_" + readystate:                              func(e *fsm.Event) { v.enterreadystate(e, v.FSM.Current()) },
			"enter_" + busyinitstate:                           func(e *fsm.Event) { v.enterbusyinitstate(e, v.FSM.Current()) },
			"enter_" + busyxactstate:                           func(e *fsm.Event) { v.enterBusyXactState(e, v.FSM.Current()) },
			"enter_" + transactionstate:                        func(e *fsm.Event) { v.entertransactionstate(e, v.FSM.Current()) },
			"enter_" + endstate:                                func(e *fsm.Event) { v.enterendstate(e, v.FSM.Current()) },
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
	if handler.uuidMap != nil {
		delete(handler.uuidMap, uuid)
	}
	handler.Unlock()
}

func (handler *Handler) notifyDuringStartup(val bool) {
	//if USER_RUNS_CC readyNotify will be nil
	if handler.readyNotify != nil {
		chaincodeLogger.Debug("Notifying during startup")
		handler.readyNotify <- val
		chaincodeLogger.Debug("Notified during startup")
	} else {
		chaincodeLogger.Debug("readyNotify is nil!!!!")
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

	chaincodeLogger.Debug("Got %s for chainletID = %s, sending back %s", e.Event, chaincodeID, pb.ChaincodeMessage_REGISTERED)
	if err := handler.ChatStream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTERED}); err != nil {
		e.Cancel(fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_REGISTERED, err))
		handler.notifyDuringStartup(false)
		return
	}
}

func (handler *Handler) notify(msg *pb.ChaincodeMessage) {
	handler.Lock()
	defer handler.Unlock()
	notfy := handler.responseNotifiers[msg.Uuid]
	if notfy == nil {
		chaincodeLogger.Debug("notifier Uuid:%s does not exist", msg.Uuid)
	} else {
		chaincodeLogger.Debug("notifying Uuid:%s", msg.Uuid)
		notfy <- msg
		chaincodeLogger.Debug("notified Uuid:%s", msg.Uuid)
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

// beforeinitstate is invoked before an init message is sent to the chaincode.
func (handler *Handler) beforeinitstate(e *fsm.Event, state string) {
	chaincodeLogger.Debug("Before state %s.. notifying waiter that we are up", state)
	handler.notifyDuringStartup(true)
}

// beforeGetState handles a GET_STATE request from the chaincode.
func (handler *Handler) beforeGetState(e *fsm.Event, state string) {
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
	// the beforeGetState function is exited. Interesting bug fix!!
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
		chaincodeID, _ := getChaincodeID(handler.ChaincodeID)
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
			// Send response msg back to chaincode. GetState will not trigger event
			chaincodeLogger.Debug("Got state. Sending %s", pb.ChaincodeMessage_RESPONSE)
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: res, Uuid: msg.Uuid}
			handler.ChatStream.Send(responseMsg)
		}

	}()
}

// beforePutState handles a PUT_STATE request from the chaincode.
func (handler *Handler) beforePutState(e *fsm.Event, state string) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s in state %s, invoking put state to ledger", pb.ChaincodeMessage_PUT_STATE, state)

	// Put state into ledger
	defer handler.handlePutState(msg)
	chaincodeLogger.Debug("Exiting PUT_STATE")
}

// Handles request to ledger to put state
func (handler *Handler) handlePutState(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the beforePutState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode uuid
		uniqueReq := handler.createUUIDEntry(msg.Uuid)
		if !uniqueReq {
			// Drop this request
			chaincodeLogger.Debug("Another state request pending for this Uuid. Cannot process.")
			return
		}

		putStateInfo := &pb.PutStateInfo{}
		unmarshalErr := proto.Unmarshal(msg.Payload, putStateInfo)
		if unmarshalErr != nil {
			payload := []byte(unmarshalErr.Error())
			chaincodeLogger.Debug("Unable to decipher payload. Sending %s", pb.ChaincodeMessage_ERROR)
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_ERROR.String(), unmarshalErr)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		ledgerObj, ledgerErr := ledger.GetLedger()
		if ledgerErr != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(ledgerErr.Error())
			chaincodeLogger.Debug("Failed to set chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_ERROR.String(), ledgerErr)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		// Invoke ledger to set state
		chaincodeID, _ := getChaincodeID(handler.ChaincodeID)
		err := ledgerObj.SetState(chaincodeID, putStateInfo.Key, putStateInfo.Value)
		if err != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed to set chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_ERROR.String(), err)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(errMsg)
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			chaincodeLogger.Debug("Got state. Sending %s", pb.ChaincodeMessage_RESPONSE)
			var res []byte
			responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: res, Uuid: msg.Uuid}
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_RESPONSE.String(), responseMsg)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event RESPONSE: %s", eventErr)
			}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(responseMsg)
		}
	}()
}

// beforeDelState handles a DEL_STATE request from the chaincode.
func (handler *Handler) beforeDelState(e *fsm.Event, state string) {
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	chaincodeLogger.Debug("Received %s, invoking delete state from ledger", pb.ChaincodeMessage_DEL_STATE)

	// Delete state from ledger
	defer handler.handleDelState(msg)
}

// Handles request to ledger to delete state
func (handler *Handler) handleDelState(msg *pb.ChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the beforeDelState function is exited. Interesting bug fix!!
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
			// Send error msg back to chaincode and trigger event
			payload := []byte(ledgerErr.Error())
			chaincodeLogger.Debug("Failed to delete chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_ERROR.String(), ledgerErr)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			handler.ChatStream.Send(errMsg)
			return
		}

		chaincodeID, _ := getChaincodeID(handler.ChaincodeID)
		err := ledgerObj.DeleteState(chaincodeID, key)
		if err != nil {
			// Send error msg back to chaincode and trigger event
			payload := []byte(err.Error())
			chaincodeLogger.Debug("Failed to delete chaincode state. Sending %s", pb.ChaincodeMessage_ERROR)
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_ERROR.String(), err)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: payload, Uuid: msg.Uuid}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(errMsg)
		} else {
			// Send response msg back to chaincode. DelState will not trigger event
			var res []byte
			chaincodeLogger.Debug("Deleted state. Sending %s", pb.ChaincodeMessage_RESPONSE)
			responseMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_RESPONSE, Payload: res, Uuid: msg.Uuid}
			// Send FSM event to trigger state change
			eventErr := handler.FSM.Event(pb.ChaincodeMessage_RESPONSE.String(), responseMsg)
			if eventErr != nil {
				chaincodeLogger.Debug("Failed to trigger FSM event ERROR: %s", eventErr)
			}
			// Remove uuid from current set
			handler.deleteUUIDEntry(msg.Uuid)
			handler.ChatStream.Send(responseMsg)
		}

	}()
}

func (handler *Handler) enterestablishedstate(e *fsm.Event, state string) {
	handler.notifyDuringStartup(true)
	chaincodeLogger.Debug("(enterestablishedstate)Entered state; notified %s", state)
}

func (handler *Handler) enterinitstate(e *fsm.Event, state string) {
	chaincodeLogger.Debug("(enterinitstate)Entered state %s", state)
	ccMsg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	//very first time entering init state from established, send message to chaincode
	if ccMsg.Type == pb.ChaincodeMessage_INIT {
		if err := handler.ChatStream.Send(ccMsg); err != nil {
			errMsg := &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: []byte(fmt.Sprintf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)), Uuid: ccMsg.Uuid}
			handler.notify(errMsg)
		}
	}
}

func (handler *Handler) enterreadystate(e *fsm.Event, state string) {
	// Now notify
	msg, ok := e.Args[0].(*pb.ChaincodeMessage)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	handler.notify(msg)

	chaincodeLogger.Debug("(enterreadystate)Entered state %s", state)
}

func (handler *Handler) enterbusyinitstate(e *fsm.Event, state string) {
	chaincodeLogger.Debug("(enterbusyinitstate)Entered state %s", state)
}

func (handler *Handler) enterBusyXactState(e *fsm.Event, state string) {
	chaincodeLogger.Debug("(enterBusyXactState)Entered state %s", state)
}

func (handler *Handler) entertransactionstate(e *fsm.Event, state string) {
	chaincodeLogger.Debug("(entertransactionstate)Entered state %s", state)
}

func (handler *Handler) enterendstate(e *fsm.Event, state string) {
	chaincodeLogger.Debug("(enterendstate)Entered state %s", state)
}

//if initArgs is set (should be for "deploy" only) move to Init
//else move to ready
func (handler *Handler) initOrReady(uuid string, f *string, initArgs []string) (chan *pb.ChaincodeMessage, error) {
	var event string
	var notfy chan *pb.ChaincodeMessage
	var ccMsg *pb.ChaincodeMessage
	if f != nil || initArgs != nil {
		chaincodeLogger.Debug("sending INIT")
		var f2 string
		if f != nil {
			f2 = *f
		}
		funcArgsMsg := &pb.ChaincodeInput{Function: f2, Args: initArgs}
		payload, err := proto.Marshal(funcArgsMsg)
		if err != nil {
			return nil, err
		}
		notfy, err = handler.createNotifier(uuid)
		if err != nil {
			return nil, err
		}
		ccMsg = &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_INIT, Payload: payload, Uuid: uuid}
		/** we need to send when actually in init state (see enterinitstate)
		if err = handler.ChatStream.Send(ccMsg); err != nil {
			notfy <- &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: []byte(fmt.Sprintf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)), Uuid: uuid}
			return notfy, fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
		}
		*/
		event = pb.ChaincodeMessage_INIT.String()
	} else {
		chaincodeLogger.Debug("sending READY")
		var payload []byte
		ccMsg = &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_READY, Payload: payload, Uuid: uuid}
		if err := handler.ChatStream.Send(ccMsg); err != nil {
			notfy <- &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_ERROR, Payload: []byte(fmt.Sprintf("Error sending %s: %s", pb.ChaincodeMessage_READY, err)), Uuid: uuid}
			return notfy, fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_READY, err)
		}
		event = pb.ChaincodeMessage_READY.String()
	}
	eventErr := handler.FSM.Event(event, ccMsg)
	if eventErr != nil {
		fmt.Printf("Failed to trigger FSM event READY : %s\n", eventErr)
	}

	return notfy, eventErr
}

// HandleMessage implementation of MessageHandler interface.  Peer's handling of Chaincode messages.
func (handler *Handler) HandleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s in state %s", msg.Type, handler.FSM.Current())

	//QUERY_COMPLETED message can happen ONLY for Transaction_QUERY (stateless)
	if msg.Type == pb.ChaincodeMessage_QUERY_COMPLETED {
		chaincodeLogger.Debug("HandleMessage- QUERY_COMPLETED for uuid:%s. Notify", msg.Uuid)
		handler.notify(msg)
		return nil
	} else if msg.Type == pb.ChaincodeMessage_QUERY_ERROR {
		chaincodeLogger.Debug("HandleMessage- QUERY_ERROR (%s) for uuid:%s. Notify", string(msg.Payload), msg.Uuid)
		handler.notify(msg)
		return nil
	}
	if handler.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Chaincode handler validator FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), handler.FSM.Current())
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

func (handler *Handler) deleteNotifier(uuid string) {
	handler.Lock()
	if handler.responseNotifiers != nil {
		delete(handler.responseNotifiers, uuid)
	}
	handler.Unlock()
}
func (handler *Handler) createNotifier(uuid string) (chan *pb.ChaincodeMessage, error) {
	if handler.responseNotifiers == nil {
		return nil, fmt.Errorf("cannot create notifier for Uuid:%s", uuid)
	}
	handler.Lock()
	defer handler.Unlock()
	if handler.responseNotifiers[uuid] != nil {
		return nil, fmt.Errorf("Uuid:%s exists", uuid)
	}
	handler.responseNotifiers[uuid] = make(chan *pb.ChaincodeMessage, 1)
	return handler.responseNotifiers[uuid], nil
}

func (handler *Handler) sendExecuteMessage(msg *pb.ChaincodeMessage) (chan *pb.ChaincodeMessage, error) {
	notfy, err := handler.createNotifier(msg.Uuid)
	if err != nil {
		return nil, err
	}
	if err := handler.ChatStream.Send(msg); err != nil {
		handler.deleteNotifier(msg.Uuid)
		return nil, fmt.Errorf("SendMessage error sending %s(%s)", msg.Uuid, err)
	}

	if msg.Type.String() == pb.ChaincodeMessage_TRANSACTION.String() {
		eventErr := handler.FSM.Event(msg.Type.String(), msg)
		if eventErr != nil {
			chaincodeLogger.Debug("Failed to trigger FSM event TRANSACTION: %s", eventErr)
		}

	}
	return notfy, nil
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
