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

package chaincode

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

type Chainlet interface {
	Run(chainletSupportClient pb.ChainletSupportClient) error
}

type ChaincodeStub struct {
}

func Start(c Chainlet) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	fmt.Printf("peer.address: %s\n", getPeerAddress())

	clientConn, err := newPeerClientConnection()
	if err != nil {
		return errors.New(fmt.Sprintf("Error trying to connect to local peer: %s", err))
	}

	fmt.Printf("os.Args returns: %s\n", os.Args)

	chainletSupportClient := pb.NewChainletSupportClient(clientConn)

	err = c.Run(chainletSupportClient)
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

func (cs *ChaincodeStub) chatWithPeer(chainletSupportClient pb.ChainletSupportClient) error {

	var errFromChat error = nil
	stream, err := chainletSupportClient.Register(context.Background())
	//v.leaderHandler = v.GetHandler(stream)

	if err != nil {
		return errors.New(fmt.Sprintf("Error chatting with leader at address=%s:  %s", peerAddress, err))
	} else {
		defer stream.CloseSend()
		stream.Send(&pb.OpenchainMessage{Type: pb.ChaincodeMessage_REGISTER})
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

				// // Call FSM.HandleMessage()
				// err = v.leaderHandler.HandleMessage(in)
				// if err != nil {
				// 	validatorLogger.Error("Error handling message: %s", err)
				// 	return
				// }
			}
		}()
		<-waitc
		return nil
	}
}
