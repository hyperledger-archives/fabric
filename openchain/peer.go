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
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	pb "github.com/openblockchain/obc-peer/protos"
)

const DefaultTimeout = time.Second * 3

type MessageHandler interface {
	HandleMessage(msg *pb.OpenchainMessage) error
	SendMessage(msg *pb.OpenchainMessage) error
}

type PeerChatStream interface {
	Send(*pb.OpenchainMessage) error
	Recv() (*pb.OpenchainMessage, error)
}

func testAcceptPeerChatStream(PeerChatStream) {

}

var peerLogger = logging.MustGetLogger("peer")

// Returns a new grpc.ClientConn to the configured local PEER.
func NewPeerClientConnection() (*grpc.ClientConn, error) {
	return NewPeerClientConnectionWithAddress(viper.GetString("peer.address"))
}

// Returns a new grpc.ClientConn to the configured local PEER.
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
	opts = append(opts, grpc.WithTimeout(DefaultTimeout))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial(peerAddress, opts...)
	if err != nil {
		return nil, err
	}
	return conn, err
}

type Peer struct {
	handlerFactory func(PeerChatStream) MessageHandler
}

func NewPeer() *Peer {
	peer := new(Peer)
	return peer
}

func NewPeerWithHandler(handlerFact func(PeerChatStream) MessageHandler) (*Peer, error) {
	peer := new(Peer)
	if handlerFact == nil {
		return nil, errors.New("Cannot supply nil handler factory")
	}
	peer.handlerFactory = handlerFact
	return peer, nil
}

func (p *Peer) Chat(stream pb.Peer_ChatServer) error {
	testAcceptPeerChatStream(stream)
	deadline, ok := stream.Context().Deadline()
	peerLogger.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	//peerChatFSM := NewPeerFSM("", stream)
	handler := p.handlerFactory(stream)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			peerLogger.Debug("Received EOF, ending Chat")
			return nil
		}
		if err != nil {
			return err
		}
		err = handler.HandleMessage(in)
		if err != nil {
			peerLogger.Error("Error handling message: %s", err)
			//return err
		}
		// if in.Type == pb.OpenchainMessage_DISC_HELLO {
		// 	peerLogger.Debug("Got %s, sending back %s", pb.OpenchainMessage_DISC_HELLO, pb.OpenchainMessage_DISC_HELLO)
		// 	if err := stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
		// 		return err
		// 	}
		// } else if in.Type == pb.OpenchainMessage_DISC_GET_PEERS {
		// 	peerLogger.Debug("Got %s, sending back peers", pb.OpenchainMessage_DISC_GET_PEERS)
		// 	if err := stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_PEERS}); err != nil {
		// 		return err
		// 	}
		// } else {
		// 	peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
		// }
	}
}

func SendTransactionsToPeer(peerAddress string, transactionsMessage *pb.TransactionsMessage) error {
	var errFromChat error = nil
	conn, err := NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))
	}
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	//testAcceptPeerChatStream(stream)
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending transactions to peer address=%s:  %s", peerAddress, err))
	} else {
		defer stream.CloseSend()
		peerLogger.Debug("Sending HELLO to Peer: %s", peerAddress)
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
				if in.Type == pb.OpenchainMessage_DISC_HELLO {
					peerLogger.Debug("Received %s message as expected, sending transactions...", in.Type)
					payload, err := proto.Marshal(transactionsMessage)
					if err != nil {
						errFromChat = errors.New(fmt.Sprintf("Error marshalling transactions to peer address=%s:  %s", peerAddress, err))
						close(waitc)
						return
					}
					stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_CHAIN_TRANSACTIONS, Payload: payload})
					peerLogger.Debug("Transactions sent to peer address: %s", peerAddress)
					close(waitc)
					return
				} else {
					peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
					close(waitc)
					return
				}
			}
		}()
		<-waitc
		return nil
	}
}

type PeerFSM struct {
	To         string
	ChatStream PeerChatStream
	FSM        *fsm.FSM
}

func NewPeerFSM(to string, peerChatStream PeerChatStream) *PeerFSM {
	d := &PeerFSM{
		To:         to,
		ChatStream: peerChatStream,
	}

	d.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_DISC_PING.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_DISC_GET_PEERS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_DISC_PEERS.String(), Src: []string{"established"}, Dst: "established"},
		},
		fsm.Callbacks{
			"enter_state":                                           func(e *fsm.Event) { d.enterState(e) },
			"before_" + pb.OpenchainMessage_DISC_HELLO.String():     func(e *fsm.Event) { d.beforeHello(e) },
			"before_" + pb.OpenchainMessage_DISC_GET_PEERS.String(): func(e *fsm.Event) { d.beforeGetPeers(e) },
		},
	)

	return d
}

func (d *PeerFSM) enterState(e *fsm.Event) {
	peerLogger.Debug("The Peer's bi-directional stream to %s is %s, from event %s\n", d.To, e.Dst, e.Event)
}

func (d *PeerFSM) beforeHello(e *fsm.Event) {
	peerLogger.Debug("Sending back %s", pb.OpenchainMessage_DISC_HELLO.String())
	if err := d.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
		e.Cancel(err)
	}
}
func (d *PeerFSM) beforeGetPeers(e *fsm.Event) {
	peerLogger.Debug("Sending back %s", pb.OpenchainMessage_DISC_PEERS.String())
	if err := d.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_PEERS}); err != nil {
		e.Cancel(err)
	}
}

func (d *PeerFSM) when(stateToCheck string) bool {
	return d.FSM.Is(stateToCheck)
}

func (d *PeerFSM) HandleMessage(msg *pb.OpenchainMessage) error {
	peerLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
	if d.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Peer FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), d.FSM.Current())
	}
	err := d.FSM.Event(msg.Type.String())
	if err != nil {
		if _, ok := err.(*fsm.NoTransitionError); !ok {
			// Only allow NoTransitionError's, all others are considered true error.
			return fmt.Errorf("Peer FSM failed while handling message (%s): current state: %s, error: %s", msg.Type.String(), d.FSM.Current(), err)
			//t.Error("expected only 'NoTransitionError'")
		}
	}

	// if err != nil {
	// 	return fmt.Errorf("Peer FSM failed while handling message (%s): current state: %s, error: %s", msg.Type.String(), d.FSM.Current(), err)
	// }
	// if d.when("created") {
	// 	switch msg.Type {
	// 	case pb.OpenchainMessage_DISC_HELLO:
	// 		return nil
	// 	}
	// }
	return nil
}

func (d *PeerFSM) SendMessage(msg *pb.OpenchainMessage) error {
	peerLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := d.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}
