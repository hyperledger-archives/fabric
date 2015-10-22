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
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/net/context"
	"google.golang.org/grpc/grpclog"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	"github.com/openblockchain/obc-peer/openchain/consensus/pbft"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

var validatorLogger = logging.MustGetLogger("validator")

type Validator interface {
	Broadcast(*pb.OpenchainMessage) error
	GetHandler(stream PeerChatStream) MessageHandler
	IsLeader() bool
}

type SimpleValidator struct {
	validatorStreams map[string]MessageHandler
	peerStreams      map[string]MessageHandler
	leaderHandler    MessageHandler // handler representing either side of stream
	isLeader         bool
}

func (v *SimpleValidator) IsLeader() bool {
	return v.isLeader
}

func (v *SimpleValidator) Broadcast(msg *pb.OpenchainMessage) error {
	validatorLogger.Debug("Broadcasting OpenchainMessage of type: %s", msg.Type)
	err := v.leaderHandler.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("Error broadcasting msg of type: %s", msg.Type)
	}
	return nil
}

func (v *SimpleValidator) GetHandler(stream PeerChatStream) MessageHandler {
	if v.isLeader {
		v.leaderHandler = NewValidatorFSM(v, "", stream)
		return v.leaderHandler
	}
	return NewValidatorFSM(v, "", stream)
}

func (v *SimpleValidator) chatWithLeader(peerAddress string) error {

	var errFromChat error = nil
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return errors.New(fmt.Sprintf("Error connecting to leader at address=%s:  %s", peerAddress, err))
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	v.leaderHandler = v.GetHandler(stream)

	if err != nil {
		return errors.New(fmt.Sprintf("Error chatting with leader at address=%s:  %s", peerAddress, err))
	} else {
		defer stream.CloseSend()
		stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO})
		waitc := make(chan struct{})
		go func() {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					errFromChat = errors.New(fmt.Sprintf("Error sending transactions to peer address=%s, received EOF when expecting %s", peerAddress, pb.OpenchainMessage_DISC_HELLO))
					close(waitc)
					return
				}
				if err != nil {
					grpclog.Fatalf("Failed to receive a DiscoverMessage from server : %v", err)
				}
				// Call FSM.HandleMessage()
				err = v.leaderHandler.HandleMessage(in)
				if err != nil {
					validatorLogger.Error("Error handling message: %s", err)
					return
				}

				// 	if in.Type == pb.OpenchainMessage_DISC_HELLO {
				// 		peerLogger.Debug("Received %s message as expected, sending transactions...", in.Type)
				// 		payload, err := proto.Marshal(transactionsMessage)
				// 		if err != nil {
				// 			errFromChat = errors.New(fmt.Sprintf("Error marshalling transactions to peer address=%s:  %s", peerAddress, err))
				// 			close(waitc)
				// 			return
				// 		}
				// 		stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CHAIN_TRANSACTIONS, Payload: payload})
				// 		peerLogger.Debug("Transactions sent to peer address: %s", peerAddress)
				// 		close(waitc)
				// 		return
				// 	} else {
				// 		peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
				// 		close(waitc)
				// 		return
				// 	}
			}
		}()
		<-waitc
		return nil
	}
}

func NewSimpleValidator(isLeader bool) (Validator, error) {
	validator := &SimpleValidator{}
	// Only perform if NOT the leader
	if !viper.GetBool("peer.consensus.leader.enabled") {
		leaderAddress := viper.GetString("peer.consensus.leader.address")
		validatorLogger.Debug("Creating client to Peer (Leader) with address: %s", leaderAddress)
		go validator.chatWithLeader(leaderAddress)
	}
	validator.isLeader = isLeader
	return validator, nil
}

type ValidatorFSM struct {
	To             string
	ChatStream     PeerChatStream
	FSM            *fsm.FSM
	PeerFSM        *PeerFSM
	validator      Validator
	storedRequests map[string]*pbft.PBFT
}

