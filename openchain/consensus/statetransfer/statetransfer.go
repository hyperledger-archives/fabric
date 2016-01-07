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

package statetransfer

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
)

// =============================================================================
// init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/obcpbft")
}

// =============================================================================
// public methods and structure definitions
// =============================================================================

type StateTransferState struct {
	ledger consensus.Ledger

	asynchronousTransferInProgress bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	id string // Useful for log messages only

	stateValid bool // Are we currently operating under the assumption that the state is valid?

	blockVerifyChunkSize uint64        // The max block length to attempt to sync at once, this prevents state transfer from being delayed while the blockchain is validated
	validBlockRanges     []*blockRange // Used by the block thread to track which pieces of the blockchain have already been hashed
	RecoverDamage        bool          // Whether state transfer should ever modify or delete existing blocks if they are determined to be corrupted

	initiateStateSync chan *syncMark       // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	blockHashReq      chan *blockHashReq   // Used to ask for particular valid block hashes, write only from the block hash receiver thread
	blockHashReceiver chan *blockHashReply // Used to process incoming valid block hashes, write only from the state thread
	blockSyncReq      chan *blockSyncReq   // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread
	completeStateSync chan uint64          // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread

	BlockRequestTimeout         time.Duration // How long to wait for a replica to respond to a block request
	StateDeltaRequestTimeout    time.Duration // How long to wait for a replica to respond to a state delta request
	StateSnapshotRequestTimeout time.Duration // How long to wait for a replica to respond to a state snapshot request

}

// Syncs to the block number specified, blocking until success
// If replicaIds is nil, all replicas will be considered sync candidates
// The function returns nil on success or error
func (sts *StateTransferState) SynchronousStateTransfer(blockNumber uint64, blockHash []byte, replicaIds []uint64) error {
	blockHeight, err := sts.ledger.GetBlockchainSize()
	if nil != err {
		return err
	}

	currentStateBlockNumber := blockHeight - 1
	blockHReply := &blockHashReply{
		syncMark: syncMark{
			blockNumber: blockNumber,
			replicaIds:  replicaIds,
		},
		blockHash: blockHash,
	}
	mark := &syncMark{
		blockNumber: blockNumber,
		replicaIds:  replicaIds,
	}
	blocksValid := false

	return sts.attemptStateTransfer(&currentStateBlockNumber, &mark, &blockHReply, &blocksValid)
}

// Syncs to at least the block number specified, without blocking
// For the sync to complete, a call to AsynchronousStateTransferValidHash(hash, replicaIds) must be made
// This call should be made any time a new valid block hash above lowBlock is discovered
// If replicaIds is nil, all replicas will be considered sync candidates
// The channel returned may be blocked on, returning the block number synced to,
// or alternatively, the calling thread may invoke AsynchronousStateTransferJustCompleted() which
// will check the channel in a non-blocking way
func (sts *StateTransferState) AsynchronousStateTransfer(lowBlock uint64, replicaIds []uint64) chan uint64 {
	sts.initiateStateSync <- &syncMark{
		blockNumber: lowBlock,
		replicaIds:  replicaIds,
	}
	sts.asynchronousTransferInProgress = true
	return sts.completeStateSync
}

// Informs the asynchronous sync of a new valid block hash, as well as a list of replicas which should be capable of supplying that block
// If the replicaIds are nil, then all replicas are assumed to have the given block
func (sts *StateTransferState) AsynchronousStateTransferValidHash(blockNumber uint64, blockHash []byte, replicaIds []uint64) {
	logger.Debug("%s informed of a new block hash for block number %d", sts.id, blockNumber)

	sts.blockHashReceiver <- &blockHashReply{
		syncMark: syncMark{
			blockNumber: blockNumber,
			replicaIds:  replicaIds,
		},
		blockHash: blockHash,
	}
}

func (sts *StateTransferState) AsynchronousStateTransferJustCompleted() (uint64, bool) {
	select {
	case blockNumber := <-sts.completeStateSync:
		return blockNumber, true
	default:
		return uint64(0), false
	}
}

