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
	"fmt"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/statetransfer"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type Orderer interface {
	Checkpoint(seqNo uint64, id []byte)
	Validate(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID) // Replies with true,nil,_ if valid, true,nil,_ if transfer required, and false,_,_ to rollback
}

type Executor interface {
	Execute(seqNo uint64, txs []*pb.Transaction, execInfo *ExecutionInfo)
	BlockUntilIdle()
	ValidState(seqNo uint64, id []byte, peerIDs []*pb.PeerID)
	SkipTo(seqNo uint64, id []byte, peerIDs []*pb.PeerID)
}

type ExecutionInfo struct {
	Checkpoint bool // Whether to reply with a checkpoint once this request commits
	Validate   bool // Whether to validate the execution before committing
}

type syncTarget struct {
	seqNo     uint64
	peerIDs   []*pb.PeerID
	blockInfo *BlockInfo
}

type transaction struct {
	seqNo    uint64
	txs      []*pb.Transaction // nil has a special meaning, that this represents some missing/discarded requests and should possibly state transfer
	execInfo *ExecutionInfo
}

type obcExecutor struct {
	executionQueue chan *transaction
	completeSync   chan *syncTarget
	syncTargets    chan *syncTarget
	threadIdle     chan struct{}
	threadExit     chan struct{}
	lastExec       uint64
	id             uint64
	orderer        Orderer
	ledger         consensus.Ledger
	sts            *statetransfer.StateTransferState
	stack          consensus.Stack // TODO, populate
}

// TODO, should not identify with replicaID, should use PeerID
func NewOBCExecutor(id uint64, config *viper.Viper, queueSize int, orderer Orderer, ledger statetransfer.PartialStack) (obcex *obcExecutor) {
	obcex.executionQueue = make(chan *transaction, queueSize)
	obcex.syncTargets = make(chan *syncTarget)
	obcex.completeSync = make(chan *syncTarget)
	obcex.threadIdle = make(chan struct{})
	obcex.threadExit = make(chan struct{})
	obcex.orderer = orderer
	obcex.ledger = ledger
	obcex.id = id

	obcex.sts = statetransfer.NewStateTransferState(config, ledger)

	listener := struct{ statetransfer.ProtoListener }{}
	listener.CompletedImpl = obcex.stateTransferCompleted
	obcex.sts.RegisterListener(&listener)

	go obcex.queueThread()
	return
}

// Informs the other end of the syncTargets if someone is listening
func (obcex *obcExecutor) ValidState(seqNo uint64, id []byte, peerIDs []*pb.PeerID) {
	blockInfo := &BlockInfo{}
	if err := proto.Unmarshal(id, blockInfo); nil != err {
		logger.Error("Error unmarshalling id to blockInfo: %s", err)
		return
	}
	select {
	case obcex.syncTargets <- &syncTarget{
		seqNo:     seqNo,
		peerIDs:   peerIDs,
		blockInfo: blockInfo,
	}:
		logger.Debug("Sent sync target")
	default:
	}
}

// Enqueues a request for execution if there is room
func (obcex *obcExecutor) Execute(seqNo uint64, txs []*pb.Transaction, execInfo *ExecutionInfo) {
	request := &transaction{
		seqNo:    seqNo,
		txs:      txs,
		execInfo: execInfo,
	}
	select {
	case obcex.executionQueue <- request:
	default:
		logger.Error("Error queueing request (queue full) for sequence number %d", seqNo)
		for range obcex.executionQueue {
		} // Discard all outstanding requests
		obcex.executionQueue <- &transaction{
			seqNo: seqNo,
			// nil txRaw indicates a missed request
		}
		obcex.executionQueue <- request // queue request
	}

}

// Skips to a point further in the execution
func (obcex *obcExecutor) SkipTo(seqNo uint64, id []byte, peerIDs []*pb.PeerID) {
	logger.Debug("Skipping to new sequence number %d", seqNo)
	for range obcex.executionQueue {
	} // Discard all outstanding requests
	obcex.executionQueue <- &transaction{
		seqNo: seqNo,
		// nil txRaw indicates a missed request
	}

	blockInfo := &BlockInfo{}

	if err := proto.Unmarshal(id, blockInfo); nil != err {
		logger.Error("Error unmarshalling id to blockInfo: %s", err)
		return
	}

	select {
	case obcex.syncTargets <- &syncTarget{
		seqNo:     seqNo,
		peerIDs:   peerIDs,
		blockInfo: blockInfo,
	}:
		logger.Debug("Sent sync target")
	case <-obcex.threadExit:
		logger.Debug("Instructed to exit before sync target could be sent")
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

		if nil == transaction.txs {
			logger.Info("Executor queue apparently has a gap in it, initiating state transfer")
			obcex.sync()
			continue
		} else {
			obcex.execute(transaction)
		}
	}
}

// Performs state transfer, this is called only from the execution thread to prevent races
func (obcex *obcExecutor) sync() uint64 {
	obcex.sts.Initiate(nil)
	for {
		select {
		case target := <-obcex.syncTargets:
			obcex.sts.AddTarget(target.blockInfo.BlockNumber, target.blockInfo.BlockHash, target.peerIDs, target)
		case finish := <-obcex.completeSync:
			obcex.lastExec = finish.seqNo
			return finish.seqNo
		case <-obcex.threadExit:
			logger.Debug("Request for shutdown in sync")
			return 0
		}
	}
}

