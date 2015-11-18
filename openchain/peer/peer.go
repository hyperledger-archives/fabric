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

	pb "github.com/openblockchain/obc-peer/protos"
)

const defaultTimeout = time.Second * 3

// MessageHandler standard interface for handling Openchain messages.
type MessageHandler interface {
	HandleMessage(msg *pb.OpenchainMessage) error
	SendMessage(msg *pb.OpenchainMessage) error
	To() (pb.PeerEndpoint, error)
	Stop() error
}

// MessageHandlerCoordinator responsible for coordinating between the registered MessageHandler's
type MessageHandlerCoordinator interface {
	RegisterHandler(messageHandler MessageHandler) error
	DeregisterHandler(messageHandler MessageHandler) error
	Broadcast(*pb.OpenchainMessage) []error
	GetPeers() (*pb.PeersMessage, error)
	PeersDiscovered(*pb.PeersMessage) error
}

// ChatStream interface supported by stream between Peers
type ChatStream interface {
	Send(*pb.OpenchainMessage) error
	Recv() (*pb.OpenchainMessage, error)
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

// GetPeerEndpoint returns the PeerEndpoint for this Peer instance.  Affected by env:peer.addressAutoDetect
func GetPeerEndpoint() (*pb.PeerEndpoint, error) {
	var peerAddress string
	if viper.GetBool("peer.addressAutoDetect") {
		// Need to get the port from the peer.address setting, and append to the determined host IP
		_, port, err := net.SplitHostPort(viper.GetString("peer.address"))
		if err != nil {
			return nil, fmt.Errorf("Error auto detecting Peer's address: %s", err)
		}
		peerAddress = net.JoinHostPort(GetLocalIP(), port)
		peerLogger.Warning("AutoDetected Peer address: %s", peerAddress)
	} else {
		peerAddress = viper.GetString("peer.address")
	}
	return &pb.PeerEndpoint{ID: &pb.PeerID{Name: viper.GetString("peer.id")}, Address: peerAddress}, nil

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

type handlerMap struct {
	sync.RWMutex
	m map[string]MessageHandler
}

// Peer implementation of the Peer service
type Peer struct {
	handlerFactory func(MessageHandlerCoordinator, ChatStream, bool, MessageHandler) (MessageHandler, error)
	handlerMap     *handlerMap
}

// NewPeerWithHandler returns a Peer which uses the supplied handler factory function for creating new handlers on new Chat service invocations.
func NewPeerWithHandler(handlerFact func(MessageHandlerCoordinator, ChatStream, bool, MessageHandler) (MessageHandler, error)) (*Peer, error) {
	peer := new(Peer)
	if handlerFact == nil {
		return nil, errors.New("Cannot supply nil handler factory")
	}
	peer.handlerFactory = handlerFact
	peer.handlerMap = &handlerMap{m: make(map[string]MessageHandler)}
	go peer.chatWithPeer(viper.GetString("peer.discovery.rootnode"))
	return peer, nil
}

// Chat implementation of the the Chat bidi streaming RPC function
func (p *Peer) Chat(stream pb.Peer_ChatServer) error {
	return p.handleChat(stream.Context(), stream, false)
}

// GetPeers returns the currently registered PeerEndpoints
func (p *Peer) GetPeers() (*pb.PeersMessage, error) {
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
func (p *Peer) PeersDiscovered(peersMessage *pb.PeersMessage) error {
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
func (p *Peer) RegisterHandler(messageHandler MessageHandler) error {
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
func (p *Peer) DeregisterHandler(messageHandler MessageHandler) error {
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
func (p *Peer) Broadcast(msg *pb.OpenchainMessage) []error {
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
func SendTransactionsToPeer(peerAddress string, transactionBlock *pb.TransactionBlock) error {
	var errFromChat error
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return fmt.Errorf("Error sending transactions to peer address=%s:  %s", peerAddress, err)
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	if err != nil {
		return fmt.Errorf("Error sending transactions to peer address=%s:  %s", peerAddress, err)
	}
	defer stream.CloseSend()
	peerLogger.Debug("Sending HELLO to Peer: %s", peerAddress)
	stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO})
	waitc := make(chan struct{})
	go func() {
		// Make sure to close the wait channel
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				errFromChat = fmt.Errorf("Error sending transactions to peer address=%s, received EOF when expecting %s", peerAddress, pb.OpenchainMessage_DISC_HELLO)
				return
			}
			if err != nil {
				errFromChat = fmt.Errorf("Unexpected error receiving on stream from peer (%s):  %s", peerAddress, err)
				return
			}
			if in.Type == pb.OpenchainMessage_DISC_HELLO {
				peerLogger.Debug("Received %s message as expected, sending transactions...", in.Type)
				payload, err := proto.Marshal(transactionBlock)
				if err != nil {
					errFromChat = fmt.Errorf("Error marshalling transactions to peer address=%s:  %s", peerAddress, err)
					return
				}
				stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CHAIN_TRANSACTIONS, Payload: payload})
				peerLogger.Debug("Transactions sent to peer address: %s", peerAddress)
				return
			}
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
			return
		}
	}()
	<-waitc
	return errFromChat
}

// SendTransactionsToPeer current temporary mechanism of forwarding transactions to the configured Validator.
func sendTransactionsToThisPeer(peerAddress string, transactionBlock *pb.TransactionBlock) (*pb.OpenchainMessage, error) {
	var errFromChat error
	var response *pb.OpenchainMessage
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return nil, fmt.Errorf("Error sending transactions to peer address=%s:  %s", peerAddress, err)
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Error sending transactions to peer address=%s:  %s", peerAddress, err)
	}
	defer stream.CloseSend()
	peerLogger.Debug("Sending %s to this Peer", pb.OpenchainMessage_REQUEST)
	data, err := proto.Marshal(transactionBlock)
	if err != nil {
		return nil, fmt.Errorf("Error sending transactions to local peer: %s", err)
	}
	stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_REQUEST, Payload: data})
	waitc := make(chan struct{})
	go func() {
		// Make sure to close the wait channel
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				errFromChat = fmt.Errorf("Error sending transactions to this peer, received EOF when expecting %s", pb.OpenchainMessage_DISC_HELLO)
				return
			}
			if err != nil {
				errFromChat = fmt.Errorf("Unexpected error receiving on stream from peer (%s):  %s", peerAddress, err)
				return
			}
			if in.Type == pb.OpenchainMessage_RESPONSE {
				peerLogger.Debug("Received %s message as expected, exiting out of receive loop", in.Type)
				response = in
				return
			}
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
			errFromChat = fmt.Errorf("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
			return
		}
	}()
	<-waitc
	return response, errFromChat
}

