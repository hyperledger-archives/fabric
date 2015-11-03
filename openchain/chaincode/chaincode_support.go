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
	"fmt"
	"sync"
	"time"

	"github.com/op/go-logging"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	pb "github.com/openblockchain/obc-peer/protos"
)

var chainletLog = logging.MustGetLogger("chaincode")

type HandlerMap struct {
	sync.RWMutex
	m map[string]*ChaincodeHandler
}

func NewChainletSupport() *chainletSupport {
	s := new(chainletSupport)
	s.handlerMap = &HandlerMap{m: make(map[string]*ChaincodeHandler)}
	return s
}

type ChaincodeStream interface {
	Send(*pb.ChaincodeMessage) error
	Recv() (*pb.ChaincodeMessage, error)
}

type chainletSupport struct {
	handlerMap *HandlerMap
}

type DuplicateChaincodeHandlerError struct {
	ChaincodeID *pb.ChainletID
}

func (d *DuplicateChaincodeHandlerError) Error() string {
	return fmt.Sprintf("Duplicate chaincodeID error: %s", d.ChaincodeID)
}

func newDuplicateChaincodeHandlerError(chaincodeHandler *ChaincodeHandler) error {
	return &DuplicateChaincodeHandlerError{ChaincodeID: chaincodeHandler.ChaincodeID}
}

func getHandlerKey(chaincodehandler *ChaincodeHandler) (string, error) {
	if chaincodehandler.ChaincodeID == nil {
		return "", fmt.Errorf("Could not find chaincode handler with nil ChaincodeID")
	}
	return chaincodehandler.ChaincodeID.Url + ":" + chaincodehandler.ChaincodeID.Version, nil
}

func (c *chainletSupport) registerHandler(chaincodehandler *ChaincodeHandler) error {
	key, err := getHandlerKey(chaincodehandler)
	if err != nil {
		return fmt.Errorf("Error registering handler: %s", err)
	}
	if _, ok := c.handlerMap.m[key]; ok == true {
		// Duplicate, return error
		return newDuplicateChaincodeHandlerError(chaincodehandler)
	}
	c.handlerMap.Lock()
	c.handlerMap.m[key] = chaincodehandler
	c.handlerMap.Unlock()
	chaincodeLogger.Debug("registered handler with key: %s", key)
	return nil
}

func (c *chainletSupport) deregisterHandler(chaincodehandler *ChaincodeHandler) error {
	key, err := getHandlerKey(chaincodehandler)
	if err != nil {
		return fmt.Errorf("Error deregistering handler: %s", err)
	}
	c.handlerMap.Lock()
	defer c.handlerMap.Unlock()
	if _, ok := c.handlerMap.m[key]; !ok {
		// Handler NOT found
		return fmt.Errorf("Error deregistering handler, could not find handler with key: %s", key)
	}
	delete(c.handlerMap.m, key)
	chaincodeLogger.Debug("Deregistered handler with key: %s", key)
	return nil
}

func (c *chainletSupport) GetExecutionContext(context context.Context, requestContext *pb.ChainletRequestContext) (*pb.ChainletExecutionContext, error) {
	//chainletId := &pb.ChainletIdentifier{Url: "github."}
	timeStamp := &google_protobuf.Timestamp{Seconds: time.Now().UnixNano(), Nanos: 0}
	executionContext := &pb.ChainletExecutionContext{ChainletId: requestContext.GetId(),
		Timestamp: timeStamp}

	chainletLog.Debug("returning execution context: %s", executionContext)
	return executionContext, nil
}

func (c *chainletSupport) Register(stream pb.ChainletSupport_RegisterServer) error {
	return HandleChaincodeStream(c, stream)
}
