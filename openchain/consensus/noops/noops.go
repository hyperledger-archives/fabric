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

var noopsLogger = logging.MustGetLogger("noops")

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
// @return true if processed and false otherwise
func (i *Noops) RecvMsg(msg *pb.OpenchainMessage) error {
	noopsLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)

	if msg.Type == pb.OpenchainMessage_REQUEST {
		txs := &pb.TransactionBlock{}
		err := proto.Unmarshal(msg.Payload, txs)
		if err != nil {
			err = fmt.Errorf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type)
			return err
		}
		//TODO...we need to change this to single transaction
		var numxacts = len(txs.Transactions)
		if numxacts <= 0 {
			return fmt.Errorf("No transactions to execute")
		} else if numxacts > 1 {
			return fmt.Errorf("Too many transaction to execute %d", numxacts)
		}
		var t = txs.Transactions[0]
		if t.Type == pb.Transaction_CHAINCODE_QUERY {
			//Don't send to consensus but execute here directly
			noopsLogger.Debug("TODO exectute query for transaction %s", t.Uuid)
			return nil
		}

		msg.Type = pb.OpenchainMessage_CONSENSUS
		noopsLogger.Debug("Broadcasting %s", msg.Type)

		// broadcast to others so they can exec the tx
		errs := i.cpi.Broadcast(msg)
		if nil != errs {
			return fmt.Errorf("Failed to broadcast with errors: %v", errs)
		}

		// WARNING: We might end up getting the same message sent back to us
		// due to Byzantine. We ignore this case for the no-ops consensus
	}
	// We process the message if it is OpenchainMessage_CONSENSUS or QUERY. For
	// QUERY, we need to return the result to the caller
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		noopsLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
		txs := &pb.TransactionBlock{}
		err := proto.Unmarshal(msg.Payload, txs)
		if err != nil {
			return err
		}
		_, errs2 := i.cpi.ExecTXs(txs.GetTransactions())
		if errs2 != nil {
			return fmt.Errorf("Fail to execute transactions: %v", errs2)
		}
	}
	return nil
}