func (obcex *obcExecutor) getBlockchainSize() uint64 {
	blockHeight, err := obcex.ledger.GetBlockchainSize()
	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("Replica %d could not determine the block height, this indicates an irrecoverable situation: %s", obcex.id, err))
	}
	return blockHeight
}

func (obcex *obcExecutor) hashBlock(block *pb.Block) []byte {
	blockHashBytes, err := obcex.ledger.HashBlock(block)

	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("Replica %d could not compute its own block hash, this indicates an irrecoverable situation: %s", obcex.id, err))
	}

	return blockHashBytes
}

// Send a checkpoint ot the Orderer
func (obcex *obcExecutor) checkpoint(tx *transaction, block *pb.Block) {
	blockHeight := obcex.getBlockchainSize()
	blockHashBytes := obcex.hashBlock(block)

	idAsBytes, err := createID(blockHeight-1, blockHashBytes)

	if nil != err {
		logger.Error("Could not send checkpoint: %v", err)
		return
	}

	obcex.orderer.Checkpoint(tx.seqNo, idAsBytes)
}

func (obcex *obcExecutor) rollback(tx *transaction) {
	if ierr := obcex.stack.RollbackTxBatch(tx); ierr != nil {
		panic(fmt.Errorf("Unable to rollback transaction batch %p: %v", tx, ierr))
	}
}

func createID(blockNumber uint64, blockHashBytes []byte) ([]byte, error) {
	id := &BlockInfo{
		BlockNumber: blockNumber,
		BlockHash:   blockHashBytes,
	}

	return proto.Marshal(id)
}

// execute an opaque request which corresponds to an OBC Transaction, but does not commit
// note, this uses the tx pointer as the batchID
func (obcex *obcExecutor) prepareCommit(tx *transaction) error {

	if err := obcex.stack.BeginTxBatch(tx); err != nil {
		return fmt.Errorf("Failed to begin transaction batch %p: %v", tx, err)
	}

	if _, err := obcex.stack.ExecTxs(tx, tx.txs); nil != err {
		err = fmt.Errorf("Fail to execute transaction batch %p: %v", tx, err)
		logger.Error(err.Error())
		obcex.rollback(tx)
		return err
	}

	return nil
}

// commits the result from prepareCommit
func (obcex *obcExecutor) commit(tx *transaction) error {
	if block, err := obcex.stack.CommitTxBatch(tx, nil); err != nil {
		obcex.rollback(tx)
		return fmt.Errorf("Failed to commit transaction batch %p to the ledger: %v", tx, err)
	} else {
		obcex.lastExec = tx.seqNo
		if tx.execInfo.Checkpoint {
			obcex.checkpoint(tx, block)
		}

		return nil
	}
}

// first validates, then commits the result from prepareCommit
func (obcex *obcExecutor) validateAndCommit(tx *transaction) (err error) {
	block, err := obcex.stack.PreviewCommitTxBatch(tx, nil)
	if err != nil {
		return fmt.Errorf("Fail to preview transaction: %v", err)
	}

	defer func() {
		if nil != err {
			obcex.rollback(tx)
		}
	}()

	blockHeight := obcex.getBlockchainSize()
	blockHashBytes := obcex.hashBlock(block)

	idAsBytes, err := createID(blockHeight, blockHashBytes)

	if err != nil {
		return fmt.Errorf("Error creating the execution id: %v", err)
	}

	commit, correctedIDAsBytes, peerIDs := obcex.orderer.Validate(tx.seqNo, idAsBytes)

	if !commit {
		return fmt.Errorf("Was told not to commit the transaction, rolling back")
	}

	if nil != correctedIDAsBytes {
		correctedID := &BlockInfo{}
		if err := proto.Unmarshal(correctedIDAsBytes, correctedID); nil != err {
			return err
		}
		go func() {
			obcex.syncTargets <- &syncTarget{
				seqNo:     tx.seqNo,
				blockInfo: correctedID,
				peerIDs:   peerIDs,
			}
		}()
		if obcex.sync() == tx.seqNo && tx.execInfo.Checkpoint {
			// If the sync arrives at this sequence number, proceed and send out a checkpoint if needed
			// but it is possible the sync will arrive at a later point, in which case do not
			obcex.checkpoint(tx, block)
		}
	} else {
		obcex.commit(tx)
	}
	return nil
}

// perform the actual transaction execution
func (obcex *obcExecutor) execute(tx *transaction) error {
	if err := obcex.prepareCommit(tx); nil != err {
		return err
	}

	if tx.execInfo.Validate {
		return obcex.validateAndCommit(tx)
	} else {
		return obcex.commit(tx)
	}

}

// Handles finishing the state transfer by executing outstanding transactions
func (obcex *obcExecutor) stateTransferCompleted(blockNumber uint64, blockHash []byte, peerIDs []*pb.PeerID, metadata interface{}) {

	if md, ok := metadata.(*syncTarget); ok {
		logger.Debug("Replica %d completed state transfer to sequence number %d, about to resume request execution", obcex.id, obcex.lastExec)
		obcex.completeSync <- md
	} else {
		logger.Error("Replica %d was informed of a completed state transfer it did not initiate, this is indicative of a serious bug", obcex.id)
	}
}
