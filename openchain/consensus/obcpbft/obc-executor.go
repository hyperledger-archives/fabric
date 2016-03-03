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

package obcpbft

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type Orderer interface {
	Checkpoint(seqNo uint64, id []byte)
	Validate(seqNo uint64, id []byte) (id []byte, err error) // Replies with nil,nil if valid, id,nil if transfer required, and nil,err if unsuccessful
}

type Executor interface {
	Execute(seqNo uint64, txRaw []byte, execInfo *ExecutionInfo)
	ValidState(seqNo uint64, id []byte)
}

type ExecutionInfo struct {
	Checkpoint bool // Whether to reply with a checkpoint once this request commits
	Validate   bool // Whether to validate the execution before committing
}

type syncTarget struct {
	seqNo   uint64
	id      []byte
	sources []uint64
}

type syncMetadata struct {
	blockInfo
	seqNo uint64
}

type transaction struct {
	seqNo    uint64
	txRaw    []byte // nil has a special meaning, that this represents some missing/discarded requests and should possibly state transfer
	execInfo ExecutionInfo
}

type obcExecutor struct {
	executionQueue chan *transaction
	completeSync   chan *syncMetadata
	syncTargets    chan *syncTarget
	threadIdle     chan struct{}
	threadExit     chan struct{}
	lastExec       uint64
	source         Orderer
	ledger         consensus.ReadOnlyLedger
	sts            *statetransfer.StateTransferState
}

func () NewOBCExecutor(uint64 id, config *viper.Viper, queueSize int, source Orderer, ledger consensus.ReadOnlyLedger) (obcex *protoimpl) {
	obcex.executionQueue = make(chan *transaction, queueSize)
	obcex.syncTargets = make(chan *syncTarget)
	obcex.completeSync = make(chan *syncMetadata)
	obcex.threadIdle = make(chan struct{})
	obcex.threadExit = make(chan struct{})
	obcex.orderer = orderer
	obcex.ledger = ledger
	obcex.internalLock = &sync.Mutex

	// TODO init
	obcex.sts = statetransfer.NewStateTransferState(fmt.Sprintf("vp%d", id), config, ledger, nil)

	listener := struct{ statetransfer.ProtoListener }{}
	listener.CompletedImpl = obcex.stateTransferCompleted
	obcex.sts.RegisterListener(&listener)

	go obcex.queueThread()
}

// Informs the other end of the syncTargets if someone is listening
func (obcex *obcExecutor) ValidState(seqNo uint64, id []byte, sources []uint64) {
	select {
	case obcex.syncTargets <- &syncTarget{
		seqNo:   seqNo,
		id:      id,
		sources: sources,
	}:
	default:
	}
}

// Enqueues a request for execution if there is room
func (obcex *obcExecutor) Execute(seqNo uint64, txRaw []byte, execInfo *ExecutionInfo) {
	request := &execution{
		seqNo:    seqNo,
		txRaw:    txRaw,
		execInfo: execInfo,
	}
	select {
	case obcex.executionQueue <- request:
	default:
		logger.Error("Error queueing request (queue full) for sequence number %d", seqNo)
		obcex.executionQueue = make(chan *execution) // Discard all outstanding requests
		obcex.executionQueue <- &execution{
			seqNo: seqNo,
			// nil txRaw indicates a missed request
		}
		obcex.executionQueue <- request // queue request
	}

}

// Blocks until the execution thread is idle
func (obcex *obcExecutor) BlockUntilIdle() {
	obcex.threadIdle <- struct{}{}
}

// Loops until told to exit, waiting for and executing requests
func (obcex *obcExecutor) queueThread() {
	var transaction *transaction
	idle := false
	for {
		if !idle {
			select {
			case <-obcex.threadExit:
				logger.Debug("Executor thread requested to exit")
				return
			case transaction = <-obcex.executionQueue:
			default:
				idle = true
			}
		} else {
			select {
			case <-obcex.threadExit:
				logger.Debug("Executor thread requested to exit")
				return
			case transaction = <-obcex.executionQueue:
				idle = false
			case <-obcex.threadIdle:
				continue

			}
		}

		logger.Debug("Executor thread attempting an execution for seqNo=%d", transaction.seqNo)
		if transaction.seqNo <= obcex.lastExec {
			logger.Debug("Skipping execution of request for seqNo=%d (lastExec=%d)", transaction.seqNo, obcex.lastExec)
			continue
		}

		if nil == transaction.txRaw {
			logger.Info("Executor queue apparently has a gap in it, initiating state transfer")
			obcex.sync()
			continue
		} else {
			op.execute(transaction)
		}
	}
}

// Performs state transfer, this is called only from the execution thread to prevent races
func (obcex *obcExecutor) sync() uint64 {
	obcex.sts.Initiate(nil)
	select {
	case target := <-obcex.syncTargets:
		blockInfo := &BlockInfo{}
		if err := proto.Unmarshal(target.id, blockInfo); nil != err {
			logger.Error("Error unmarshalling id to blockInfo: %s", err)
		}
		peerIDs := make([]*pb.PeerID, len(target.sources))
		for i, num := range target.sources {
			peerIDs[i] = fmt.Sprintf("vp%d", num)
		}
		obcex.sts.AddTarget(blockInfo.BlockNumber, blockInfo.BlockHash, peerIDs, blockInfo)
	case finish := <-obcex.completeSync:
		for tx := range executionQueue {
			obcex.lastExec = tx.seqNo
		}
	case <-obcex.threadExit:
		logger.Debug("Request for shutdown in sync")
	}
}