func (p *Peer) chatWithPeer(peerAddress string) error {
	peerLogger.Debug("Initiating Chat with peer address: %s", peerAddress)
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		e := fmt.Errorf("Error creating connection to peer address=%s:  %s", peerAddress, err)
		peerLogger.Error(e.Error())
		return e
	}
	serverClient := pb.NewPeerClient(conn)
	ctx := context.Background()
	stream, err := serverClient.Chat(ctx)
	if err != nil {
		return fmt.Errorf("Error establishing chat with peer address=%s:  %s", peerAddress, err)
	}
	defer stream.CloseSend()
	peerLogger.Debug("Established Chat with peer address: %s", peerAddress)
	p.handleChat(ctx, stream, true)
	return nil
}

// Chat implementation of the the Chat bidi streaming RPC function
func (p *Peer) handleChat(ctx context.Context, stream ChatStream, initiatedStream bool) error {
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
	var localaddr = viper.GetString("peer.address")
	if viper.GetString("peer.mode") == "dev" { //in dev mode, we have the "noops" validator
		return localaddr
	} else if valaddr := viper.GetString("peer.discovery.rootnode"); valaddr != "" {
		return valaddr
	}
	return localaddr
}

//ExecuteTransaction executes transactions decides to do execute in dev or prod mode
func ExecuteTransaction(transaction *pb.Transaction) ([]byte, error) {
	transactionBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{transaction}}
	peerAddress := getValidatorStreamAddress()
	var payload []byte
	var err error
	if viper.GetString("peer.mode") == "dev" {
		//TODO in dev mode, we have to do something else
		response, err := sendTransactionsToThisPeer(peerAddress, transactionBlock)
		if err != nil {
			return nil, fmt.Errorf("Error executing transaction: %s", err)
		}
		payload = response.Payload

	} else {
		err = SendTransactionsToPeer(peerAddress, transactionBlock)
	}

	return payload, err
}
