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
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt/state"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

const defaultTimeout = time.Second * 3

// Peer provides interface for a peer
type Peer interface {
	GetPeerEndpoint() (*pb.PeerEndpoint, error)
	NewOpenchainDiscoveryHello() (*pb.OpenchainMessage, error)
}

// BlocksRetriever interface for retrieving blocks .
type BlocksRetriever interface {
	GetBlocks(*pb.SyncBlockRange) (<-chan *pb.SyncBlocks, error)
}

// BlockChainAccessor interface for retreiving blocks by block number
type BlockChainAccessor interface {
	GetBlockByNumber(blockNumber uint64) (*pb.Block, error)
}

// StateAccessor interface for retreiving blocks by block number
type StateAccessor interface {
	GetStateSnapshot() (*state.StateSnapshot, error)
}

// MessageHandler standard interface for handling Openchain messages.
type MessageHandler interface {
	BlocksRetriever
	HandleMessage(msg *pb.OpenchainMessage) error
	SendMessage(msg *pb.OpenchainMessage) error
	To() (pb.PeerEndpoint, error)
	Stop() error
}

// MessageHandlerCoordinator responsible for coordinating between the registered MessageHandler's
type MessageHandlerCoordinator interface {
	Peer
	SecurityAccessor
	BlockChainAccessor
	StateAccessor
	RegisterHandler(messageHandler MessageHandler) error
	DeregisterHandler(messageHandler MessageHandler) error
	Broadcast(*pb.OpenchainMessage) []error
	GetPeers() (*pb.PeersMessage, error)
	PeersDiscovered(*pb.PeersMessage) error
	ExecuteTransaction(transaction *pb.Transaction) *pb.Response
}

// ChatStream interface supported by stream between Peers
type ChatStream interface {
	Send(*pb.OpenchainMessage) error
	Recv() (*pb.OpenchainMessage, error)
}

// SecurityAccessor interface enables a Peer to hand out the crypto object for Peer
type SecurityAccessor interface {
	GetSecHelper() crypto.Peer
}

var peerLogger = logging.MustGetLogger("peer")

