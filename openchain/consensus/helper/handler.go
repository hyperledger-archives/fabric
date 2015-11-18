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
	"github.com/openblockchain/obc-peer/openchain/consensus/controller"
	"github.com/openblockchain/obc-peer/openchain/peer"

	pb "github.com/openblockchain/obc-peer/protos"
)

var handlerLogger = logging.MustGetLogger("consensus")

// ConsensusHandler is an object to hande consensus messages. It also implements
// the consensus CPI
type ConsensusHandler struct {
	Coordinator peer.MessageHandlerCoordinator
	ChatStream  peer.ChatStream
	doneChan    chan bool
	PeerHandler peer.MessageHandler
	consenter   Consenter
}

// NewConsensusHandler constructs a new Noops MessageHandler
func NewConsensusHandler(coord peer.MessageHandlerCoordinator,
	stream peer.ChatStream, initiatedStream bool,
	next peer.MessageHandler) (peer.MessageHandler, error) {
	d := &ConsensusHandler{
		ChatStream:  stream,
		Coordinator: coord,
		PeerHandler: next,
	}
	var err error
	d.PeerHandler, err = peer.NewPeerHandler(coord, stream, initiatedStream, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating PeerHandler: %s", err)
	}

	// Set consenter
	d.consenter = controller.NewConsenter(NewHelper(coord))

	d.doneChan = make(chan bool)

	return d, nil
}

// HandleMessage handles the Openchain messages for the Peer.
func (i *ConsensusHandler) HandleMessage(msg *pb.OpenchainMessage) error {
	if msg.Type == pb.OpenchainMessage_CONSENSUS ||
		msg.Type == pb.OpenchainMessage_REQUEST {
		return i.consenter.RecvMsg(msg)
	}
	if msg.Type == pb.OpenchainMessage_QUERY {
		return handleQuery(msg)
	}

	if handlerLogger.IsEnabledFor(logging.DEBUG) {
		handlerLogger.Debug("Did not handle message of type %s, passing on to next MessageHandler", msg.Type)
	}
	return i.PeerHandler.HandleMessage(msg)
}

func handleQuery(msg *pb.OpenchainMessage) (*pb.OpenchainMessage, error) {
	// TODO: Exec query and return result to the caller as a response msg
	return nil, nil
}

// SendMessage sends a message to the remote PEER through the stream
func (i *ConsensusHandler) SendMessage(msg *pb.OpenchainMessage) error {
	peerLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := i.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}

// Stop stops this MessageHandler, which then delegates to the contained PeerHandler to stop (and thus deregister this Peer)
func (i *ConsensusHandler) Stop() error {
	// Deregister the handler
	err := i.PeerHandler.Stop()
	i.doneChan <- true
	if err != nil {
		return fmt.Errorf("Error stopping ConsensusHandler handler: %s", err)
	}
	return nil
}

// To return the PeerEndpoint this Handler is connected to by delegating to the contained PeerHandler.
func (i *ConsensusHandler) To() (pb.PeerEndpoint, error) {
	return i.PeerHandler.To()
}
