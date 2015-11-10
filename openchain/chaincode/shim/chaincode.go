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

// Logger for the shim package.
var chaincodeLogger = logging.MustGetLogger("chaincode")

// Handler to shim that handles all control logic.
var handler *Handler

// Standard chaincode callback interface - this is the interface the chaincode developer needs to implement.
type Chaincode interface {
	// Run method will be called during init and for every transaction
	//Run(chainletSupportClient pb.ChainletSupportClient) ([]byte, error)
	Run(stub *ChaincodeStub, function string, args []string) ([]byte, error)
	// Query is to be used for read-only access to chaincode state
	//Query(chainletSupportClient pb.ChainletSupportClient) ([]byte, error)
	Query(stub *ChaincodeStub, function string, args []string) ([]byte, error)
}

// ChaincodeStub for shim side handling.
type ChaincodeStub struct {
}

// Start entry point for chaincodes bootstrap.
func Start(cc Chaincode) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	fmt.Printf("peer.address: %s\n", getPeerAddress())

	// Establish connection with validating peer
	clientConn, err := newPeerClientConnection()
	if err != nil {
		return fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	fmt.Printf("os.Args returns: %s\n", os.Args)

	chainletSupportClient := pb.NewChainletSupportClient(clientConn)

	//err = c.Run(chainletSupportClient)
	//if err != nil {
	//}
	// Handle message exchange with validating peer
	err = chatWithPeer(chainletSupportClient, cc)

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

func chatWithPeer(chainletSupportClient pb.ChainletSupportClient, cc Chaincode) error {

	var errFromChat error
	// Establish stream with validating peer
	stream, err := chainletSupportClient.Register(context.Background())
	if err != nil {
		return fmt.Errorf("Error chatting with leader at address=%s:  %s", getPeerAddress(), err)
	}

	// Create the chaincode stub which will be passed to the chaincode
	stub := &ChaincodeStub{}

	// Create the shim handler responsible for all control logic
	handler = newChaincodeHandler(getPeerAddress(), stream, cc, stub)

	defer stream.CloseSend()
	// Send the ChaincodeID during register.
	chaincodeID := &pb.ChainletID{Url: viper.GetString("chainlet.id.url"), Version: viper.GetString("chainlet.id.version")}
	payload, err := proto.Marshal(chaincodeID)
	if err != nil {
		return fmt.Errorf("Error marshalling chaincodeID during chaincode registration: %s", err)
	}
	// Register on the stream
	chaincodeLogger.Debug("Registering.. sending %s", pb.ChaincodeMessage_REGISTER)
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
				grpclog.Fatalf("Failed to receive a Registered message from server : %v", err)
			}

			// Call FSM.handleMessage()
			err = handler.handleMessage(in)
			if err != nil {
				chaincodeLogger.Error(fmt.Sprintf("Error handling message: %s", err))
				return
			}
		}
	}()
	<-waitc
	return nil
}

// This is the method chaincode will call to get state.
func (stub *ChaincodeStub) GetState(key string) ([]byte, error) {
	return handler.handleGetState(key)
}

// This is the method chaincode will call to put state.
func (stub *ChaincodeStub) PutState(key string, value []byte) error {
	return handler.handlePutState(key, value)
}

// This is the method chaincode will call to delete state.
func (stub *ChaincodeStub) DelState(key string) error {
	return handler.handleDelState(key)
}

// This is the method chaincode will call to invoke another chaincode
func (stub *ChaincodeStub) InvokeChaincode(chaincodeID string, args []byte) ([]byte, error) {
	return handler.handleInvokeChaincode(chaincodeID, args)
}
