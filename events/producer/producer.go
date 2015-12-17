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

package producer

import (
	"fmt"
	"io"
	"time"

	"github.com/op/go-logging"

	pb "github.com/openblockchain/obc-peer/protos"
)

const defaultTimeout = time.Second * 3

var producerLogger = logging.MustGetLogger("eventhub_producer")

// OpenchainEventsServer implementation of the Peer service
type OpenchainEventsServer struct {
}

//singleton - if we want to create multiple servers, we need to subsume events.gEventConsumers into OpenchainEventsServer
var globalOpenchainEventsServer *OpenchainEventsServer

// NewOpenchainEventsServer returns a OpenchainEventsServer
func NewOpenchainEventsServer(bufferSize uint, timeout int) *OpenchainEventsServer {
	if globalOpenchainEventsServer != nil {
		panic("Cannot create multiple event hub servers")
	}
	globalOpenchainEventsServer = new(OpenchainEventsServer)
	initializeEvents(bufferSize, timeout)
	return globalOpenchainEventsServer
}

// Chat implementation of the the Chat bidi streaming RPC function
func (p *OpenchainEventsServer) Chat(stream pb.OpenchainEvents_ChatServer) error {
	handler, err := newOpenchainEventHandler(stream)
	if err != nil {
		return fmt.Errorf("Error creating handler during handleChat initiation: %s", err)
	}
	defer handler.Stop()
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			producerLogger.Debug("Received EOF, ending Chat")
			return nil
		}
		if err != nil {
			e := fmt.Errorf("Error during Chat, stopping handler: %s", err)
			producerLogger.Error(e.Error())
			return e
		}
		err = handler.HandleMessage(in)
		if err != nil {
			producerLogger.Error(fmt.Sprintf("Error handling message: %s", err))
			//return err
		}
	}
}