func NewValidatorFSM(parent Validator, to string, peerChatStream PeerChatStream) *ValidatorFSM {
	v := &ValidatorFSM{
		To:         to,
		ChatStream: peerChatStream,
		validator:  parent,
	}
	v.storedRequests = make(map[string]*pbft.PBFT)

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pbft.PBFT_REQUEST.String(), Src: []string{"established"}, Dst: "prepare_result_sent"},
			{Name: pbft.PBFT_PRE_PREPARE.String(), Src: []string{"established"}, Dst: "prepare_result_sent"},
			{Name: pbft.PBFT_PREPARE_RESULT.String(), Src: []string{"prepare_result_sent"}, Dst: "commit_result_sent"},
			{Name: pbft.PBFT_COMMIT_RESULT.String(), Src: []string{"commit_result_sent"}, Dst: "committed_block"},
		},
		fsm.Callbacks{
			"before_" + pb.OpenchainMessage_DISC_HELLO.String():         func(e *fsm.Event) { v.beforeHello(e) },
			"before_" + pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(): func(e *fsm.Event) { v.beforeChainTransactions(e) },
			"before_" + pbft.PBFT_REQUEST.String():                      func(e *fsm.Event) { v.beforeRequest(e) },
			"before_" + pbft.PBFT_PRE_PREPARE.String():                  func(e *fsm.Event) { v.beforePrePrepareResult(e) },
			"before_" + pbft.PBFT_PREPARE_RESULT.String():               func(e *fsm.Event) { v.beforePrepareResult(e) },
			"before_" + pbft.PBFT_COMMIT_RESULT.String():                func(e *fsm.Event) { v.beforeCommitResult(e) },
		},
	)
	// v.FSM = fsm.NewFSM(
	// 	"created",
	// 	fsm.Events{
	// 		{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
	// 		{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
	// 	},
	// 	fsm.Callbacks{
	// 		"enter_state":                                               func(e *fsm.Event) { v.enterState(e) },
	// 		"before_" + pb.OpenchainMessage_DISC_HELLO.String():         func(e *fsm.Event) { v.beforeHello(e) },
	// 		"before_" + pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(): func(e *fsm.Event) { v.beforeChainTransactions(e) },
	// 	},
	// )

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
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {

	}
	msg := e.Args[0].(*pb.OpenchainMessage)

	//
	//proto.Marshal()
	uuid, err := util.GenerateUUID()
	if err != nil {
		e.Cancel(fmt.Errorf("Error generating UUID: %s", err))
		return
	}
	// For now unpack the lone transaction and send as the payload
	transactionsMessage := &pb.TransactionsMessage{}
	err = proto.Unmarshal(msg.Payload, transactionsMessage)
	if err != nil {
		e.Cancel(fmt.Errorf("Error generating UUID: %s", err))
		return
	}
	transactionToSend := transactionsMessage.Transactions[0]
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
	validatorLogger.Debug("Getting ready to create CONSENSUS from this message type : %s", msg.Type)
	v.validator.Broadcast(newMsg)
	// if err := v.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
	// 	e.Cancel(err)
	// }
}

func (v *ValidatorFSM) when(stateToCheck string) bool {
	return v.FSM.Is(stateToCheck)
}

