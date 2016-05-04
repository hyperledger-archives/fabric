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
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/consensus/util"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/peer"

	pb "github.com/hyperledger/fabric/protos"
)

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/handler")
}

const (
	DefaultConsensusQueueSize int = 1000
)

// ConsensusHandler handles consensus messages.
// It also implements the Stack.
type ConsensusHandler struct {
	//	*Helper
	consenterChan chan *util.Message
	coordinator   peer.MessageHandlerCoordinator
	peerHandler   peer.MessageHandler
}

// NewConsensusHandler constructs a new MessageHandler for the plugin.
// Is instance of peer.HandlerFactory
func NewConsensusHandler(coord peer.MessageHandlerCoordinator,
	stream peer.ChatStream, initiatedStream bool,
	next peer.MessageHandler) (peer.MessageHandler, error) {
	handler := &ConsensusHandler{
		coordinator: coord,
		peerHandler: next,
	}

	var err error
	handler.peerHandler, err = peer.NewPeerHandler(coord, stream, initiatedStream, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating PeerHandler: %s", err)
	}

	consensusQueueSize := viper.GetInt("peer.validator.consensus.buffersize")

	if consensusQueueSize <= 0 {
		logger.Error("peer.validator.consensus.buffersize is set to %d, but this must be a positive integer, defaulting to %d", consensusQueueSize, DefaultConsensusQueueSize)
		consensusQueueSize = DefaultConsensusQueueSize
	}

	pe, _ := handler.peerHandler.To()

	handler.consenterChan = make(chan *util.Message, consensusQueueSize)
	getEngineImpl().consensusFan.RegisterChannel(pe.ID, handler.consenterChan)

	return handler, nil
}

// HandleMessage handles the incoming Fabric messages for the Peer
func (handler *ConsensusHandler) HandleMessage(msg *pb.Message) error {
	if msg.Type == pb.Message_CONSENSUS {
		senderPE, _ := handler.peerHandler.To()
		select {
		case handler.consenterChan <- &util.Message{
			Msg:    msg,
			Sender: senderPE.ID,
		}:
			return nil
		default:
			err := fmt.Errorf("Message channel for %v full, rejecting", senderPE.ID)
			logger.Error("Failed to queue consensus message because: %v", err)
			return err
		}
	} else if msg.Type == pb.Message_CHAIN_TRANSACTION {
		tx := &pb.Transaction{}
		err := proto.Unmarshal(msg.Payload, tx)
		if err == nil {
			if tx.Type == pb.Transaction_CHAINCODE_QUERY {
				return handler.doChainQuery(tx)
			}
		  return handler.doChainTransaction(msg, tx)
		}
	}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Did not handle message of type %s, passing on to next MessageHandler", msg.Type)
	}
	return handler.peerHandler.HandleMessage(msg)
}

func (handler *ConsensusHandler) doChainTransaction(msg *pb.Message, tx *pb.Transaction) error {
	var response *pb.Response
	defer func() {
		payload, _ := proto.Marshal(response)
		handler.SendMessage(&pb.Message{Type: pb.Message_RESPONSE, Payload: payload})
	}()

	// Verify transaction signature if security is enabled
	secHelper := handler.coordinator.GetSecHelper()
	if nil != secHelper {
		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Verifying transaction signature %s", tx.Uuid)
		}
		var err error
		if tx, err = secHelper.TransactionPreValidation(tx); nil != err {
			response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(err.Error())}
			logger.Debug("Failed to verify transaction %v", err)
			return err
		}
	}

	// Pass the message to the plugin handler (ie PBFT)
	selfPE, _ := handler.coordinator.GetPeerEndpoint() // we are the validator introducting this tx into the system
	select {
	case handler.consenterChan <- &util.Message{
		Msg:    msg,
		Sender: selfPE.ID,
	}:
		logger.Debug("Queued new transaction from %v to channel", selfPE.ID)
		response = &pb.Response{Status: pb.Response_SUCCESS, Msg: []byte(tx.Uuid)}
		return nil
	default:
		err := fmt.Errorf("Message channel for %v full, rejecting", selfPE.ID)
		logger.Error("Failed to queue new transaction because %v: ", err)
		response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(err.Error())}
		return err
	}
}

func (handler *ConsensusHandler) doChainQuery(tx *pb.Transaction) error {
	var response *pb.Response
	var err error
	// Verify transaction signature if security is enabled
	secHelper := handler.coordinator.GetSecHelper()
	if nil != secHelper {
		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Verifying transaction signature %s", tx.Uuid)
		}
		if tx, err = secHelper.TransactionPreValidation(tx); nil != err {
			response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(err.Error())}
			logger.Debug("Failed to verify transaction %v", err)
		}

	}
	// execute if response nil (ie, no error)
	if nil == response {
		// The secHelper is set during creat ChaincodeSupport, so we don't need this step
		// cxt := context.WithValue(context.Background(), "security", secHelper)
		cxt := context.Background()
		result, err := chaincode.Execute(cxt, chaincode.GetChain(chaincode.DefaultChain), tx)
		if err != nil {
			response = &pb.Response{Status: pb.Response_FAILURE,
				Msg: []byte(fmt.Sprintf("Error:%s", err))}
		} else {
			response = &pb.Response{Status: pb.Response_SUCCESS, Msg: result}
		}
	}
	payload, _ := proto.Marshal(response)
	handler.SendMessage(&pb.Message{Type: pb.Message_RESPONSE, Payload: payload})
	return nil
}

// SendMessage sends a message to the remote Peer through the stream
func (handler *ConsensusHandler) SendMessage(msg *pb.Message) error {
	logger.Debug("Sending to stream a message of type: %s", msg.Type)
	// hand over the message to the peerHandler to serialize
	err := handler.peerHandler.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("Error sending message through ChatStream: %s", err)
	}
	return nil
}

// Stop stops this MessageHandler, which then delegates to the contained PeerHandler to stop (and thus deregister this Peer)
func (handler *ConsensusHandler) Stop() error {
	err := handler.peerHandler.Stop() // deregister the handler
	if err != nil {
		return fmt.Errorf("Error stopping ConsensusHandler: %s", err)
	}
	return nil
}

// To returns the PeerEndpoint this Handler is connected to by delegating to the contained PeerHandler
func (handler *ConsensusHandler) To() (pb.PeerEndpoint, error) {
	return handler.peerHandler.To()
}

// RequestBlocks returns the current sync block
func (handler *ConsensusHandler) RequestBlocks(syncBlockRange *pb.SyncBlockRange) (<-chan *pb.SyncBlocks, error) {
	return handler.peerHandler.RequestBlocks(syncBlockRange)
}

// RequestStateSnapshot returns the current state
func (handler *ConsensusHandler) RequestStateSnapshot() (<-chan *pb.SyncStateSnapshot, error) {
	return handler.peerHandler.RequestStateSnapshot()
}

// RequestStateDeltas returns state deltas for a block range
func (handler *ConsensusHandler) RequestStateDeltas(syncBlockRange *pb.SyncBlockRange) (<-chan *pb.SyncStateDeltas, error) {
	return handler.peerHandler.RequestStateDeltas(syncBlockRange)
}
