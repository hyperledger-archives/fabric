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

package leader

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"

	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/consensus/pbft"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

var leaderLogger = logging.MustGetLogger("validator")

type LeaderFSM struct {
	To             string
	ChatStream     openchain.PeerChatStream
	FSM            *fsm.FSM
	PeerFSM        *openchain.PeerFSM
	validator      openchain.Validator
	storedRequests map[string]*pbft.Request
}

func NewLeaderFSM(parent openchain.Validator, to string, peerChatStream openchain.PeerChatStream) *LeaderFSM {
	v := &LeaderFSM{
		To:         to,
		ChatStream: peerChatStream,
		validator:  parent,
	}

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pbft.PBFT_REQUEST.String(), Src: []string{"established"}, Dst: "prepare_result_sent"},
			{Name: pbft.PBFT_PREPARE_RESULT.String(), Src: []string{"prepare_result_sent"}, Dst: "commit_result_sent"},
			{Name: pbft.PBFT_COMMIT_RESULT.String(), Src: []string{"commit_result_sent"}, Dst: "committed_block"},
		},
		fsm.Callbacks{
			"before_" + pb.OpenchainMessage_DISC_HELLO.String():         func(e *fsm.Event) { v.beforeHello(e) },
			"before_" + pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(): func(e *fsm.Event) { v.beforeChainTransactions(e) },
			"before_" + pbft.PBFT_REQUEST.String():                      func(e *fsm.Event) { v.beforeRequest(e) },
			"before_" + pbft.PBFT_PREPARE_RESULT.String():               func(e *fsm.Event) { v.beforePrepareResult(e) },
			"before_" + pbft.PBFT_COMMIT_RESULT.String():                func(e *fsm.Event) { v.beforeCommitResult(e) },
		},
	)

	return v
}

func (v *LeaderFSM) enterState(e *fsm.Event) {
	leaderLogger.Debug("The leader's bi-directional stream to %s is %s, from event %s\n", v.To, e.Dst, e.Event)
}

func (v *LeaderFSM) beforeHello(e *fsm.Event) {
	leaderLogger.Debug("Sending back %s", pb.OpenchainMessage_DISC_HELLO.String())
	if err := v.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
		e.Cancel(err)
	}
}

func (v *LeaderFSM) beforeChainTransactions(e *fsm.Event) {
	leaderLogger.Debug("Sending broadcast to all validators upon receipt of %s", pb.OpenchainMessage_DISC_HELLO.String())
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {

	}
	msg := e.Args[0].(*pb.OpenchainMessage)
	uuid, err := util.GenerateUUID()
	if err != nil {
		e.Cancel(fmt.Errorf("Error generating UUID: %s", err))
		return
	}
	// For now unpack the lone transaction and send as the payload
	transactionBlock := &pb.TransactionBlock{}
	err = proto.Unmarshal(msg.Payload, transactionBlock)
	if err != nil {
		e.Cancel(fmt.Errorf("Error generating UUID: %s", err))
		return
	}
	transactionToSend := transactionBlock.Transactions[0]
	data, err := proto.Marshal(transactionToSend)
	if err != nil {
		e.Cancel(fmt.Errorf("Error marshalling transaction to PBFT struct: %s", err))
		return
	}
	pbftData, err := proto.Marshal(&pbft.PBFT{Type: pbft.PBFT_REQUEST, ID: uuid, Payload: data})
	if err != nil {
		e.Cancel(fmt.Errorf("Error marshalling pbft: %s", err))
		return
	}
	newMsg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: pbftData}
	leaderLogger.Debug("Getting ready to create CONSENSUS from this message type : %s", msg.Type)
	v.validator.Broadcast(newMsg)
}

func (v *LeaderFSM) when(stateToCheck string) bool {
	return v.FSM.Is(stateToCheck)
}