func (sts *StateTransferState) AsynchronousStateTransferResultChannel() chan uint64 {
	return sts.completeStateSync
}

func (sts *StateTransferState) AsynchronousStateTransferInProgress() bool {
	return sts.asynchronousTransferInProgress
}

func (sts *StateTransferState) InvalidateState() {
	sts.stateValid = false
}

// =============================================================================
// constructors
// =============================================================================

func ThreadlessNewStateTransferState(id string, config *viper.Viper, ledger consensus.Ledger) *StateTransferState {
	sts := &StateTransferState{}

	sts.ledger = ledger

	sts.asynchronousTransferInProgress = false

	sts.RecoverDamage = config.GetBool("statetransfer.recoverdamage")

	sts.stateValid = true // Assume our starting state is correct unless told otherwise

	sts.validBlockRanges = make([]*blockRange, 0)
	sts.blockVerifyChunkSize = uint64(config.GetInt("statetransfer.blocksperrequest"))
	if sts.blockVerifyChunkSize == 0 {
		panic(fmt.Errorf("Must set statetransfer.blocksperrequest to be nonzero"))
	}

	sts.initiateStateSync = make(chan *syncMark)
	sts.blockHashReq = make(chan *blockHashReq)
	sts.blockHashReceiver = make(chan *blockHashReply)
	sts.blockSyncReq = make(chan *blockSyncReq)
	sts.completeStateSync = make(chan uint64)

	var err error

	sts.BlockRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.singleblock"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singleblock timeout: %s", err))
	}
	sts.StateDeltaRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.singlestatedelta"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singlestatedelta timeout: %s", err))
	}
	sts.StateSnapshotRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.fullstate"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.fullstate timeout: %s", err))
	}

	return sts
}

func NewStateTransferState(id string, config *viper.Viper, ledger consensus.Ledger) *StateTransferState {
	sts := ThreadlessNewStateTransferState(id, config, ledger)

	go sts.stateThread()
	go sts.blockThread()
	go sts.blockHashReceiverThread()

	return sts
}

// =============================================================================
// custom interfaces and structure definitions
// =============================================================================

type syncMark struct {
	blockNumber uint64
	replicaIds  []uint64
}

type blockHashReq struct {
	blockNumber uint64
	replyChan   chan *blockHashReply
}

type blockHashReply struct {
	syncMark
	blockHash []byte
}

type blockSyncReq struct {
	syncMark
	reportOnBlock  uint64
	replyChan      chan error
	firstBlockHash []byte
}

type blockRange struct {
	highBlock   uint64
	lowBlock    uint64
	lowNextHash []byte
}

type blockRangeSlice []*blockRange

func (a blockRangeSlice) Len() int {
	return len(a)
}
func (a blockRangeSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a blockRangeSlice) Less(i, j int) bool {
	if a[i].highBlock == a[j].highBlock {
		// If the highs match, the bigger range comes first
		return a[i].lowBlock > a[j].lowBlock
	}
	return a[i].highBlock < a[j].highBlock
}

func (a blockRangeSlice) Delete(i int) {
	for j := i; j < len(a)-1; j++ {
		a[j] = a[j+1]
	}
	a = a[:len(a)-1]
}

// =============================================================================
// helper functions for state transfer
// =============================================================================

// Executes a func trying each replica included in replicaIds until successful
// Attempts to execute over all replicas if replicaIds is nil
func (sts *StateTransferState) tryOverReplicas(replicaIds []uint64, do func(replicaId uint64) error) (err error) {
	startIndex := uint64(rand.Int() % 4) // TODO this is very wrong, all these 4's should be replicaCount, but there are other problems
	if nil == replicaIds {
		for i := 0; i < 4; i++ {
			index := (uint64(i) + startIndex) % uint64(4)
			// TODO, this is broken, need some way to get a list of all replicaIds, cannot assume they are sequential from 0
			err = do(index)
			if err == nil {
				break
			} else {
				logger.Warning("%s", err)
			}
		}
	} else {
		numReplicas := uint64(len(replicaIds))

		for i := uint64(0); i < uint64(numReplicas); i++ {
			index := (i + startIndex) % numReplicas
			err = do(replicaIds[index])
			if err == nil {
				break
			} else {
				logger.Warning("%s in tryOverReplicas loop : %s", sts.id, err)
			}
		}
	}

	return err

}

