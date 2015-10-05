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
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/op/go-logging"
	"github.com/spf13/viper"

	pb "github.com/openblockchain/obc-peer/protos"
)

const DefaultTimeout = time.Second * 3

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
}

func NewPeer() *Peer {
	peer := new(Peer)
	return peer
}

func (*Peer) Chat(stream pb.Peer_ChatServer) error {
	testAcceptPeerChatStream(stream)
	deadline, ok := stream.Context().Deadline()
	peerLogger.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	for {
		in, err := stream.Recv()
		// Appears no deadling is set as ok is 'false'
		if err == io.EOF {
			peerLogger.Debug("Received EOF, ending discovery handshake")
			return nil
		}
		if err != nil {
			return err
		}
		if in.Type == pb.OpenchainMessage_DISC_HELLO {
			peerLogger.Debug("Got %s, sending back %s", pb.OpenchainMessage_DISC_HELLO, pb.OpenchainMessage_DISC_HELLO)
			if err := stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
				return err
			}
		} else if in.Type == pb.OpenchainMessage_DISC_GET_PEERS {
			peerLogger.Debug("Got %s, sending back peers", pb.OpenchainMessage_DISC_GET_PEERS)
			if err := stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_PEERS}); err != nil {
				return err
			}
		} else {
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
		}
	}
}
