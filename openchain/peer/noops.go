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

package peer

import (
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("noops")

type Noops struct {
	Handler     // The consensus programming interface
	NextHandler MessageHandler
}

func NewNoopsHandler(coord MessageHandlerCoordinator, stream ChatStream, initiatedStream bool, next MessageHandler) (MessageHandler, error) {
	d := &Noops{
		Handler: Handler{
			ChatStream:      stream,
			Coordinator:     coord,
			initiatedStream: initiatedStream,
		},
		NextHandler: next,
	}
	d.doneChan = make(chan bool)

	return d, nil
}

// HandleMessage handles the Openchain messages for the Peer.
func (i *Noops) HandleMessage(msg *pb.OpenchainMessage) error {
	logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
	if msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTIONS {
		msg.Type = pb.OpenchainMessage_CONSENSUS
		logger.Debug("Broadcasting %s", msg.Type)
		// broadcast to others so they can exec the tx
		i.Coordinator.Broadcast(msg)

		// WARNING: We might end up getting the same message sent back to us
		// due to Byzantine. We ignore this case for the no-ops consensus
		return nil
	}
	// We process the message if it is OpenchainMessage_CONSENSUS
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		// cpi.ExecTXs(ctx context.Context, txs []*pb.Transaction) ([]byte, []error)
		return nil
	}
	logger.Debug("Did not handle message of type %s, passing on to next MessageHandler", msg.Type)
	return i.NextHandler.HandleMessage(msg)
}
