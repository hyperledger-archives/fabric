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

package noops

import (
	"github.com/openblockchain/obc-peer/openchain/consensus"

	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("noops")

// Noops is a consensus plugin object implementing consensus.Consenter interface
type Noops struct {
	cpi consensus.CPI // The consensus programming interface
}

// New is a constructor returning a consensus.Consenter object
func New(c consensus.CPI) consensus.Consenter {
	i := &Noops{}
	i.cpi = c
	return i
}

// RecvMsg is called when there is a pb.OpenchainMessage_REQUEST message
func (i *Noops) RecvMsg(msg *pb.OpenchainMessage) (err error) {
	logger.Info("Message received %s", msg.Type)

	if msg.Type == pb.OpenchainMessage_REQUEST {
		msg.Type = pb.OpenchainMessage_CONSENSUS
		logger.Debug("Broadcasting %s", msg.Type)
		cpi.Broadcast(msg) // broadcast to others so they can exec the tx
		// the msg must be pb.OpenchainMessage_CONSENSUS
		logger.Debug("Executing transaction %s", msg.Type)
		return nil // cpi.ExecTXs(ctx context.Context, txs []*pb.Transaction) ([]byte, []error)
	}
	logger.Info("Got a wrong message %s", msg.Type)

	return nil
}
