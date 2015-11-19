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

package helper

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/controller"
	"github.com/openblockchain/obc-peer/openchain/peer"

	pb "github.com/openblockchain/obc-peer/protos"
)

// ConsensusHandler handles consensus messages.
// It also implements the CPI.
type ConsensusHandler struct {
	chatStream  peer.ChatStream
	consenter   consensus.Consenter
	coordinator peer.MessageHandlerCoordinator
	done        chan bool
	peerHandler peer.MessageHandler
}

// NewConsensusHandler constructs a new MessageHandler for the plugin.
func NewConsensusHandler(coord peer.MessageHandlerCoordinator,
	stream peer.ChatStream, initiatedStream bool,
	next peer.MessageHandler) (peer.MessageHandler, error) {
	handler := &ConsensusHandler{
		chatStream:  stream,
		coordinator: coord,
		peerHandler: next,
	}

	var err error
	handler.peerHandler, err = peer.NewPeerHandler(coord, stream, initiatedStream, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating PeerHandler: %s", err)
	}

	handler.consenter = controller.NewConsenter(NewHelper(coord))
	handler.done = make(chan bool)

	return handler, nil
}

// HandleMessage handles the incoming Openchain messages for the Peer.
func (handler *ConsensusHandler) HandleMessage(msg *pb.OpenchainMessage) error {
	if msg.Type == pb.OpenchainMessage_CONSENSUS || msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		//if transaction return Uuid (we have to return *something* to the blocking grpc client)
		if msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
			var response *pb.Response
			t := &pb.Transaction{}
			err := proto.Unmarshal(msg.Payload, t)
			if err != nil {
				response =  &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type))}
			} else {
				response =  &pb.Response{Status: pb.Response_SUCCESS, Msg: []byte(t.Uuid)}
			}
			payload, err := proto.Marshal(response)
			handler.SendMessage(&pb.OpenchainMessage{Type: pb.OpenchainMessage_RESPONSE, Payload: payload})
		}
		return handler.consenter.RecvMsg(msg)
	}
	if msg.Type == pb.OpenchainMessage_CHAIN_QUERY {
		var response *pb.Response
		t := &pb.Transaction{}
		err := proto.Unmarshal(msg.Payload, t)
		if err != nil {
			response =  &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type))}
		} else {
			b, err := chaincode.Execute(context.Background(), chaincode.GetChain(chaincode.DefaultChain), t)
			if err != nil  {
				response =  &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error:%s", err))}
			} else {
				response =  &pb.Response{Status: pb.Response_SUCCESS, Msg: b}
			}
		}
		payload, _ := proto.Marshal(response)
		handler.SendMessage(&pb.OpenchainMessage{Type: pb.OpenchainMessage_RESPONSE, Payload: payload})
		return nil
	}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Did not handle message of type %s, passing on to next MessageHandler", msg.Type)
	}
	return handler.peerHandler.HandleMessage(msg)
}

// SendMessage sends a message to the remote Peer through the stream.
func (handler *ConsensusHandler) SendMessage(msg *pb.OpenchainMessage) error {
	logger.Debug("Sending to stream a message of type: %s", msg.Type)
	err := handler.chatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error sending message through ChatStream: %s", err)
	}
	return nil
}

// Stop stops this MessageHandler, which then delegates to the contained PeerHandler to stop (and thus deregister this Peer).
func (handler *ConsensusHandler) Stop() error {
	err := handler.peerHandler.Stop() // deregister the handler
	handler.done <- true
	if err != nil {
		return fmt.Errorf("Error stopping ConsensusHandler: %s", err)
	}
	return nil
}

// To returns the PeerEndpoint this Handler is connected to by delegating to the contained PeerHandler
func (handler *ConsensusHandler) To() (pb.PeerEndpoint, error) {
	return handler.peerHandler.To()
}
