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

	"github.com/openblockchain/obc-peer/openchain/consensus/statetransfer"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type Orderer interface {
	Startup(seqNo uint64, id []byte)
	Checkpoint(seqNo uint64, id []byte)
	Validate(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID) // Replies with true,nil,_ if valid, true,nil,_ if transfer required, and false,_,_ to rollback
}

type Executor interface {
	Execute(seqNo uint64, txs []*pb.Transaction, execInfo *ExecutionInfo)
	ValidState(seqNo uint64, id []byte, peerIDs []*pb.PeerID, execInfo *ExecutionInfo)
	SkipTo(seqNo uint64, id []byte, peerIDs []*pb.PeerID, execInfo *ExecutionInfo)
	IdleChan() <-chan struct{}
}

// Orderers only need to implement the Checkpoint/Validate methods if they pass in the corresponding ExecutionInfo flag
type ExecutionInfo struct {
	Checkpoint bool // Whether to reply with a checkpoint once this request commits
	Validate   bool // Whether to validate the execution before committing
	Null       bool // Whether this request should be empty or not
}

type syncTarget struct {
	seqNo     uint64
	peerIDs   []*pb.PeerID
	blockInfo *BlockInfo
	execInfo  *ExecutionInfo
}

type transaction struct {
	seqNo    uint64
	txs      []*pb.Transaction // nil has a special meaning, that this represents some missing/discarded requests and should possibly state transfer
	execInfo *ExecutionInfo
	sync     bool
}

type obcExecutor struct {
	executionQueue chan *transaction
	completeSync   chan *syncTarget
	syncTargets    chan *syncTarget
	threadIdle     chan struct{}
	threadExit     chan struct{}
	lastExec       uint64
	id             *pb.PeerID
	orderer        Orderer
	sts            *statetransfer.StateTransferState
	executorStack  statetransfer.PartialStack
}

func NewOBCExecutor(config *viper.Viper, orderer Orderer, stack statetransfer.PartialStack) (obcex *obcExecutor) {
	var err error
	obcex = &obcExecutor{}
	queueSize := config.GetInt("executor.queuesize")
	if queueSize <= 0 {
		panic("Queue size must be positive")
	}

	obcex.executionQueue = make(chan *transaction, queueSize)
	obcex.syncTargets = make(chan *syncTarget)
	obcex.completeSync = make(chan *syncTarget)
	obcex.threadIdle = make(chan struct{})
	obcex.threadExit = make(chan struct{})
	obcex.orderer = orderer
	obcex.executorStack = stack
	obcex.id, _, err = stack.GetNetworkHandles()
	if nil != err {
		logger.Error("Could not resolve our own PeerID, assigning dummy")
		obcex.id = &pb.PeerID{"Dummy"}
	}

	logger.Info("Executor for %v using queue size of %d", obcex.id, queueSize)

	obcex.sts = statetransfer.NewStateTransferState(config, stack)

	listener := struct{ statetransfer.ProtoListener }{}
	listener.CompletedImpl = obcex.stateTransferCompleted
	obcex.sts.RegisterListener(&listener)

	go obcex.queueThread()
	return
}

func (obcex *obcExecutor) Stop() {
	logger.Debug("%v stopping executor", obcex.id)
	select {
	case <-obcex.threadExit:
	default:
		close(obcex.threadExit)
	}
	obcex.sts.Stop()
}

// Informs the other end of the syncTargets if someone is listening
func (obcex *obcExecutor) ValidState(seqNo uint64, id []byte, peerIDs []*pb.PeerID, execInfo *ExecutionInfo) {
	blockInfo := &BlockInfo{}
	if err := proto.Unmarshal(id, blockInfo); nil != err {
		logger.Error("%v error unmarshalling id to blockInfo: %s", obcex.id, err)
		return
	}
	select {
	case obcex.syncTargets <- &syncTarget{
		seqNo:     seqNo,
		peerIDs:   peerIDs,
		blockInfo: blockInfo,
		execInfo:  execInfo,
	}:
		logger.Debug("%v sent sync target", obcex.id)
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
		logger.Debug("%v queued request for sequence number %d", obcex.id, seqNo)
	default:
		logger.Error("%v error queueing request (queue full) for sequence number %d", obcex.id, seqNo)
		obcex.drainExecutionQueue()
		obcex.executionQueue <- &transaction{
			seqNo: seqNo,
			// nil txRaw indicates a missed request
		}
		obcex.executionQueue <- request // queue request
	}

}

