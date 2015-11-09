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

package shim

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

var chaincodeLogger = logging.MustGetLogger("chaincode")

// Chainlet standard chaincode callback interface
type Chainlet interface {
	Run(chainletSupportClient pb.ChainletSupportClient) error
}

// ChaincodeStub placeholder for shim side handling.
type ChaincodeStub struct {
}

// Start entry point for chaincodes bootstrap
func Start(c Chainlet) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	fmt.Printf("peer.address: %s\n", getPeerAddress())

	clientConn, err := newPeerClientConnection()
	if err != nil {
		return fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	fmt.Printf("os.Args returns: %s\n", os.Args)

	chainletSupportClient := pb.NewChainletSupportClient(clientConn)

	chatWithPeer(chainletSupportClient)

	if err != nil {

	}
	return err
}

func getPeerAddress() string {
	if viper.GetString("peer.address") == "" {
		// Assume docker container, return well known docker host address
		return "172.17.42.1:30303"
	}
	return viper.GetString("peer.address")
}

func newPeerClientConnection() (*grpc.ClientConn, error) {
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
	opts = append(opts, grpc.WithTimeout(1*time.Second))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial(getPeerAddress(), opts...)
	if err != nil {
		return nil, err
	}
	return conn, err
}

func chatWithPeer(chainletSupportClient pb.ChainletSupportClient) error {

	var errFromChat error
	stream, err := chainletSupportClient.Register(context.Background())
	handler := newChaincodeHandler(getPeerAddress(), stream)

	if err != nil {
		return fmt.Errorf("Error chatting with leader at address=%s:  %s", getPeerAddress(), err)
	}
	defer stream.CloseSend()
	// Send the ChainletID during register.
	chainletID := &pb.ChainletID{Url: viper.GetString("chainlet.id.url"), Version: viper.GetString("chainlet.id.version")}
	payload, err := proto.Marshal(chainletID)
	if err != nil {
		return fmt.Errorf("Error marshalling chainletID during chaincode registration: %s", err)
	}
	stream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTER, Payload: payload})
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				errFromChat = fmt.Errorf("Error sending transactions to peer address=%s, received EOF when expecting %s", getPeerAddress(), pb.OpenchainMessage_DISC_HELLO)
				close(waitc)
				return
			}
			if err != nil {
				grpclog.Fatalf("Failed to receive a DiscoverMessage from server : %v", err)
			}

			// Call FSM.HandleMessage()
			err = handler.HandleMessage(in)
			if err != nil {
				chaincodeLogger.Error(fmt.Sprintf("Error handling message: %s", err))
				return
			}
		}
	}()
	<-waitc
	return nil
}