// Attempts to complete a blockSyncReq using the supplied replicas
// Will return the last block number attempted to sync, and the last block successfully synced (or nil) and error on failure
// This means on failure, the returned block corresponds to 1 higher than the returned block number
func (sts *StateTransferState) syncBlocks(highBlock, lowBlock uint64, highHash []byte, replicaIds []uint64) (uint64, *protos.Block, error) {
	logger.Debug("%s syncing blocks from %d to %d", sts.id, highBlock, lowBlock)
	validBlockHash := highHash
	blockCursor := highBlock
	var block *protos.Block

	err := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {
		blockChan, err := sts.ledger.GetRemoteBlocks(replicaId, blockCursor, lowBlock)
		if nil != err {
			logger.Warning("%s failed to get blocks from %d to %d from replica %d: %s",
				sts.id, blockCursor, lowBlock, replicaId, err)
			return err
		}
		for {
			select {
			case syncBlockMessage, ok := <-blockChan:

				if !ok {
					return fmt.Errorf("Channel closed before we could finish reading")
				}

				if syncBlockMessage.Range.Start < syncBlockMessage.Range.End {
					// If the message is not replying with blocks backwards, we did not ask for it
					continue
				}

				var i int
				for i, block = range syncBlockMessage.Blocks {
					// It is possible to get duplication or out of range blocks due to an implementation detail, we must check for them
					if syncBlockMessage.Range.Start-uint64(i) != blockCursor {
						continue
					}

					testHash, err := sts.ledger.HashBlock(block)
					if nil != err {
						return fmt.Errorf("%s got a block %d which could not hash from replica %d: %s",
							sts.id, blockCursor, replicaId, err)
					}

					if !bytes.Equal(testHash, validBlockHash) {
						return fmt.Errorf("%s got block %d from replica %d with hash %x, was expecting hash %x",
							sts.id, blockCursor, replicaId, testHash, validBlockHash)
					}

					logger.Debug("%s putting block %d to with PreviousBlockHash %x and StateHash %x", sts.id, blockCursor, block.PreviousBlockHash, block.StateHash)
					if !sts.RecoverDamage {

						// If we are not supposed to be destructive in our recovery, check to make sure this block doesn't already exist
						if oldBlock, err := sts.ledger.GetBlock(blockCursor); err == nil && oldBlock != nil {
							oldBlockHash, err := sts.ledger.HashBlock(oldBlock)
							if nil == err {
								if !bytes.Equal(oldBlockHash, validBlockHash) {
									panic("The blockchain is corrupt and the configuration has specified that bad blocks should not be deleted/overridden")
								}
							} else {
								logger.Error("%s could not compute the hash of block %d", sts.id, blockCursor)
								panic("The blockchain is corrupt and the configuration has specified that bad blocks should not be deleted/overridden")
							}
							logger.Debug("%s not actually putting block %d to with PreviousBlockHash %x and StateHash %x, as it already exists", sts.id, blockCursor, block.PreviousBlockHash, block.StateHash)
						} else {
							sts.ledger.PutBlock(blockCursor, block)
						}
					} else {
						sts.ledger.PutBlock(blockCursor, block)
					}

					validBlockHash = block.PreviousBlockHash

					if blockCursor == lowBlock {
						logger.Debug("%s successfully synced from block %d to block %d", sts.id, highBlock, lowBlock)
						return nil
					}
					blockCursor--

				}
			case <-time.After(sts.BlockRequestTimeout):
				return fmt.Errorf("%s had block sync request to %d time out", sts.id, replicaId)
			}
		}
	})

	if nil != block {
		logger.Debug("%s returned from sync with block %d and hash %x", sts.id, blockCursor, block.StateHash)
	} else {
		logger.Debug("%s returned from sync with no new blocks", sts.id)
	}

	return blockCursor, block, err

}