// Send a checkpoint ot the Orderer
func (obcex *obcExecutor) checkpoint(tx *transaction) {
	blockHeight, err := obcex.ledger.GetBlockchainSize()
	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("Replica %d could not determine the block height, this indicates an irrecoverable situation: %s", instance.id, err))
	}

	blockHashBytes, err := obcex.ledger.HashBlock(block)

	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("Replica %d could not compute its own block hash, this indicates an irrecoverable situation: %s", instance.id, err))
	}

	id := &BlockInfo{
		BlockNumber: blockHeight - 1,
		BlockHash:   blockHashBytes,
	}

	idAsBytes, _ := proto.Marshal(id)

	orderer.Checkpoint(seqNo, idAsBytes)
}

// execute an opaque request which corresponds to an OBC Transaction, but does not commit
// note, this uses the tx pointer as the batchID
func (obcex *obcExecutor) prepareCommit(tx *transaction) error {
	tb := &pb.TransactionBlock{}
	if err := proto.Unmarshal(tx.txRaw, tb); err != nil {
		return err
	}

	txs := tb.Transactions

	for i, tx := range txs {
		txRaw, _ := proto.Marshal(tx)
		if err = op.validate(txRaw); err != nil {
			return fmt.Errorf("Request in transaction %d from batch %p did not verify: %s", i, tx, err)
		}
	}

	if err := op.stack.BeginTxBatch(tx); err != nil {
		return fmt.Errorf("Failed to begin transaction batch %p: %v", tx, err)
	}

	if _, err := op.stack.ExecTxs(tx, txs); nil != err {
		err = fmt.Errorf("Fail to execute transaction batch %p: %v", tx, err)
		logger.Error(err.Error())
		if ierr = op.stack.RollbackTxBatch(tx); ierr != nil {
			panic(fmt.Errorf("Unable to rollback transaction batch %p: %v", tx, ierr))
		}
		return err
	}
}

// commits the result from prepareCommit
func (obcex *obcExecutor) commit(tx *transaction) error {
	if block, err = op.stack.CommitTxBatch(tx, nil); err != nil {
		return nil, fmt.Errorf("Failed to commit transaction batch %p to the ledger: %v", tx, err)
		if err := op.stack.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction batch %p: %v", tx, err))
		}
	} else {
		obcex.lastExec = transaction.seqNo
		if tx.execInfo.Checkpoint {
			return obcex.checkpoint(tx)
		} else {
			return nil
		}
	}
}

// first validates, then commits the result from prepareCommit
func (obcex *obcExecutor) validateAndCommit(tx *transaction) error {
	if block, err = op.stack.PreviewCommitTxBatch(txBatchID, nil); err != nil {
		return nil, fmt.Errorf("Fail to preview transaction: %v", err)
	}

	blockHeight, err := obcex.ledger.GetBlockchainSize()
	if nil != err {
		return nil, err
	}

	blockHashBytes, err := obcex.ledger.HashBlock(block)

	if nil != err {
		return nil, err
	}

	id := &BlockInfo{
		BlockNumber: blockHeight, // not - 1 since the block has not been committed yet
		BlockHash:   blockHashBytes,
	}

	idAsBytes, _ := proto.Marshal(id)

	if nil != err {
		return nil, err

	}

	id, err := obcex.source.Validate(tx.seqNo, hash)

	if nil != err {
		return err
	}

	if nil != id {
		go func() {
			obcex.syncTargets <- &syncTarget{
				// TODO add peer list (efficiency)
				id,
				seqNo: tx.seqNo,
			}
		}()
		if obcex.sync() == tx.seqNo && tx.execInfo.Checkpoint {
			// If the sync arrives at this sequence number, proceed and send out a checkpoint if needed
			// but it is possible the sync will arrive at a later point, in which case do not
			obcex.checkpoint(tx)
		}
	} else {
		obcex.commit(tx)
	}
}

// perform the actual transaction execution
func (obcex *obcExecutor) execute(tx *transaction) error {
	if err := obcex.prepareCommit(tx); nil != err {
		return nil, err
	}

	var block *pb.Block

	if tx.execInfo.Validate {
		return obcex.validateAndCommit(tx)
	} else {
		return obcex.commit(tx)
	}

}

// Handles finishing the state transfer by executing outstanding transactions
func (obcex *obcExecutor) stateTransferCompleted(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID, metadata interface{}) {

	if md, ok := metadata.(*syncMetadata); ok {
		logger.Debug("Replica %d completed state transfer to sequence number %d, about to resume request execution", obcex.id, obcex.lastExec)
		obcex.completeSync <- md
	} else {
		logger.Error("Replica %d was informed of a completed state transfer it did not initiate, this is indicative of a serious bug", instance.id)
	}
}
