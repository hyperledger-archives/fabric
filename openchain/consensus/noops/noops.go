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
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
)

// =============================================================================
// Init
// =============================================================================

// Package-level logger
var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("consensus/noops")
}

// =============================================================================
// Structures go here
// =============================================================================

// Noops is a consensus plugin object implementing the consensus.Consenter interface
type Noops struct {
	cpi consensus.CPI // The consensus programming interface
}

// =============================================================================
// Constructors go here
// =============================================================================

// New is a constructor returning a consensus.Consenter object
func New(c consensus.CPI) consensus.Consenter {
	i := &Noops{}
	i.cpi = c
	return i
}

// RecvMsg is called when there is a pb.OpenchainMessage_REQUEST message
// @return true if processed and false otherwise
func (i *Noops) RecvMsg(msg *pb.OpenchainMessage) error {
	logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)

	if msg.Type == pb.OpenchainMessage_REQUEST {
		msg.Type = pb.OpenchainMessage_CONSENSUS
		// broadcast to others so they can exec the tx
		err := i.cpi.Broadcast(msg)
		if nil != err {
			return fmt.Errorf("Failed to broadcast: %v", err)
		}

		// WARNING: We might end up getting the same message sent back to us
		// due to Byzantine. We ignore this case for the no-ops consensus
	}

	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		txs := &pb.TransactionBlock{}
		err := proto.Unmarshal(msg.Payload, txs)
		if err != nil {
			return err
		}
		_, errs := i.cpi.ExecTXs(txs.GetTransactions())
		if errs != nil {
			return fmt.Errorf("Fail to execute transactions: %v", errs)
		}
	}

	return nil
}