func (sts *StateTransferState) syncBlockchainToCheckpoint(blockSyncReq *blockSyncReq) {

	logger.Debug("%s is processing a blockSyncReq to block %d", sts.id, blockSyncReq.blockNumber)

	blockchainSize, err := sts.ledger.GetBlockchainSize()

	if nil != err {
		panic("We can't determine how long our blockchain is, this is irrecoverable")
	}

	if blockSyncReq.blockNumber < blockchainSize {
		if !sts.RecoverDamage {
			panic("The blockchain height is higher than advertised by consensus, the configuration has specified that bad blocks should not be deleted/overridden, so we cannot proceed")
		} else {
			// TODO For now, unimplemented because we have no way to delete blocks
			panic("Our blockchain is already higher than a sync target, this is unlikely, but unimplemented")
		}
	} else {

		blockNumber, block, err := sts.syncBlocks(blockSyncReq.blockNumber, blockSyncReq.reportOnBlock, blockSyncReq.firstBlockHash, blockSyncReq.replicaIds)

		goodRange := &blockRange{
			highBlock: blockSyncReq.blockNumber,
		}

		if nil != blockSyncReq.replyChan {
			blockSyncReq.replyChan <- err
			goodRange.lowBlock = blockNumber
		}

		goodRange.lowNextHash = block.PreviousBlockHash
		sts.validBlockRanges = append(sts.validBlockRanges, goodRange)
	}
}

