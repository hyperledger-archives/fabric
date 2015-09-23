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
	"time"

	"github.com/op/go-logging"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	pb "hub.jazz.net/openchain-peer/protos"
)

var chainletLog = logging.MustGetLogger("chaincode")

func NewChainletSupport() *chainletSupport {
	s := new(chainletSupport)
	return s
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

func (*chainletSupport) GetExecutionContext(context context.Context, requestContext *pb.ChainletRequestContext) (*pb.ChainletExecutionContext, error) {
	//chainletId := &pb.ChainletIdentifier{Url: "github."}
	timeStamp := &google_protobuf.Timestamp{Seconds: time.Now().UnixNano(), Nanos: 0}
	executionContext := &pb.ChainletExecutionContext{ChainletId: requestContext.GetId(),
		Timestamp: timeStamp}

	chainletLog.Debug("returning execution context: %s", executionContext)
	return executionContext, nil
}
