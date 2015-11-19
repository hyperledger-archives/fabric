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

package peer

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("noops")

// Noops the Noops structure associated with the Noops MessageHandler
type Noops struct {
	Coordinator MessageHandlerCoordinator
	ChatStream  ChatStream
	doneChan    chan bool
	PeerHandler MessageHandler
}

// NewNoopsHandler constructs a new Noops MessageHandler
func NewNoopsHandler(coord MessageHandlerCoordinator, stream ChatStream, initiatedStream bool, next MessageHandler) (MessageHandler, error) {
	d := &Noops{
		ChatStream:  stream,
		Coordinator: coord,
		PeerHandler: next,
	}
	var err error
	d.PeerHandler, err = NewPeerHandler(coord, stream, initiatedStream, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating PeerHandler: %s", err)
	}
	d.doneChan = make(chan bool)

	return d, nil
}

// HandleMessage handles the Openchain messages for the Peer.
func (i *Noops) HandleMessage(msg *pb.OpenchainMessage) error {
	logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
	if msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION || msg.Type == pb.OpenchainMessage_CHAIN_QUERY {
		if msg.Type == pb.OpenchainMessage_CHAIN_QUERY {
			//TODO exec query directly here
			return nil
		}
		msg.Type = pb.OpenchainMessage_CONSENSUS
		logger.Debug("Broadcasting %s", msg.Type)
		// broadcast to others so they can exec the tx
		i.Coordinator.Broadcast(msg)

		// WARNING: We might end up getting the same message sent back to us
		// due to Byzantine. We ignore this case for the no-ops consensus
		return nil
	}
	// We process the message if it is OpenchainMessage_CONSENSUS
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		txs := &pb.TransactionBlock{}
		err := proto.Unmarshal(msg.Payload, txs)
		if err != nil {
			err = fmt.Errorf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type)
			return err
		}
		_, errs := chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs.GetTransactions())
		if errs != nil {
			err = fmt.Errorf("Fail to execute transactions: %s", err)
			return err
		}
		return nil
	}

	logger.Debug("Did not handle message of type %s, passing on to next MessageHandler", msg.Type)
	return i.PeerHandler.HandleMessage(msg)
}

// SendMessage sends a message to the remote PEER through the stream
func (i *Noops) SendMessage(msg *pb.OpenchainMessage) error {
	peerLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := i.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}

// Stop stops this MessageHandler, which then delegates to the contained PeerHandler to stop (and thus deregister this Peer)
func (i *Noops) Stop() error {
	// Deregister the handler
	err := i.PeerHandler.Stop()
	i.doneChan <- true
	if err != nil {
		return fmt.Errorf("Error stopping Noops handler: %s", err)
	}
	return nil
}

// To return the PeerEndpoint this Handler is connected to by delegating to the contained PeerHandler.
func (i *Noops) To() (pb.PeerEndpoint, error) {
	return i.PeerHandler.To()
}
