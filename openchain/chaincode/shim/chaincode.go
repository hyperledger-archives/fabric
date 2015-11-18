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

// Chaincode is the standard chaincode callback interface that the chaincode developer needs to implement.
type Chaincode interface {
	// Run method will be called during init and for every transaction
	//Run(chaincodeSupportClient pb.ChaincodeSupportClient) ([]byte, error)
	Run(stub *ChaincodeStub, function string, args []string) ([]byte, error)
	// Query is to be used for read-only access to chaincode state
	//Query(chaincodeSupportClient pb.ChaincodeSupportClient) ([]byte, error)
	Query(stub *ChaincodeStub, function string, args []string) ([]byte, error)
}

// ChaincodeStub for shim side handling.
type ChaincodeStub struct {
	UUID string
}

// Start entry point for chaincodes bootstrap.
func Start(cc Chaincode) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	/*
		viper.SetConfigName("openchain") // name of config file (without extension)
		viper.AddConfigPath("./../../../")        // path to look for the config file in
		err := viper.ReadInConfig()      // Find and read the config file
		if err != nil {                  // Handle errors reading the config file
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	*/
	fmt.Printf("peer.address: %s\n", getPeerAddress())

	// Establish connection with validating peer
	clientConn, err := newPeerClientConnection()
	if err != nil {
		return fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	fmt.Printf("os.Args returns: %s\n", os.Args)

	chaincodeSupportClient := pb.NewChaincodeSupportClient(clientConn)

	//err = c.Run(chaincodeSupportClient)
	//if err != nil {
	//}
	// Handle message exchange with validating peer
	err = chatWithPeer(chaincodeSupportClient, cc)

	return err
}

func getPeerAddress() string {
	//fmt.Printf("Peer address from viper is: %s", viper.GetString("peer.address"))
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

func chatWithPeer(chaincodeSupportClient pb.ChaincodeSupportClient, cc Chaincode) error {

	var errFromChat error
	// Establish stream with validating peer
	stream, err := chaincodeSupportClient.Register(context.Background())
	if err != nil {
		return fmt.Errorf("Error chatting with leader at address=%s:  %s", getPeerAddress(), err)
	}

	// Create the chaincode stub which will be passed to the chaincode
	//stub := &ChaincodeStub{}

	// Create the shim handler responsible for all control logic
	handler = newChaincodeHandler(getPeerAddress(), stream, cc)

	defer stream.CloseSend()
	// Send the ChaincodeID during register.
	chaincodeID := &pb.ChaincodeID{Url: viper.GetString("chaincode.id.url"), Version: viper.GetString("chaincode.id.version")}
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

// GetState function can be invoked by a chaincode to get a state from the ledger.
func (stub *ChaincodeStub) GetState(key string) ([]byte, error) {
	return handler.handleGetState(key, stub.UUID)
}

// PutState function can be invoked by a chaincode to put state into the ledger.
func (stub *ChaincodeStub) PutState(key string, value []byte) error {
	return handler.handlePutState(key, value, stub.UUID)
}

// DelState function can be invoked by a chaincode to del state from the ledger.
func (stub *ChaincodeStub) DelState(key string) error {
	return handler.handleDelState(key, stub.UUID)
}

// InvokeChaincode function can be invoked by a chaincode to execute another chaincode.
func (stub *ChaincodeStub) InvokeChaincode(chaincodeID string, args []byte) ([]byte, error) {
	return handler.handleInvokeChaincode(chaincodeID, args, stub.UUID)
}