// Skips to a point further in the execution
func (obcex *obcExecutor) SkipTo(seqNo uint64, id []byte, peerIDs []*pb.PeerID, execInfo *ExecutionInfo) {
	logger.Debug("%v skipping to new sequence number %d", obcex.id, seqNo)
	obcex.drainExecutionQueue()
	logger.Debug("%v queue cleared, queueing 'skip' transaction", obcex.id)
	obcex.executionQueue <- &transaction{
		seqNo:    seqNo,
		sync:     true,
		execInfo: execInfo,
	}

	blockInfo := &BlockInfo{}

	if err := proto.Unmarshal(id, blockInfo); nil != err {
		logger.Error("%v error unmarshalling id to blockInfo: %s", obcex.id, err)
		return
	}

	select {
	case obcex.syncTargets <- &syncTarget{
		seqNo:     seqNo,
		peerIDs:   peerIDs,
		blockInfo: blockInfo,
		execInfo:  execInfo,
	}:
		logger.Debug("%v sent sync target", obcex.id)
	case <-obcex.threadExit:
		logger.Debug("%v instructed to exit before sync target could be sent", obcex.id)
	}

}

// A channel which only reads when the executor thread is otherwise idle
func (obcex *obcExecutor) IdleChan() <-chan struct{} {
	return obcex.threadIdle
}

func (obcex *obcExecutor) drainExecutionQueue() {
	for {
		select {
		case <-obcex.executionQueue:
		default:
			return
		}
	} // Discard all outstanding requests
}

// Sends the startup message to the Orderer
func (obcex *obcExecutor) getCurrentInfo() (*BlockInfo, error) {
	logger.Debug("%v executor sending last checkpoint", obcex.id)
	height := obcex.getBlockchainSize()

	if 0 == height {
		return nil, fmt.Errorf("There are no blocks in the blockchain, including the genesis block")
	}

	headBlock, err := obcex.executorStack.GetBlock(height - 1)
	if nil != err {
		return nil, fmt.Errorf("Could not retrieve the latest block: %v", err)
	}

	blockHash := obcex.hashBlock(headBlock)

	return &BlockInfo{
		BlockNumber: height - 1,
		BlockHash:   blockHash,
	}, nil

}