// NewPeerClientConnection Returns a new grpc.ClientConn to the configured local PEER.
func NewPeerClientConnection() (*grpc.ClientConn, error) {
	return NewPeerClientConnectionWithAddress(viper.GetString("peer.address"))
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// GetLocalAddress returns the address:port the local peer is operating on.  Affected by env:peer.addressAutoDetect
func GetLocalAddress() (peerAddress string, err error) {
	if viper.GetBool("peer.addressAutoDetect") {
		// Need to get the port from the peer.address setting, and append to the determined host IP
		_, port, err := net.SplitHostPort(viper.GetString("peer.address"))
		if err != nil {
			err = fmt.Errorf("Error auto detecting Peer's address: %s", err)
			return "", err
		}
		peerAddress = net.JoinHostPort(GetLocalIP(), port)
		peerLogger.Info("Auto detected peer address: %s", peerAddress)
	} else {
		peerAddress = viper.GetString("peer.address")
	}
	return
}

// GetPeerEndpoint returns the PeerEndpoint for this Peer instance.  Affected by env:peer.addressAutoDetect
func GetPeerEndpoint() (*pb.PeerEndpoint, error) {
	var peerAddress string
	var peerType pb.PeerEndpoint_Type
	peerAddress, err := GetLocalAddress()
	if err != nil {
		return nil, err
	}
	if viper.GetBool("peer.validator.enabled") {
		peerType = pb.PeerEndpoint_VALIDATOR
	} else {
		peerType = pb.PeerEndpoint_NON_VALIDATOR
	}
	return &pb.PeerEndpoint{ID: &pb.PeerID{Name: viper.GetString("peer.id")}, Address: peerAddress, Type: peerType}, nil
}

// NewPeerClientConnectionWithAddress Returns a new grpc.ClientConn to the configured local PEER.
func NewPeerClientConnectionWithAddress(peerAddress string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if viper.GetBool("peer.tls.enabled") {
		var sn string
		if viper.GetString("peer.tls.server-host-override") != "" {
			sn = viper.GetString("peer.tls.server-host-override")
		}
		var creds credentials.TransportAuthenticator
		if viper.GetString("peer.tls.cert.file") != "" {
			var err error
			creds, err = credentials.NewClientTLSFromFile(viper.GetString("peer.tls.cert.file"), sn)
			if err != nil {
				grpclog.Fatalf("Failed to create TLS credentials %v", err)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}
	opts = append(opts, grpc.WithTimeout(defaultTimeout))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial(peerAddress, opts...)
	if err != nil {
		return nil, err
	}
	return conn, err
}

type ledgerWrapper struct {
	sync.RWMutex
	ledger *ledger.Ledger
}

type handlerMap struct {
	sync.RWMutex
	m map[string]MessageHandler
}

// PeerImpl implementation of the Peer service
type PeerImpl struct {
	handlerFactory func(MessageHandlerCoordinator, ChatStream, bool, MessageHandler) (MessageHandler, error)
	handlerMap     *handlerMap
	ledgerWrapper  *ledgerWrapper
	secHelper      crypto.Peer
}

// NewPeerWithHandler returns a Peer which uses the supplied handler factory function for creating new handlers on new Chat service invocations.
func NewPeerWithHandler(handlerFact func(MessageHandlerCoordinator, ChatStream, bool, MessageHandler) (MessageHandler, error)) (*PeerImpl, error) {
	peer := new(PeerImpl)
	if handlerFact == nil {
		return nil, errors.New("Cannot supply nil handler factory")
	}
	peer.handlerFactory = handlerFact
	peer.handlerMap = &handlerMap{m: make(map[string]MessageHandler)}

	// Install security object for peer
	if viper.GetBool("security.enabled") {
		enrollID := viper.GetString("security.enrollID")
		enrollSecret := viper.GetString("security.enrollSecret")
		var err error
		if viper.GetBool("peer.validator.enabled") {
			peerLogger.Debug("Registering validator with enroll ID: %s", enrollID)
			if err = crypto.RegisterValidator(enrollID, nil, enrollID, enrollSecret); nil != err {
				return nil, err
			}
			peerLogger.Debug("Initializing validator with enroll ID: %s", enrollID)
			peer.secHelper, err = crypto.InitValidator(enrollID, nil)
			if nil != err {
				return nil, err
			}
		} else {
			peerLogger.Debug("Registering non-validator with enroll ID: %s", enrollID)
			if err = crypto.RegisterPeer(enrollID, nil, enrollID, enrollSecret); nil != err {
				return nil, err
			}
			peerLogger.Debug("Initializing non-validator with enroll ID: %s", enrollID)
			peer.secHelper, err = crypto.InitPeer(enrollID, nil)
			if nil != err {
				return nil, err
			}
		}
	}

	ledgerPtr, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Error constructing NewPeerWithHandler: %s", err)
	}
	peer.ledgerWrapper = &ledgerWrapper{ledger: ledgerPtr}
	go peer.chatWithPeer(viper.GetString("peer.discovery.rootnode"))
	return peer, nil
}

// Chat implementation of the the Chat bidi streaming RPC function
func (p *PeerImpl) Chat(stream pb.Peer_ChatServer) error {
	return p.handleChat(stream.Context(), stream, false)
}

// GetPeers returns the currently registered PeerEndpoints
func (p *PeerImpl) GetPeers() (*pb.PeersMessage, error) {
	p.handlerMap.Lock()
	defer p.handlerMap.Unlock()
	peers := []*pb.PeerEndpoint{}
	for _, msgHandler := range p.handlerMap.m {
		peerEndpoint, err := msgHandler.To()
		if err != nil {
			return nil, fmt.Errorf("Error Getting Peers: %s", err)
		}
		peers = append(peers, &peerEndpoint)
	}
	peersMessage := &pb.PeersMessage{Peers: peers}
	return peersMessage, nil
}

// PeersDiscovered used by MessageHandlers for notifying this coordinator of discovered PeerEndoints.  May include this Peer's PeerEndpoint.
func (p *PeerImpl) PeersDiscovered(peersMessage *pb.PeersMessage) error {
	p.handlerMap.Lock()
	defer p.handlerMap.Unlock()
	thisPeersEndpoint, err := GetPeerEndpoint()
	if err != nil {
		return fmt.Errorf("Error in processing PeersDiscovered: %s", err)
	}
	for _, peerEndpoint := range peersMessage.Peers {
		// Filter out THIS Peer's endpoint
		if getHandlerKeyFromPeerEndpoint(thisPeersEndpoint) == getHandlerKeyFromPeerEndpoint(peerEndpoint) {
			// NOOP
		} else if _, ok := p.handlerMap.m[getHandlerKeyFromPeerEndpoint(peerEndpoint)]; ok == false {
			// Start chat with Peer
			go p.chatWithPeer(peerEndpoint.Address)
		}
	}
	return nil
}

func getHandlerKey(peerMessageHandler MessageHandler) (string, error) {
	peerEndpoint, err := peerMessageHandler.To()
	if err != nil {
		return "", fmt.Errorf("Error getting messageHandler key: %s", err)
	}
	return peerEndpoint.ID.Name, nil
}

func getHandlerKeyFromPeerEndpoint(peerEndpoint *pb.PeerEndpoint) string {
	return peerEndpoint.ID.Name
}

// RegisterHandler register a MessageHandler with this coordinator
func (p *PeerImpl) RegisterHandler(messageHandler MessageHandler) error {
	key, err := getHandlerKey(messageHandler)
	if err != nil {
		return fmt.Errorf("Error registering handler: %s", err)
	}
	p.handlerMap.Lock()
	defer p.handlerMap.Unlock()
	if _, ok := p.handlerMap.m[key]; ok == true {
		// Duplicate, return error
		return newDuplicateHandlerError(messageHandler)
	}
	p.handlerMap.m[key] = messageHandler
	peerLogger.Debug("registered handler with key: %s", key)
	return nil
}

// DeregisterHandler deregisters an already registered MessageHandler for this coordinator
func (p *PeerImpl) DeregisterHandler(messageHandler MessageHandler) error {
	key, err := getHandlerKey(messageHandler)
	if err != nil {
		return fmt.Errorf("Error deregistering handler: %s", err)
	}
	p.handlerMap.Lock()
	defer p.handlerMap.Unlock()
	if _, ok := p.handlerMap.m[key]; !ok {
		// Handler NOT found
		return fmt.Errorf("Error deregistering handler, could not find handler with key: %s", key)
	}
	delete(p.handlerMap.m, key)
	peerLogger.Debug("Deregistered handler with key: %s", key)
	return nil
}

// Broadcast broadcast a message to each of the currently registered PeerEndpoints.
func (p *PeerImpl) Broadcast(msg *pb.OpenchainMessage) []error {
	p.handlerMap.Lock()
	defer p.handlerMap.Unlock()
	var errorsFromHandlers []error
	for _, msgHandler := range p.handlerMap.m {
		err := msgHandler.SendMessage(msg)
		if err != nil {
			toPeerEndpoint, _ := msgHandler.To()
			errorsFromHandlers = append(errorsFromHandlers, fmt.Errorf("Error broadcasting msg (%s) to PeerEndpoint (%s): %s", msg.Type, toPeerEndpoint, err))
		}
	}
	return errorsFromHandlers
}

// SendTransactionsToPeer current temporary mechanism of forwarding transactions to the configured Validator.
func (p *PeerImpl) SendTransactionsToPeer(peerAddress string, transaction *pb.Transaction) *pb.Response {
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))}
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))}
	}
	defer stream.CloseSend()
	peerLogger.Debug("Sending HELLO to Peer: %s", peerAddress)

	helloMessage, err := p.NewOpenchainDiscoveryHello()
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Unexpected error creating new HelloMessage (%s):  %s", peerAddress, err))}
	}
	stream.Send(helloMessage)

	waitc := make(chan struct{})
	var response *pb.Response
	go func() {
		// Make sure to close the wait channel
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to peer address=%s, received EOF when expecting %s", peerAddress, pb.OpenchainMessage_DISC_HELLO))}
				return
			}
			if err != nil {
				response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Unexpected error receiving on stream from peer (%s):  %s", peerAddress, err))}
				return
			}
			if in.Type == pb.OpenchainMessage_DISC_HELLO {
				peerLogger.Debug("Received %s message as expected, sending transaction...", in.Type)
				payload, err := proto.Marshal(transaction)
				if err != nil {
					response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error marshalling transaction to peer address=%s:  %s", peerAddress, err))}
					return
				}
				var ttyp pb.OpenchainMessage_Type
				if transaction.Type == pb.Transaction_CHAINCODE_EXECUTE || transaction.Type == pb.Transaction_CHAINCODE_NEW {
					ttyp = pb.OpenchainMessage_CHAIN_TRANSACTION
				} else {
					ttyp = pb.OpenchainMessage_CHAIN_QUERY
				}

				msg := &pb.OpenchainMessage{Type: ttyp, Payload: payload, Timestamp: util.CreateUtcTimestamp()}
				peerLogger.Debug("Sending message %s with timestamp %v to Peer %s", msg.Type, msg.Timestamp, peerAddress)
				stream.Send(msg)
			} else if in.Type == pb.OpenchainMessage_RESPONSE {
				peerLogger.Debug("Received %s message as expected, exiting out of receive loop", in.Type)
				response = &pb.Response{}
				err = proto.Unmarshal(in.Payload, response)
				if err != nil {
					response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unpacking Payload from %s message: %s", pb.OpenchainMessage_CONSENSUS, err))}
				}
				return
			}
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
		}
	}()

	//TODO Timeout handling
	<-waitc
	return response
}