func (v *LeaderFSM) HandleMessage(msg *pb.OpenchainMessage) error {
	leaderLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)

	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		pbft := &pbft.PBFT{}
		err := proto.Unmarshal(msg.Payload, pbft)
		if err != nil {
			return fmt.Errorf("Error unpacking Payload from %s message: %s", pb.OpenchainMessage_CONSENSUS, err)
		}
		if v.FSM.Cannot(pbft.Type.String()) {
			return fmt.Errorf("Validator FSM cannot handle CONSENSUS message (%s) with payload size (%d) while in state: %s", pbft.Type.String(), len(pbft.Payload), v.FSM.Current())
		}
		v.FSM.Event(pbft.Type.String(), pbft)
		if err != nil {
			if _, ok := err.(*fsm.NoTransitionError); !ok {
				// Only allow NoTransitionError's, all others are considered true error.
				return fmt.Errorf("Validator FSM failed while handling CONSENSUS message (%s): current state: %s, error: %s", pbft.Type.String(), v.FSM.Current(), err)
				//t.Error("expected only 'NoTransitionError'")
			}
		}
	} else {
		if v.FSM.Cannot(msg.Type.String()) {
			return fmt.Errorf("Validator FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), v.FSM.Current())
		}
		err := v.FSM.Event(msg.Type.String(), msg)
		if err != nil {
			if _, ok := err.(*fsm.NoTransitionError); !ok {
				// Only allow NoTransitionError's, all others are considered true error.
				return fmt.Errorf("Validator FSM failed while handling message (%s): current state: %s, error: %s", msg.Type.String(), v.FSM.Current(), err)
				//t.Error("expected only 'NoTransitionError'")
			}
		}
	}
	return nil
}

func (v *LeaderFSM) SendMessage(msg *pb.OpenchainMessage) error {
	leaderLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := v.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}

func (v *LeaderFSM) beforeRequest(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {
		e.Cancel(fmt.Errorf("Unexpected message received."))
		return
	}
	message := e.Args[0].(*pb.OpenchainMessage)
	// Calculate hash.
	hash := util.ComputeCryptoHash(message.Payload)
	hashString := string(hash[:])
	// Extract Request message.
	newRequest := &pbft.Request{}
	err := proto.Unmarshal(message.Payload, newRequest)
	if err != nil {
		e.Cancel(fmt.Errorf("Unmarshaling error: %v", err))
		return
	}
	// Store in map.
	if _, ok := v.storedRequests[hashString]; ok {
		e.Cancel(fmt.Errorf("Message (hash: %v) already stored,", hashString))
		return
	} else {
		v.storedRequests[hashString] = newRequest
	}
	// How many Requests currently in map?
	storedCount := len(v.storedRequests)
	if storedCount >= 2 {
		// First: Broadcast OpenchainMessage_CONSENSUS with PAYLOAD: PRE_PREPARE.
		requests := make([]*pbft.Request, storedCount)
		for _, request := range v.storedRequests {
			requests = append(requests, request)
		}
		// Marshal the array of Request messages.
		data1, err := proto.Marshal(&pbft.Requests{Requests: requests})
		if err != nil {
			e.Cancel(fmt.Errorf("Error marshalling array of Request messages: %s", err))
			return
		}
		// Marshal the PRE_PREPARE message.
		data2, err := proto.Marshal(&pbft.PBFT{Type: pbft.PBFT_PRE_PREPARE, ID: "nil", Payload: data1})
		if err != nil {
			e.Cancel(fmt.Errorf("Error marshalling PRE_PREPARE message: %s", err))
			return
		}
		// Create new consensus message.
		newMsg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: data2}
		leaderLogger.Debug("Getting ready to create CONSENSUS from this message type : %s", newMsg.Type)
		v.validator.Broadcast(newMsg)
		// TODO: Various checks should go here -- skipped for now.
		// TODO: Execute transactions in PRE_PREPARE using Murali's code.
		// TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:PREPARE_RESULT message.
	} else {
		e.Cancel(fmt.Errorf("Leader remains in established state."))
	}

}

func (v *LeaderFSM) beforePrepareResult(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {

	}
	msg := e.Args[0].(*pb.OpenchainMessage)
	leaderLogger.Debug("TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:COMMIT_RESULT message after msg of type: %s", msg.Type)
	// TODO: Various checks should go here -- skipped for now.
	// TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:COMMIT_RESULT message.
}

func (v *LeaderFSM) beforeCommitResult(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {

	}
	msg := e.Args[0].(*pb.OpenchainMessage)
	leaderLogger.Debug("TODO: Commit referenced transactions to blockchain after msg of type: %s", msg.Type)
	// TODO: Various checks should go here -- skipped for now.
	// TODO: Commit referenced transactions to blockchain.
}