// Loops until told to exit, waiting for and executing requests
func (obcex *obcExecutor) queueThread() {
	logger.Debug("%v executor thread starting", obcex.id)
	defer close(obcex.threadIdle) // When the executor thread exits, cause the threadIdle response to always return
	bi, err := obcex.getCurrentInfo()
	idAsBytes, err := obcex.createID(bi)
	if nil != err {
		panic(fmt.Sprintf("This is irrecoverable: %v", err))
	}
	obcex.orderer.Startup(0, idAsBytes) // TODO, set the sequence number based on some external storage

	var transaction *transaction
	idle := false
	for {
		if !idle {
			select {
			case <-obcex.threadExit:
				logger.Debug("%v executor thread requested to exit", obcex.id)
				return
			case transaction = <-obcex.executionQueue:
			default:
				idle = true
				continue
			}
		} else {
			select {
			case <-obcex.threadExit:
				logger.Debug("%v executor thread requested to exit", obcex.id)
				return
			case transaction = <-obcex.executionQueue:
				idle = false
			case obcex.threadIdle <- struct{}{}:
				logger.Debug("%v responding to idle request", obcex.id)
				continue

			}
		}

		blockHeight := obcex.getBlockchainSize()

		logger.Debug("%v executor thread attempting an execution for seqNo=%d, candidate for block %d", obcex.id, transaction.seqNo, blockHeight)
		if transaction.seqNo <= obcex.lastExec {
			logger.Debug("%v skipping execution of request for seqNo=%d (lastExec=%d)", obcex.id, transaction.seqNo, obcex.lastExec)
			continue
		}

		var err error
		var id *BlockInfo
		var seqNo uint64
		var sync bool

		if nil == transaction.execInfo {
			transaction.execInfo = &ExecutionInfo{}
		}

		execInfo := transaction.execInfo

		switch {
		case transaction.sync:
			logger.Info("%v executor queue apparently has a gap in it, initiating state transfer", obcex.id)
			sync = true
		case execInfo.Null:
			seqNo = transaction.seqNo
			id, err = obcex.getCurrentInfo()
			if nil != err {
				logger.Critical("Requested to send a checkpoint for a Null request, but could not create one: %v", err)
				continue
			}
		default:
			sync, id, err = obcex.execute(transaction)
		}

		if nil != err {
			logger.Error("%v encountered an error while processing transaction: %v", err)
			continue
		}

		if sync {
			logger.Debug("%v requested sync while processing transaction: %v", err)
			seqNo, id, execInfo = obcex.sync()
			if nil == id {
				// id should only be nil during shutdown
				continue
			}
		}

		if nil != err {
			logger.Error("%v encountered an error while performing state sync: %v", err)
			continue
		}

		if execInfo.Checkpoint {
			idAsBytes, _ := obcex.createID(id)
			logger.Debug("%v sending checkpoint: %x", obcex.id, idAsBytes)
			obcex.orderer.Checkpoint(seqNo, idAsBytes)
		}
	}
}

// Performs state transfer, this is called only from the execution thread to prevent races
func (obcex *obcExecutor) sync() (uint64, *BlockInfo, *ExecutionInfo) {
	logger.Debug("%v attempting to sync", obcex.id)
	obcex.sts.Initiate(nil)
	for {
		select {
		case target := <-obcex.syncTargets:
			logger.Debug("%v adding possible sync target", obcex.id)
			obcex.sts.AddTarget(target.blockInfo.BlockNumber, target.blockInfo.BlockHash, target.peerIDs, target)
		case finish := <-obcex.completeSync:
			logger.Debug("%v completed sync to seqNo %d", obcex.id, finish.seqNo)
			obcex.lastExec = finish.seqNo
			return finish.seqNo, finish.blockInfo, finish.execInfo
		case <-obcex.threadExit:
			logger.Debug("%v request for shutdown in sync", obcex.id)
			return 0, nil, nil
		}
	}
}

func (obcex *obcExecutor) getBlockchainSize() uint64 {
	blockHeight, err := obcex.executorStack.GetBlockchainSize()
	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("%v could not determine the block height, this indicates an irrecoverable situation: %s", obcex.id, err))
	}
	return blockHeight
}

func (obcex *obcExecutor) hashBlock(block *pb.Block) []byte {
	blockHashBytes, err := obcex.executorStack.HashBlock(block)

	if nil != err {
		// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
		panic(fmt.Errorf("%v could not compute its own block hash, this indicates an irrecoverable situation: %s", obcex.id, err))
	}

	return blockHashBytes
}

func (obcex *obcExecutor) rollback(tx *transaction) {
	logger.Debug("%v rolling back transaction %p", obcex.id, tx)
	if ierr := obcex.executorStack.RollbackTxBatch(tx); ierr != nil {
		panic(fmt.Errorf("%v unable to rollback transaction batch %p: %v", obcex.id, tx, ierr))
	}
}

func (obcex *obcExecutor) createID(id *BlockInfo) ([]byte, error) {
	logger.Debug("%v creating id for blockNumber %d and hash %x", obcex.id, id.BlockNumber, id.BlockHash)

	return proto.Marshal(id)
}

// execute an opaque request which corresponds to an OBC Transaction, but does not commit
// note, this uses the tx pointer as the batchID
func (obcex *obcExecutor) prepareCommit(tx *transaction) error {
	logger.Debug("%v preparing transaction %p to commit", obcex.id, tx)

	if err := obcex.executorStack.BeginTxBatch(tx); err != nil {
		return fmt.Errorf("Failed to begin transaction batch %p: %v", tx, err)
	}

	if _, err := obcex.executorStack.ExecTxs(tx, tx.txs); nil != err {
		err = fmt.Errorf("%v fail to execute transaction batch %p: %v", obcex.id, tx, err)
		logger.Error(err.Error())
		obcex.rollback(tx)
		return err
	}

	return nil
}