// SendTransactionsToPeer current temporary mechanism of forwarding transactions to the configured Validator.
func sendTransactionsToThisPeer(peerAddress string, transaction *pb.Transaction) *pb.Response {
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))}
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))}
	}
	defer stream.CloseSend()
	peerLogger.Debug("Sending transaction %s to self", transaction.Type)
	data, err := proto.Marshal(transaction)
	if err != nil {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transaction to local peer: %s", err))}
	}
	var ttyp pb.OpenchainMessage_Type
	if transaction.Type == pb.Transaction_CHAINCODE_EXECUTE || transaction.Type == pb.Transaction_CHAINCODE_NEW {
		ttyp = pb.OpenchainMessage_CHAIN_TRANSACTION
	} else {
		ttyp = pb.OpenchainMessage_CHAIN_QUERY
	}

	msg := &pb.OpenchainMessage{Type: ttyp, Payload: data, Timestamp: util.CreateUtcTimestamp()}
	peerLogger.Debug("Sending message %s with timestamp %v to self", msg.Type, msg.Timestamp)
	stream.Send(msg)

	waitc := make(chan struct{})
	var response *pb.Response
	go func() {
		// Make sure to close the wait channel
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error sending transactions to this peer, received EOF when expecting %s", pb.OpenchainMessage_DISC_HELLO))}
				return
			}
			if err != nil {
				response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Unexpected error receiving on stream from peer (%s):  %s", peerAddress, err))}
				return
			}
			if in.Type == pb.OpenchainMessage_RESPONSE {
				peerLogger.Debug("Received %s message as expected, exiting out of receive loop", in.Type)
				response = &pb.Response{}
				err = proto.Unmarshal(in.Payload, response)
				if err != nil {
					response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unpacking Payload from %s message: %s", pb.OpenchainMessage_CONSENSUS, err))}
				}
				return
			}
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
			response = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload)))}
			return
		}
	}()
	<-waitc
	return response
}

