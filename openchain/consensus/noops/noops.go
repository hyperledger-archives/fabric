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
	"github.com/openblockchain/obc-peer/openchain/ledger"
	pb "github.com/openblockchain/obc-peer/protos"
)

// =============================================================================
// Init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/noops")
}

// =============================================================================
// Structures go here
// =============================================================================

// Noops is a plugin object implementing the consensus.Consenter interface.
type Noops struct {
	cpi consensus.CPI
}

// =============================================================================
// Constructors go here
// =============================================================================

// New is a constructor returning a consensus.Consenter object.
func New(c consensus.CPI) consensus.Consenter {
	i := &Noops{}
	i.cpi = c
	return i
}

// RecvMsg is called for OpenchainMessage_CHAIN_TRANSACTION and OpenchainMessage_CONSENSUS messages.
func (i *Noops) RecvMsg(msg *pb.OpenchainMessage) error {
	logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)

	//cannot be QUERY. it is filtered out by handler
	if msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		t := &pb.Transaction{}
		if err := proto.Unmarshal(msg.Payload, t); err != nil {
			return fmt.Errorf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type)
		}

		// Change the msg type to consensus and broadcast to the network so that
		// other validators may execute the transaction
		msg.Type = pb.OpenchainMessage_CONSENSUS
		logger.Debug("Broadcasting %s", msg.Type)
		txs := &pb.TransactionBlock{Transactions: []*pb.Transaction{t}}
		if payload, err := proto.Marshal(txs); err != nil {
			return err
		}
		msg.Payload = payload
		if errs := i.cpi.Broadcast(msg); nil != errs {
			return fmt.Errorf("Failed to broadcast with errors: %v", errs)
		}
	}
	// We process the message if it is OpenchainMessage_CONSENSUS. ie, all transactions
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
		txs := &pb.TransactionBlock{}
		if err := proto.Unmarshal(msg.Payload, txs); err != nil {
			return err
		}
		txarr := txs.GetTransactions()
		logger.Debug("Executing transactions")

		// We start each tx in its own block
		if ledger, err := ledger.GetLedger(); err != nil {
			return fmt.Errorf("Fail to get the ledger: %v", err)
		}
		if err := ledger.BeginTxBatch(msg.timestamp); err != nil {
			return fmt.Errorf("Fail to begin transaction with the ledger: %v", err)
		}

		hash, errs := i.cpi.ExecTXs(txarr)
		//there are n+1 elements of errors in this array. On complete success
		//they'll all be nil. In particular, the last err will be error in
		//producing the hash, if any. That's the only error we do want to check
		if errs[len(txarr)] != nil {
			return fmt.Errorf("Fail to execute transactions: %v", errs)
		}

		if err := ledger.CommitTxBatch(msg.timestamp, txarr, nil); err != nil {
			ledger.RollbackTxBatch(msg.timestamp)
			return fmt.Errorf("Fail to commit transaction to the ledger: %v", err)
		}

		// TODO: Broadcast CHAIN_NEW_BLOCK to connected NVPs
		// For now, send to everyone until broadcast has better discrimination
		block := ledger.GetBlockByNumber(ledger.GetBlockchainSize())
		delta := ledger.GetStateDelta(ledger.GetBlockchainSize())
		data, err := proto.Marshal(&pb.NewBlock{Block: block, StateDelta: delta})
		if err != nil {
			return fmt.Errorf("Fail to marshall NewBlock structure: %v", err)
		}
		msg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_SYNC_NEW_BLOCK, Payload: data}
		if errs := i.cpi.Broadcast(msg); nil != errs {
			return fmt.Errorf("Failed to broadcast with errors: %v", errs)
		}

	}
	return nil
}
