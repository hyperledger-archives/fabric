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
	"github.com/openblockchain/obc-peer/openchain/util"
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
	txQ *util.Queue
}

var iNoops consensus.Consenter

// =============================================================================
// Constructors go here
// =============================================================================

// NewNoops is a constructor returning a consensus.Consenter object.
func NewNoops(c consensus.CPI) consensus.Consenter {
	logger.Debug("Creating a NOOPS object")
	i := &Noops{}
	i.cpi = c
	i.txQ = util.NewQueue()
	return i
}

// GetNoops returns a singleton of NOOPS
func GetNoops(c consensus.CPI) consensus.Consenter {
	if iNoops == nil {
		iNoops = NewNoops(c)
	}
	return iNoops
}

// RecvMsg is called for OpenchainMessage_CHAIN_TRANSACTION and OpenchainMessage_CONSENSUS messages.
func (i *Noops) RecvMsg(msg *pb.OpenchainMessage) error {
	logger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)

	if msg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		if err := i.broadcastConsensusMsg(msg); nil != err {
			return err
		}
	}
	if msg.Type == pb.OpenchainMessage_CONSENSUS {
		txarr, err := i.getTransactionsFromMsg(msg)
		if nil != err {
			return err
		}
		if i.canProcess(txarr) {
			i.txQ.Push(txarr)
			return i.doTransactions(msg)
		}
		i.queueTransactions(txarr)
	}
	return nil
}

func (i *Noops) broadcastConsensusMsg(msg *pb.OpenchainMessage) error {
	t := &pb.Transaction{}
	if err := proto.Unmarshal(msg.Payload, t); err != nil {
		return fmt.Errorf("Error unmarshalling payload of received OpenchainMessage:%s.", msg.Type)
	}

	// Change the msg type to consensus and broadcast to the network so that
	// other validators may execute the transaction
	msg.Type = pb.OpenchainMessage_CONSENSUS
	logger.Debug("Broadcasting %s", msg.Type)
	txs := &pb.TransactionBlock{Transactions: []*pb.Transaction{t}}
	payload, err := proto.Marshal(txs)
	if err != nil {
		return err
	}
	msg.Payload = payload
	if errs := i.cpi.Broadcast(msg); nil != errs {
		return fmt.Errorf("Failed to broadcast with errors: %v", errs)
	}
	return nil
}

func (i *Noops) getTransactionsFromMsg(msg *pb.OpenchainMessage) ([]*pb.Transaction, error) {
	txs := &pb.TransactionBlock{}
	if err := proto.Unmarshal(msg.Payload, txs); err != nil {
		return nil, err
	}
	return txs.GetTransactions(), nil
}

func (i *Noops) getTransactionsFromQueue() []*pb.Transaction {
	len := i.txQ.Size()
	if len == 1 {
		return i.txQ.Pop().([]*pb.Transaction)
	}
	txarr := make([]*pb.Transaction, len)
	for k := 0; k < len; k++ {
		txs := i.txQ.Pop().([]*pb.Transaction)
		txarr[k] = txs[0]
	}
	return txarr
}

func (i *Noops) canProcess(txarr []*pb.Transaction) bool {
	// For NOOPS, if we have completed the sync since we last connected,
	// we can assume that we are at the current state; otherwise, we need to
	// wait for the sync process to complete before we can exec the transactions

	// TODO: Ask coordinator if we need to start sync

	return true
}

func (i *Noops) doTransactions(msg *pb.OpenchainMessage) error {
	logger.Debug("Executing transactions")
	logger.Debug("Starting TX batch with timestamp: %v", msg.Timestamp)
	if err := i.cpi.BeginTxBatch(msg.Timestamp); err != nil {
		return err
	}

	// Grab all transactions from the FIFO queue and run them in order
	txarr := i.getTransactionsFromQueue()
	logger.Debug("Executing batch of %d transactions", len(txarr))
	_, errs := i.cpi.ExecTXs(txarr)

	//there are n+1 elements of errors in this array. On complete success
	//they'll all be nil. In particular, the last err will be error in
	//producing the hash, if any. That's the only error we do want to check
	if errs[len(txarr)] != nil {
		logger.Debug("Rolling back TX batch with timestamp: %v", msg.Timestamp)
		i.cpi.RollbackTxBatch(msg.Timestamp)
		return fmt.Errorf("Fail to execute transactions: %v", errs)
	}

	logger.Debug("Committing TX batch with timestamp: %v", msg.Timestamp)
	if err := i.cpi.CommitTxBatch(msg.Timestamp, txarr, nil); err != nil {
		logger.Debug("Rolling back TX batch with timestamp: %v", msg.Timestamp)
		i.cpi.RollbackTxBatch(msg.Timestamp)
		return err
	}

	return i.notifyBlockAdded()
}

func (i *Noops) notifyBlockAdded() error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Fail to get the ledger: %v", err)
	}

	// TODO: Broadcast SYNC_BLOCK_ADDED to connected NVPs
	// VPs already know about this newly added block since they participate
	// in the execution. That is, they can compare their current block with
	// the network block
	// For now, send to everyone until broadcast has better discrimination
	blockHeight := ledger.GetBlockchainSize()
	logger.Debug("Preparing to broadcast with block number %v", blockHeight)
	block, err := ledger.GetBlockByNumber(blockHeight - 1)
	if nil != err {
		return err
	}
	//delta, err := ledger.GetStateDeltaBytes(blockHeight)
	delta, err := ledger.GetStateDelta(blockHeight - 1)
	if nil != err {
		return err
	}

	logger.Debug("Got the delta state of block number %v", blockHeight)
	data, err := proto.Marshal(&pb.BlockState{Block: block, StateDelta: delta.Marshal()})
	if err != nil {
		return fmt.Errorf("Fail to marshall BlockState structure: %v", err)
	}

	logger.Debug("Broadcasting OpenchainMessage_SYNC_BLOCK_ADDED")
	msg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_SYNC_BLOCK_ADDED,
		Payload: data, Timestamp: util.CreateUtcTimestamp()}
	if errs := i.cpi.Broadcast(msg); nil != errs {
		return fmt.Errorf("Failed to broadcast with errors: %v", errs)
	}
	return nil
}

func (i *Noops) queueTransactions(txarr []*pb.Transaction) {
	i.txQ.Push(txarr)
	logger.Debug("Transaction queue size: %d", i.txQ.Size())
}