func (p *PeerImpl) chatWithPeer(peerAddress string) error {
	if len(peerAddress) == 0 {
		peerLogger.Debug("Starting up the first peer")
		return nil // nothing to do
	}
	for {
		time.Sleep(1 * time.Second)
		peerLogger.Debug("Initiating Chat with peer address: %s", peerAddress)
		conn, err := NewPeerClientConnectionWithAddress(peerAddress)
		if err != nil {
			e := fmt.Errorf("Error creating connection to peer address=%s:  %s", peerAddress, err)
			peerLogger.Error(e.Error())
			continue
		}
		serverClient := pb.NewPeerClient(conn)
		ctx := context.Background()
		stream, err := serverClient.Chat(ctx)
		if err != nil {
			e := fmt.Errorf("Error establishing chat with peer address=%s:  %s", peerAddress, err)
			peerLogger.Error("%s", e.Error())
			continue
		}
		peerLogger.Debug("Established Chat with peer address: %s", peerAddress)
		p.handleChat(ctx, stream, true)
		stream.CloseSend()
	}
}

// Chat implementation of the the Chat bidi streaming RPC function
func (p *PeerImpl) handleChat(ctx context.Context, stream ChatStream, initiatedStream bool) error {
	deadline, ok := ctx.Deadline()
	peerLogger.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	handler, err := p.handlerFactory(p, stream, initiatedStream, nil)
	if err != nil {
		return fmt.Errorf("Error creating handler during handleChat initiation: %s", err)
	}
	defer handler.Stop()
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			peerLogger.Debug("Received EOF, ending Chat")
			return nil
		}
		if err != nil {
			e := fmt.Errorf("Error during Chat, stopping handler: %s", err)
			peerLogger.Error(e.Error())
			return e
		}
		err = handler.HandleMessage(in)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error handling message: %s", err))
			//return err
		}
	}
}

