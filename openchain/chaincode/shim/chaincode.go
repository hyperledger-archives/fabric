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
	"errors"
	"flag"
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
	Run(stub *ChaincodeStub, function string, args []string) ([]byte, error)
	// Query is to be used for read-only access to chaincode state
	Query(stub *ChaincodeStub, function string, args []string) ([]byte, error)
}

// ChaincodeStub for shim side handling.
type ChaincodeStub struct {
	UUID string
}

// Peer address derived from command line or env var
var peerAddress string

// Start entry point for chaincodes bootstrap.
func Start(cc Chaincode) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	flag.StringVar(&peerAddress, "peer.address", "", "peer address")

	flag.Parse()

	chaincodeLogger.Debug("Peer address: %s", getPeerAddress())

	// Establish connection with validating peer
	clientConn, err := newPeerClientConnection()
	if err != nil {
		chaincodeLogger.Error(fmt.Sprintf("Error trying to connect to local peer: %s", err))
		return fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	chaincodeLogger.Debug("os.Args returns: %s", os.Args)

	chaincodeSupportClient := pb.NewChaincodeSupportClient(clientConn)

	err = chatWithPeer(chaincodeSupportClient, cc)

	return err
}

func getPeerAddress() string {
	if peerAddress != "" {
		return peerAddress
	}

	if peerAddress = viper.GetString("peer.address"); peerAddress == "" {
		// Assume docker container, return well known docker host address
		peerAddress = "172.17.42.1:30303"
	}

	return peerAddress
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
	chaincodeID := &pb.ChaincodeID{Name: viper.GetString("chaincode.id.name")}
	payload, err := proto.Marshal(chaincodeID)
	if err != nil {
		return fmt.Errorf("Error marshalling chaincodeID during chaincode registration: %s", err)
	}
	// Register on the stream
	chaincodeLogger.Debug("Registering.. sending %s", pb.ChaincodeMessage_REGISTER)
	handler.serialSend(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTER, Payload: payload})
	waitc := make(chan struct{})
	go func() {
		defer close(waitc)
		msgAvail := make(chan *pb.ChaincodeMessage)
		var nsInfo *nextStateInfo
		var in *pb.ChaincodeMessage
		recv := true
		for {
			in = nil
			err = nil
			nsInfo = nil
			if recv {
				recv = false
				go func() {
					var in2 *pb.ChaincodeMessage
					in2, err = stream.Recv()
					msgAvail <- in2
				}()
			}
			select {
			case in = <-msgAvail:
				if err == io.EOF {
					chaincodeLogger.Debug("Received EOF, ending chaincode stream, %s", err)
					return
				} else if err != nil {
					chaincodeLogger.Error(fmt.Sprintf("Received error from server: %s, ending chaincode stream", err))
					return
				} else if in == nil {
					err = fmt.Errorf("Received nil message, ending chaincode stream")
					chaincodeLogger.Debug("Received nil message, ending chaincode stream")
					return
				}
				chaincodeLogger.Debug("[%s]Received message %s from shim", shortuuid(in.Uuid), in.Type.String())
				recv = true
			case nsInfo = <-handler.nextState:
				in = nsInfo.msg
				if in == nil {
					panic("nil msg")
				}
				chaincodeLogger.Debug("[%s]Move state message %s", shortuuid(in.Uuid), in.Type.String())
			}

			// Call FSM.handleMessage()
			err = handler.handleMessage(in)
			if err != nil {
				err = fmt.Errorf("Error handling message: %s", err)
				return
			}
			if nsInfo != nil && nsInfo.sendToCC {
				chaincodeLogger.Debug("[%s]send state message %s", shortuuid(in.Uuid), in.Type.String())
				if err = handler.serialSend(in); err != nil {
					err = fmt.Errorf("Error sending %s: %s", in.Type.String(), err)
					return
				}
			}
		}
	}()
	<-waitc
	return err
}

// GetState function can be invoked by a chaincode to get a state from the ledger.
func (stub *ChaincodeStub) GetState(key string) ([]byte, error) {
	return handler.handleGetState(key, stub.UUID)
}

// PutState function can be invoked by a chaincode to put state into the ledger.
func (stub *ChaincodeStub) PutState(key string, value []byte) error {
	return handler.handlePutState(key, value, stub.UUID)
}

// DelState function can be invoked by a chaincode to delete state from the ledger.
func (stub *ChaincodeStub) DelState(key string) error {
	return handler.handleDelState(key, stub.UUID)
}

// StateRangeQueryIterator allows a chaincode to iterate over a range of
// key/value pairs in the state.
type StateRangeQueryIterator struct {
	handler    *Handler
	uuid       string
	response   *pb.RangeQueryStateResponse
	currentLoc int
}

// RangeQueryState function can be invoked by a chaincode to query of a range
// of keys in the state. Assuming the startKey and endKey are in lexical order,
// an iterator will be returned that can be used to iterate over all keys
// between the startKey and endKey, inclusive. The order in which keys are
// returned by the iterator is random.
func (stub *ChaincodeStub) RangeQueryState(startKey, endKey string) (*StateRangeQueryIterator, error) {
	response, err := handler.handleRangeQueryState(startKey, endKey, stub.UUID)
	if err != nil {
		return nil, err
	}
	return &StateRangeQueryIterator{handler, stub.UUID, response, 0}, nil
}

// HasNext returns true if the range query iterator contains additional keys
// and values.
func (iter *StateRangeQueryIterator) HasNext() bool {
	if iter.currentLoc < len(iter.response.KeysAndValues) || iter.response.HasMore {
		return true
	}
	return false
}

// Next returns the next key and value in the range query iterator.
func (iter *StateRangeQueryIterator) Next() (string, []byte, error) {
	if iter.currentLoc < len(iter.response.KeysAndValues) {
		keyValue := iter.response.KeysAndValues[iter.currentLoc]
		iter.currentLoc++
		return keyValue.Key, keyValue.Value, nil
	} else if !iter.response.HasMore {
		return "", nil, errors.New("No such key")
	} else {
		response, err := iter.handler.handleRangeQueryStateNext(iter.response.ID, iter.uuid)

		if err != nil {
			return "", nil, err
		}

		iter.currentLoc = 0
		iter.response = response
		keyValue := iter.response.KeysAndValues[iter.currentLoc]
		iter.currentLoc++
		return keyValue.Key, keyValue.Value, nil

	}
}

// Close closes the range query iterator. This should be called when done
// reading from the iterator to free up resources.
func (iter *StateRangeQueryIterator) Close() error {
	_, err := iter.handler.handleRangeQueryStateClose(iter.response.ID, iter.uuid)
	return err
}

// InvokeChaincode function can be invoked by a chaincode to execute another chaincode.
func (stub *ChaincodeStub) InvokeChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return handler.handleInvokeChaincode(chaincodeName, function, args, stub.UUID)
}

// QueryChaincode function can be invoked by a chaincode to query another chaincode.
func (stub *ChaincodeStub) QueryChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return handler.handleQueryChaincode(chaincodeName, function, args, stub.UUID)
}