func (v *ValidatorFSM) HandleMessage(msg *pb.OpenchainMessage) error {
	validatorLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
	if viper.GetBool("peer.consensus.leader.enabled") {
		validatorLogger.Debug("storedRequests length = %d", len(v.storedRequests))
	}
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		pbft := &pbft.PBFT{}
		err := proto.Unmarshal(msg.Payload, pbft)
		if err != nil {
			return fmt.Errorf("Error unpacking Payload from %s message: %s", pb.OpenchainMessage_CONSENSUS, err)
		}
		validatorLogger.Debug("Handling pbft type: %s", pbft.Type)
		if v.FSM.Cannot(pbft.Type.String()) {
			return fmt.Errorf("Validator FSM cannot handle CONSENSUS message (%s) with payload size (%d) while in state: %s", pbft.Type.String(), len(pbft.Payload), v.FSM.Current())
		}
		err = v.FSM.Event(pbft.Type.String(), pbft)
		validatorLogger.Debug("Processed pbft event: %s, current state: %s", pbft.Type, v.FSM.Current())

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

func (v *ValidatorFSM) SendMessage(msg *pb.OpenchainMessage) error {
	validatorLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := v.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}

func (v *ValidatorFSM) beforeRequest(e *fsm.Event) {
	validatorLogger.Debug("Handling beforeRequest for event: %s", e.Event)
	// Check incoming message.
	if _, ok := e.Args[0].(*pbft.PBFT); !ok {
		e.Cancel(fmt.Errorf("Unexpected message received."))
		return
	}
	message := e.Args[0].(*pbft.PBFT)
	// Calculate hash.
	hash := util.ComputeCryptoHash(message.Payload)
	//hashString := string(hash[:])
	hashString := base64.StdEncoding.EncodeToString(hash)
	// Extract Request message.
	// newRequest := &pbft.Request{}
	// err := proto.Unmarshal(message.Payload, newRequest)
	// if err != nil {
	// 	e.Cancel(fmt.Errorf("Unmarshaling error: %v", err))
	// 	return
	// }
	// Store in map.
	validatorLogger.Debug("Before storing requests in map for event: %s", e.Event)
	if _, ok := v.storedRequests[hashString]; ok {
		e.Cancel(fmt.Errorf("Message (hash: %v) already stored,", hashString))
		return
	} else {
		v.storedRequests[hashString] = message
		validatorLogger.Debug("Stored newRequest in map (length %d) under key (%s), value isNil (%v)", len(v.storedRequests), hashString, message == nil)
	}
	if v.validator.IsLeader() {
		v.broadcastPrePrepareAndPrepare(e)
	} else {
		// Cancel transition if NOT leader
		validatorLogger.Debug("Cancelling transition of non-leader for event: %s", e.Event)
		e.Cancel()
	}

}

func (v *ValidatorFSM) beforePrePrepareResult(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pbft.PBFT); !ok {
		e.Cancel(fmt.Errorf("Unexpected message received."))
		return
	}
	msg := e.Args[0].(*pbft.PBFT)
	if v.validator.IsLeader() {
		validatorLogger.Debug("Cancelling transition for leader due to event: %s", e.Event)
		e.Cancel()
		return
	}
	validatorLogger.Debug("Need to Execute transactions in PRE_PREPARE using Murali's code after msg of type: %s", msg.Type)
	// TODO: Various checks should go here -- skipped for now.
	// TODO: Execute transactions in PRE_PREPARE using Murali's code.
	// TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:PREPARE_RESULT message.
}

func (v *ValidatorFSM) beforePrepareResult(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pbft.PBFT); !ok {
		e.Cancel(fmt.Errorf("Unexpected message received."))
		return
	}
	msg := e.Args[0].(*pbft.PBFT)
	validatorLogger.Debug("TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:COMMIT_RESULT message after msg of type: %s", msg.Type)
	// TODO: Various checks should go here -- skipped for now.
	// TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:COMMIT_RESULT message.
}

func (v *ValidatorFSM) beforeCommitResult(e *fsm.Event) {
	// Check incoming message.
	if _, ok := e.Args[0].(*pbft.PBFT); !ok {
		e.Cancel(fmt.Errorf("Unexpected message received."))
		return
	}
	msg := e.Args[0].(*pbft.PBFT)
	validatorLogger.Debug("TODO: Commit referenced transactions to blockchain after msg of type: %s", msg.Type)
	// TODO: Various checks should go here -- skipped for now.
	// TODO: Commit referenced transactions to blockchain.
}

func (v *ValidatorFSM) broadcastPrePrepareAndPrepare(e *fsm.Event) {
	// How many Requests currently in map?
	storedCount := len(v.storedRequests)
	if storedCount >= 2 {
		// First: Broadcast OpenchainMessage_CONSENSUS with PAYLOAD: PRE_PREPARE.
		pbfts := make([]*pbft.PBFT, 0)
		for _, pbft := range v.storedRequests {
			pbfts = append(pbfts, pbft)
		}
		// Marshal the array of Request messages.
		data1, err := proto.Marshal(&pbft.PBFTArray{Pbfts: pbfts})
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
		validatorLogger.Debug("Getting ready to create CONSENSUS from this message type : %s", newMsg.Type)
		v.validator.Broadcast(newMsg)
		// TODO: Various checks should go here -- skipped for now.
		// TODO: Execute transactions in PRE_PREPARE using Murali's code.
		// TODO: Create OpenchainMessage_CONSENSUS message where PAYLOAD is a PHASE:PREPARE_RESULT message.
	} else {
		e.Cancel(fmt.Errorf("Leader remains in established state."))
	}
}