//The address to stream requests to
func getValidatorStreamAddress() string {
	localaddr, _ := GetLocalAddress()
	if viper.GetBool("peer.validator.enabled") { // in validator mode, send your own address
		return localaddr
	} else if valaddr := viper.GetString("peer.discovery.rootnode"); valaddr != "" {
		return valaddr
	}
	return localaddr
}

//ExecuteTransaction executes transactions decides to do execute in dev or prod mode
func (p *PeerImpl) ExecuteTransaction(transaction *pb.Transaction) *pb.Response {
	peerAddress := getValidatorStreamAddress()
	var response *pb.Response
	if viper.GetBool("peer.validator.enabled") { // send gRPC request to yourself
		response = sendTransactionsToThisPeer(peerAddress, transaction)

	} else {
		response = p.SendTransactionsToPeer(peerAddress, transaction)
	}

	return response
}

// GetPeerEndpoint returns the endpoint for this peer
func (p *PeerImpl) GetPeerEndpoint() (*pb.PeerEndpoint, error) {
	return GetPeerEndpoint()
}

func (p *PeerImpl) newHelloMessage() (*pb.HelloMessage, error) {
	endpoint, err := GetPeerEndpoint()
	if err != nil {
		return nil, fmt.Errorf("Error creating hello message: %s", err)
	}
	p.ledgerWrapper.RLock()
	defer p.ledgerWrapper.RUnlock()
	size := p.ledgerWrapper.ledger.GetBlockchainSize()
	return &pb.HelloMessage{PeerEndpoint: endpoint, BlockNumber: size}, nil
}

// GetBlockByNumber return a block by block number
func (p *PeerImpl) GetBlockByNumber(blockNumber uint64) (*pb.Block, error) {
	p.ledgerWrapper.RLock()
	defer p.ledgerWrapper.RUnlock()
	return p.ledgerWrapper.ledger.GetBlockByNumber(blockNumber)
}

func (p *PeerImpl) GetStateSnapshot() (*state.StateSnapshot, error) {
	p.ledgerWrapper.RLock()
	defer p.ledgerWrapper.RUnlock()
	return p.ledgerWrapper.ledger.GetStateSnapshot()
}

// NewOpenchainDiscoveryHello constructs a new HelloMessage for sending
func (p *PeerImpl) NewOpenchainDiscoveryHello() (*pb.OpenchainMessage, error) {
	helloMessage, err := p.newHelloMessage()
	if err != nil {
		return nil, fmt.Errorf("Error getting new HelloMessage: %s", err)
	}
	data, err := proto.Marshal(helloMessage)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling HelloMessage: %s", err)
	}
	return &pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO, Payload: data, Timestamp: util.CreateUtcTimestamp()}, nil
}

// GetSecHelper returns the crypto.Peer
func (p *PeerImpl) GetSecHelper() crypto.Peer {
	return p.secHelper
}