// commits the result from prepareCommit
func (obcex *obcExecutor) commit(tx *transaction) ([]byte, error) {
	logger.Debug("%v committing transaction %p", obcex.id, tx)
	if block, err := obcex.executorStack.CommitTxBatch(tx, nil); err != nil {
		obcex.rollback(tx)
		return nil, fmt.Errorf("Failed to commit transaction batch %p to the ledger: %v", tx, err)
	} else {
		obcex.lastExec = tx.seqNo
		id := obcex.hashBlock(block)
		logger.Debug("%v responding with id %x for transaction %p", obcex.id, id, tx)
		return id, nil
	}
}

// first validates, then commits the result from prepareCommit
func (obcex *obcExecutor) validate(tx *transaction) (sync bool, err error) {
	logger.Debug("%v previewing transaction %p", obcex.id, tx)
	block, err := obcex.executorStack.PreviewCommitTxBatch(tx, nil)
	if err != nil {
		return false, fmt.Errorf("Fail to preview transaction: %v", err)
	}

	defer func() {
		if nil != err {
			obcex.rollback(tx)
		}
	}()

	blockHeight := obcex.getBlockchainSize()
	blockHashBytes := obcex.hashBlock(block)

	idAsBytes, err := obcex.createID(&BlockInfo{
		BlockNumber: blockHeight, // Not -1, as the block is not yet committed
		BlockHash:   blockHashBytes,
	})

	if err != nil {
		return false, fmt.Errorf("Error creating the execution id: %v", err)
	}

	logger.Debug("%v validating transaction %p", obcex.id, tx)
	commit, correctedIDAsBytes, peerIDs := obcex.orderer.Validate(tx.seqNo, idAsBytes)

	if !commit {
		return false, fmt.Errorf("Was told not to commit the transaction, rolling back")
	}

	if nil != correctedIDAsBytes {
		logger.Debug("%v transaction %p results incorrect, recovering", obcex.id, tx)
		obcex.rollback(tx)
		correctedID := &BlockInfo{}
		if err := proto.Unmarshal(correctedIDAsBytes, correctedID); nil != err {
			logger.Critical("%v transaction %p did not unmarshal to the required BlockInfo: %v", obcex.id, tx, err)
			return true, err
		}
		go func() {
			obcex.syncTargets <- &syncTarget{
				seqNo:     tx.seqNo,
				blockInfo: correctedID,
				peerIDs:   peerIDs,
			}
		}()
		return true, nil
	}

	return false, nil
}

// perform the actual transaction execution
func (obcex *obcExecutor) execute(tx *transaction) (sync bool, blockInfo *BlockInfo, err error) {
	if err = obcex.prepareCommit(tx); nil != err {
		return
	}

	if tx.execInfo.Validate {
		sync, err = obcex.validate(tx)
	}

	if nil != err || sync {
		return
	}

	var blockHash []byte
	blockHash, err = obcex.commit(tx)

	if nil != err {
		return
	}

	blockHeight := obcex.getBlockchainSize()

	blockInfo = &BlockInfo{
		BlockNumber: blockHeight - 1,
		BlockHash:   blockHash,
	}

	return

}

// Handles finishing the state transfer by executing outstanding transactions
func (obcex *obcExecutor) stateTransferCompleted(blockNumber uint64, blockHash []byte, peerIDs []*pb.PeerID, metadata interface{}) {

	if md, ok := metadata.(*syncTarget); ok {
		logger.Debug("%v completed state transfer to sequence number %d with block number %d and hash %x, about to resume request execution", obcex.id, md.seqNo, blockNumber, blockHash)
		obcex.completeSync <- md
	} else {
		logger.Error("%v was informed of a completed state transfer it did not initiate, this is indicative of a serious bug", obcex.id)
	}
}
