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

	"github.com/op/go-logging"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	pb "github.com/openblockchain/obc-peer/protos"
)

var chainletLog = logging.MustGetLogger("chaincode")

func NewChainletSupport() *chainletSupport {
	s := new(chainletSupport)
	return s
}

type ChaincodeStream interface {
	Send(*pb.ChaincodeMessage) error
	Recv() (*pb.ChaincodeMessage, error)
}

type chainletSupport struct {
}

// func worker(id int, die chan bool) {
// 	for {
// 		select {
// 		case <-die:
// 			chainletLog.Debug("worker %d terminating", id)
// 			return
// 		default:
// 			chainletLog.Debug("%d is working...", id)
// 			runtime.Gosched()
// 		}
// 	}
// }

func (c *chainletSupport) GetExecutionContext(context context.Context, requestContext *pb.ChainletRequestContext) (*pb.ChainletExecutionContext, error) {
	//chainletId := &pb.ChainletIdentifier{Url: "github."}
	timeStamp := &google_protobuf.Timestamp{Seconds: time.Now().UnixNano(), Nanos: 0}
	executionContext := &pb.ChainletExecutionContext{ChainletId: requestContext.GetId(),
		Timestamp: timeStamp}

	chainletLog.Debug("returning execution context: %s", executionContext)
	return executionContext, nil
}

func (c *chainletSupport) Register(stream pb.ChainletSupport_RegisterServer) error {
	deadline, ok := stream.Context().Deadline()
	peerLogger.Debug("Current context deadline = %s, ok = %v", deadline, ok)
	//peerChatFSM := NewPeerFSM("", stream)
	//handler := p.handlerFactory(stream)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			peerLogger.Debug("Received EOF, ending Chat")
			return nil
		}
		if err != nil {
			return err
		}
		// err = handler.HandleMessage(in)
		// if err != nil {
		// 	peerLogger.Error("Error handling message: %s", err)
		// 	//return err
		// }
		if in.Type == pb.ChaincodeMessage_REGISTER {
			peerLogger.Debug("Got %s, sending back %s", pb.ChaincodeMessage_REGISTER, pb.ChaincodeMessage_REGISTERED)
			if err := stream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTERED}); err != nil {
				return err
			}
		} else {
			peerLogger.Debug("Got unexpected message %s, with bytes length = %d,  doing nothing", in.Type, len(in.Payload))
		}
	}
}