func (sts *StateTransferState) VerifyAndRecoverBlockchain() bool {

	if 0 == len(sts.validBlockRanges) {
		size, err := sts.ledger.GetBlockchainSize()
		if nil != err {
			panic("We cannot determine how long our blockchain is, this is irrecoverable")
		}
		if 0 == size {
			logger.Warning("%s has no blocks in its blockchain, including the genesis block", sts.id)
			return false
		}

		block, err := sts.ledger.GetBlock(size - 1)
		if nil != err {
			logger.Warning("%s could not retrieve its head block %d: %s", sts.id, size, err)
			return false
		}

		sts.validBlockRanges = append(sts.validBlockRanges, &blockRange{
			highBlock:   size - 1,
			lowBlock:    size - 1,
			lowNextHash: block.PreviousBlockHash,
		})

	}

	sort.Sort(blockRangeSlice(sts.validBlockRanges))

	lowBlock := sts.validBlockRanges[0].lowBlock

	if 1 == len(sts.validBlockRanges) {
		if 0 == lowBlock {
			// We have exactly one valid block range, and it is from 0 to at least the block height at startup, consider the chain valid
			return true
		}
	}

	lowNextHash := sts.validBlockRanges[0].lowNextHash
	targetBlock := uint64(0)

	if 1 < len(sts.validBlockRanges) {
		if sts.validBlockRanges[1].highBlock+1 >= lowBlock {
			// Ranges are not distinct (or are adjacent), we will collapse them or discard the lower if it does not chain
			if sts.validBlockRanges[1].lowBlock < lowBlock {
				// Range overlaps or is adjacent
				block, err := sts.ledger.GetBlock(lowBlock - 1) // Subtraction is safe here, lowBlock > 0
				if nil != err {
					logger.Warning("%s could not retrieve block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
				} else {
					if blockHash, err := sts.ledger.HashBlock(block); nil != err {
						if bytes.Equal(blockHash, lowNextHash) {
							// The chains connect, no need to validate all the way down
							sts.validBlockRanges[0].lowBlock = sts.validBlockRanges[1].lowBlock
							sts.validBlockRanges[0].lowNextHash = sts.validBlockRanges[1].lowNextHash
						} else {
							logger.Warning("%s detected that a block range starting at %d previously believed to be valid did not hash correctly", sts.id, lowBlock-1)
						}
					} else {
						logger.Warning("%s could not hash block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
					}
				}
			} else {
				// Range is a subset, we will simply delete
			}

			// If there was an error validating or retrieving, delete, if it was successful, delete
			blockRangeSlice(sts.validBlockRanges).Delete(1)
			return false
		}

		// Ranges are distinct and not adjacent
		targetBlock = sts.validBlockRanges[1].highBlock
	}

	if targetBlock+sts.blockVerifyChunkSize > lowBlock {
		// The sync range is small enough
	} else {
		// targetBlock >=0, targetBlock+blockVeriyChunkSize <= lowBlock --> lowBlock - blockVerifyChunkSize >= 0
		targetBlock = lowBlock - sts.blockVerifyChunkSize
	}

	blockNumber, block, err1 := sts.syncBlocks(lowBlock-1, targetBlock, lowNextHash, nil)

	if blockHash, err2 := sts.ledger.HashBlock(block); nil == err2 {
		if nil == err1 {
			sts.validBlockRanges[0].lowBlock = blockNumber
			sts.validBlockRanges[0].lowNextHash = blockHash
		} else {
			sts.validBlockRanges[0].lowBlock = blockNumber + 1
			sts.validBlockRanges[0].lowNextHash = blockHash
		}
	} else {
		if nil == err1 {
			logger.Warning("%s just recovered block %d but cannot compute its hash: %s", sts.id, blockNumber, err2)
		} else {
			logger.Warning("%s just recovered block %d but cannot compute its hash: %s", sts.id, blockNumber+1, err2)
		}
	}

	return false
}

func (sts *StateTransferState) blockHashReceiverThread() {
	var lastHashReceived *blockHashReply
	for {
		select {
		case request := <-sts.blockHashReq:
			logger.Debug("%s block hash request thread received a block hash request for block greater than %d", sts.id, request.blockNumber)
			for {
				if nil == lastHashReceived {
					logger.Debug("%s block hash request thread waiting for new block hash", sts.id)
					lastHashReceived = <-sts.blockHashReceiver
				}

				if request.blockNumber > lastHashReceived.blockNumber {
					logger.Debug("%s block hash request thread did not received an appropriate block hash, block number was %d", sts.id, request.blockNumber)
					// The hash is not for a sufficiently high block number
					lastHashReceived = nil
					continue
				}

				logger.Debug("%s replying to block hash request with block %d and hash (%x)", sts.id, lastHashReceived.blockNumber, lastHashReceived.blockHash)
				request.replyChan <- lastHashReceived
				lastHashReceived = nil
			}
		case lastHashReceived = <-sts.blockHashReceiver:
		}
	}
}

func (sts *StateTransferState) blockThread() {

	for {

		select {
		case blockSyncReq := <-sts.blockSyncReq:
			sts.syncBlockchainToCheckpoint(blockSyncReq)
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
			if !sts.VerifyAndRecoverBlockchain() {
				// There is more verification to be done, so loop
				continue
			}
		}

		logger.Debug("%s has validated its blockchain to the genesis block", sts.id)

		// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
		blockSyncReq := <-sts.blockSyncReq
		sts.syncBlockchainToCheckpoint(blockSyncReq)
	}
}

func (sts *StateTransferState) attemptStateTransfer(currentStateBlockNumber *uint64, mark **syncMark, blockHReply **blockHashReply, blocksValid *bool) error {
	var err error
	//currentStateBlockNumber := *currentStateBlockNumberP
	//blockHReply := *blockHReplyP
	//blocksValid := *blocksValidP
	//mark := *syncMarkP

	if !sts.stateValid {
		// Our state is currently bad, so get a new one
		*currentStateBlockNumber, err = sts.syncStateSnapshot((*mark).blockNumber, (*mark).replicaIds)

		if nil != err {
			*mark = &syncMark{ // Let's try to just sync state from anyone, for any sequence number
				blockNumber: 0,
				replicaIds:  nil,
			}
			return fmt.Errorf("%s could not retrieve state as recent as advertised checkpoints above %d, indicates byzantine of f+1", sts.id, (*mark).blockNumber)
		}

		logger.Debug("%s completed state transfer to block %d", sts.id, *currentStateBlockNumber)
	} else {
		*currentStateBlockNumber, err = sts.ledger.GetBlockchainSize()
		if nil != err {
			panic(fmt.Errorf("Cannot get our blockchain size, this is irrecoverable: %s", err))
		}
		*currentStateBlockNumber-- // The block height is one more than the latest block number
	}

	// TODO, eventually we should allow lower block numbers and rewind transactions as needed
	if nil == *blockHReply || (*blockHReply).blockNumber < *currentStateBlockNumber {

		if nil == *blockHReply {
			logger.Debug("%s has no valid block hash to validate the blockchain with yet, waiting for a known valid block hash", sts.id)
		} else {
			logger.Debug("%s already has valid blocks through %d but needs to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
		}

		replyChan := make(chan *blockHashReply)
		sts.blockHashReq <- &blockHashReq{
			blockNumber: *currentStateBlockNumber,
			replyChan:   replyChan,
		}

		*blockHReply = <-replyChan
		*blocksValid = false // If we retrieve a new hash, we will need to sync to a new block
	}

	if !*blocksValid {
		(*mark) = &syncMark{ // We now know of a more recent block hash
			blockNumber: (*blockHReply).blockNumber,
			replicaIds:  (*blockHReply).replicaIds,
		}

		blockReplyChannel := make(chan error)

		sts.blockSyncReq <- &blockSyncReq{
			syncMark:       *(*mark),
			reportOnBlock:  *currentStateBlockNumber,
			replyChan:      blockReplyChannel,
			firstBlockHash: (*blockHReply).blockHash,
		}

		logger.Debug("%s state transfer thread waiting for block sync to complete", sts.id)
		err = <-blockReplyChannel

		if err != nil {
			return fmt.Errorf("%s could not retrieve blocks as recent as %d as the block hash advertised", sts.id, (*mark).blockNumber)
		}

		*blocksValid = true
	} else {
		logger.Debug("%s already has valid blocks through %d necessary to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
	}

	stateHash, err := sts.ledger.GetCurrentStateHash()
	if nil != err {
		sts.stateValid = false
		return fmt.Errorf("%s could not compute its current state hash: %s", sts.id, err)

	}

	block, err := sts.ledger.GetBlock(*currentStateBlockNumber)
	if nil != err {
		*blocksValid = false
		return fmt.Errorf("%s believed its state for block %d to be valid, but it could not retrieve it : %s", sts.id, *currentStateBlockNumber, err)
	}

	if !bytes.Equal(stateHash, block.StateHash) {
		sts.stateValid = false
		if sts.stateValid {
			return fmt.Errorf("%s believed its state for block %d to be valid, but its hash (%x) did not match the recovered blockchain's (%x)", sts.id, (*blockHReply).blockNumber, stateHash, block.StateHash)
		} else {
			return fmt.Errorf("%s recovered to an incorrect state at block number %d, (%x %x) retrying", sts.id, *currentStateBlockNumber, stateHash, block.StateHash)
		}
	}

	sts.stateValid = true

	if *currentStateBlockNumber < (*blockHReply).blockNumber {
		*currentStateBlockNumber, err = sts.playStateUpToCheckpoint(*currentStateBlockNumber+uint64(1), (*blockHReply).blockNumber, (*blockHReply).replicaIds)
		if nil != err {
			// This is unlikely, in the future, we may wish to play transactions forward rather than retry
			sts.stateValid = false
			return fmt.Errorf("%s was unable to play the state from block number %d forward to block %d, retrying with new state : %s", sts.id, *currentStateBlockNumber, (*blockHReply).blockNumber, err)
		}
	}

	return nil
}

// A thread to process state transfer
func (sts *StateTransferState) stateThread() {
	for {
		// Wait for state sync to become necessary
		mark := <-sts.initiateStateSync

		logger.Debug("%s is initiating state transfer", sts.id)

		var currentStateBlockNumber uint64
		var blockHReply *blockHashReply
		blocksValid := false

		for {
			if err := sts.attemptStateTransfer(&currentStateBlockNumber, &mark, &blockHReply, &blocksValid); err != nil {
				logger.Error("%s", err)
				continue
			}

			break
		}

		sts.asynchronousTransferInProgress = false
		sts.completeStateSync <- blockHReply.blockNumber
	}
}

func (sts *StateTransferState) playStateUpToCheckpoint(fromBlockNumber, toBlockNumber uint64, replicaIds []uint64) (uint64, error) {
	currentBlock := fromBlockNumber
	err := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {

		deltaMessages, err := sts.ledger.GetRemoteStateDeltas(replicaId, currentBlock, toBlockNumber)
		if err != nil {
			return fmt.Errorf("%s received an error while trying to get the state deltas for blocks %d through %d from replica %d", sts.id, fromBlockNumber, toBlockNumber, replicaId)
		}

		for {
			select {
			case deltaMessage, ok := <-deltaMessages:
				if !ok {
					if currentBlock == toBlockNumber+1 {
						return nil
					}
					return fmt.Errorf("%s was only able to recover to block number %d when desired to recover to %d", sts.id, currentBlock, toBlockNumber)
				}

				if deltaMessage.Range.Start != currentBlock || deltaMessage.Range.End < deltaMessage.Range.Start || deltaMessage.Range.End > toBlockNumber {
					continue // this is an unfortunately normal case, as we can get duplicates, just ignore it
				}
				for _, delta := range deltaMessage.Deltas {
					sts.ledger.ApplyStateDelta(delta, false)
				}

				success := false

				testBlock, err := sts.ledger.GetBlock(deltaMessage.Range.End)

				if nil != err {
					logger.Warning("%s could not retrieve block %d, though it should be present", sts.id, deltaMessage.Range.End)
				} else {

					stateHash, err := sts.ledger.GetCurrentStateHash()
					if nil != err {
						logger.Warning("%s could not compute its state hash for some reason: %s", sts.id, err)
					}
					logger.Debug("%s has played state forward from replica %d to block %d with StateHash (%x), the corresponding block has StateHash (%x)",
						sts.id, replicaId, deltaMessage.Range.End, stateHash, testBlock.StateHash)

					if bytes.Equal(testBlock.StateHash, stateHash) {
						success = true
					}
				}

				if !success {
					for _, delta := range deltaMessage.Deltas {
						sts.ledger.ApplyStateDelta(delta, true)
					}

					// TODO, this is wrong, our state might not rewind correctly, change to the new commit/rollback API
					return fmt.Errorf("%s played state forward according to replica %d, but the state hash did not match", sts.id, replicaId)
				} else {
					currentBlock++
				}
			case <-time.After(sts.StateDeltaRequestTimeout):
				logger.Warning("%s timed out during state delta recovery from replica %d", sts.id, replicaId)
				return fmt.Errorf("%s timed out during state delta recovery from replica %d", sts.id, replicaId)
			}
		}

		return nil

	})
	return currentBlock, err
}

// This function will retrieve the current state from a replica.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (sts *StateTransferState) syncStateSnapshot(minBlockNumber uint64, replicaIds []uint64) (uint64, error) {

	currentStateBlock := uint64(0)

	ok := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {
		logger.Debug("%s is initiating state recovery from from replica %d", sts.id, replicaId)

		sts.ledger.EmptyState()

		stateChan, err := sts.ledger.GetRemoteStateSnapshot(replicaId)

		if err != nil {
			sts.ledger.EmptyState()
			return err
		}

		timer := time.NewTimer(sts.StateSnapshotRequestTimeout)

		for {
			select {
			case piece, ok := <-stateChan:
				if !ok {
					return nil
				}
				sts.ledger.ApplyStateDelta(piece.Delta, false)
				currentStateBlock = piece.BlockNumber
			case <-timer.C:
				logger.Warning("%s timed out during state recovery from replica %d", sts.id, replicaId)
				return fmt.Errorf("%s timed out during state recovery from replica %d", sts.id, replicaId)
			}
		}

	})

	return currentStateBlock, ok
}
