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
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
)

type syncMark struct {
	blockNumber uint64
	replicaIds  []uint64
}

type ckptWeakCertReq struct {
	blockNumber uint64
	certChan    chan *chkptWeakCert
}

type chkptWeakCert struct {
	syncMark
	blockHash []byte
}

type blockSyncReq struct {
	syncMark
	reportOnBlock  uint64
	replyChan      chan *blockSyncReply
	firstBlockHash []byte
}

type blockSyncReply struct {
	blockNumber uint64
	stateHash   []byte
	err         error
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

type sortableUint64Slice []uint64

func (a sortableUint64Slice) Len() int {
	return len(a)
}
func (a sortableUint64Slice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a sortableUint64Slice) Less(i, j int) bool {
	return a[i] < a[j]
}

type stateTransferState struct {
	pbft   *pbftCore
	ledger consensus.Ledger

	OutOfDate bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	hChkpts map[uint64]uint64 // highest checkpoint sequence number observed for each replica

	blockVerifyChunkSize uint64        // The max block length to attempt to sync at once, this prevents state transfer from being delayed while the blockchain is validated
	validBlockRanges     []*blockRange // Used by the block thread to track which pieces of the blockchain have already been hashed
	recoverDamage        bool          // Whether state transfer should ever modify or delete existing blocks if they are determined to be corrupted

	initiateStateSync chan *syncMark        // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	chkptWeakCertReq  chan *ckptWeakCertReq // Used to wait for a relevant checkpoint weak certificate, write only from the state thread
	blockSyncReq      chan *blockSyncReq    // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread
	completeStateSync chan uint64           // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread

	blockRequestTimeout         time.Duration // How long to wait for a replica to respond to a block request
	stateDeltaRequestTimeout    time.Duration // How long to wait for a replica to respond to a state delta request
	stateSnapshotRequestTimeout time.Duration // How long to wait for a replica to respond to a state snapshot request

}

func threadlessNewStateTransferState(pbft *pbftCore, config *viper.Viper, ledger consensus.Ledger) *stateTransferState {
	sts := &stateTransferState{}

	sts.pbft = pbft
	sts.ledger = ledger

	sts.OutOfDate = false

	sts.recoverDamage = config.GetBool("stateTransfer.recoverdamage")

	sts.hChkpts = make(map[uint64]uint64)

	sts.validBlockRanges = make([]*blockRange, 0)
	sts.blockVerifyChunkSize = uint64(config.GetInt("statetransfer.blocksperrequest"))
	if sts.blockVerifyChunkSize == 0 {
		panic(fmt.Errorf("Must set statetransfer.blocksperrequest to be nonzero"))
	}

	sts.initiateStateSync = make(chan *syncMark)
	sts.chkptWeakCertReq = make(chan *ckptWeakCertReq, 1) // May have the reading thread re-queue a request, so buffer of 1 is required to prevent deadlock
	sts.blockSyncReq = make(chan *blockSyncReq)
	sts.completeStateSync = make(chan uint64)

	var err error

	sts.blockRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.singleblock"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singleblock timeout: %s", err))
	}
	sts.stateDeltaRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.singlestatedelta"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singlestatedelta timeout: %s", err))
	}
	sts.stateSnapshotRequestTimeout, err = time.ParseDuration(config.GetString("statetransfer.timeout.fullstate"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.fullstate timeout: %s", err))
	}

	return sts
}

func newStateTransferState(pbft *pbftCore, config *viper.Viper, ledger consensus.Ledger) *stateTransferState {
	sts := threadlessNewStateTransferState(pbft, config, ledger)

	go sts.stateThread()
	go sts.blockThread()

	return sts
}

// Executes a func trying each replica included in replicaIds until successful
// Attempts to execute over all replicas if replicaIds is nil
func (sts *stateTransferState) tryOverReplicas(replicaIds []uint64, do func(replicaId uint64) error) (err error) {
	startIndex := uint64(rand.Int() % sts.pbft.replicaCount)
	if nil == replicaIds {
		for i := 0; i < sts.pbft.replicaCount; i++ {
			index := (uint64(i) + startIndex) % uint64(sts.pbft.replicaCount)
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
			// logger.Debug("Replica %d Tryloop picking index %d and replica %d to do work", sts.pbft.id, index, replicaIds[index])
			err = do(replicaIds[index])
			if err == nil {
				break
			} else {
				logger.Warning("Replica %d in tryOverReplicas loop : %s", sts.pbft.id, err)
			}
		}
	}

	return err

}

// Attempts to complete a blockSyncReq using the supplied replicas
// Will return the last block number attempted to sync, and the last block successfully synced (or nil) and error on failure
// This means on failure, the returned block corresponds to 1 higher than the returned block number
func (sts *stateTransferState) syncBlocks(highBlock, lowBlock uint64, highHash []byte, replicaIds []uint64) (uint64, *protos.Block, error) {
	logger.Debug("Replica %d syncing blocks from %d to %d", sts.pbft.id, highBlock, lowBlock)
	validBlockHash := highHash
	blockCursor := highBlock
	var block *protos.Block

	err := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {
		blockChan, err := sts.ledger.GetRemoteBlocks(replicaId, blockCursor, lowBlock)
		if nil != err {
			logger.Warning("Replica %d failed to get blocks from %d to %d from replica %d: %s",
				sts.pbft.id, blockCursor, lowBlock, replicaId, err)
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
						return fmt.Errorf("Replica %d got a block %d which could not hash from replica %d: %s",
							sts.pbft.id, blockCursor, replicaId, err)
					}

					if !bytes.Equal(testHash, validBlockHash) {
						return fmt.Errorf("Replica %d got block %d from replica %d with hash %x, was expecting hash %x",
							sts.pbft.id, blockCursor, replicaId, testHash, validBlockHash)
					}

					logger.Debug("Replica %d putting block %d to with PreviousBlockHash %x and StateHash %x", sts.pbft.id, blockCursor, block.PreviousBlockHash, block.StateHash)

					if !sts.recoverDamage {
						// If we are not supposed to be destructive in our recovery, check to make sure this block doesn't already exist
						if oldBlock, err := sts.ledger.GetBlock(blockCursor); err == nil && oldBlock != nil {
							panic("The blockchain is corrupt and the configuration has specified that bad blocks should not be deleted/overridden")
						}
					}

					sts.ledger.PutBlock(blockCursor, block)

					validBlockHash = block.PreviousBlockHash

					if blockCursor == lowBlock {
						logger.Debug("Replica %d successfully synced from block %d to block %d", sts.pbft.id, highBlock, lowBlock)
						return nil
					}
					blockCursor--

				}
			case <-time.After(sts.blockRequestTimeout):
				return fmt.Errorf("Replica %d had block sync request to %d time out", sts.pbft.id, replicaId)
			}
		}
	})

	if nil != block {
		logger.Debug("Replica %d returned from sync with block %d and hash %x", sts.pbft.id, blockCursor, block.StateHash)
	} else {
		logger.Debug("Replica %d returned from sync with no new blocks", sts.pbft.id)
	}

	return blockCursor, block, err

}

func (sts *stateTransferState) syncBlockchainToCheckpoint(blockSyncReq *blockSyncReq) {

	logger.Debug("Replica %d is processing a blockSyncReq to block %d", sts.pbft.id, blockSyncReq.blockNumber)

	blockchainSize := sts.ledger.GetBlockchainSize()

	if blockSyncReq.blockNumber < blockchainSize {
		if !sts.recoverDamage {
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

		if err == nil {
			if nil != blockSyncReq.replyChan {
				blockSyncReq.replyChan <- &blockSyncReply{
					blockNumber: blockNumber,
					stateHash:   block.StateHash,
					err:         nil,
				}
				goodRange.lowBlock = blockNumber
			}
		} else {
			if nil != blockSyncReq.replyChan {
				blockSyncReq.replyChan <- &blockSyncReply{
					blockNumber: blockNumber,
					stateHash:   nil,
					err:         err,
				}
				goodRange.lowBlock = blockNumber
			}
		}

		goodRange.lowNextHash = block.PreviousBlockHash
		sts.validBlockRanges = append(sts.validBlockRanges, goodRange)
	}
}

func (sts *stateTransferState) verifyAndRecoverBlockchain() bool {

	if 0 == len(sts.validBlockRanges) {
		size := sts.ledger.GetBlockchainSize()
		if 0 == size {
			logger.Warning("Replica %d has no blocks in its blockchain, including the genesis block", sts.pbft.id)
			return false
		}

		block, err := sts.ledger.GetBlock(size - 1)
		if nil != err {
			logger.Warning("Replica %d could not retrieve its head block %d: %s", sts.pbft.id, size, err)
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
					logger.Warning("Replica %d could not retrieve block %d which it believed to be valid: %s", sts.pbft.id, lowBlock-1, err)
				} else {
					if blockHash, err := sts.ledger.HashBlock(block); nil != err {
						if bytes.Equal(blockHash, lowNextHash) {
							// The chains connect, no need to validate all the way down
							sts.validBlockRanges[0].lowBlock = sts.validBlockRanges[1].lowBlock
							sts.validBlockRanges[0].lowNextHash = sts.validBlockRanges[1].lowNextHash
						} else {
							logger.Warning("Replica %d detected that a block range starting at %d previously believed to be valid did not hash correctly", sts.pbft.id, lowBlock-1)
						}
					} else {
						logger.Warning("Replica %d could not hash block %d which it believed to be valid: %s", sts.pbft.id, lowBlock-1, err)
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
			logger.Warning("Replica %d just recovered block %d but cannot compute its hash: %s", sts.pbft.id, blockNumber, err2)
		} else {
			logger.Warning("Replica %d just recovered block %d but cannot compute its hash: %s", sts.pbft.id, blockNumber+1, err2)
		}
	}

	return false
}

func (sts *stateTransferState) blockThread() {

	for {

		select {
		case blockSyncReq := <-sts.blockSyncReq:
			sts.syncBlockchainToCheckpoint(blockSyncReq)
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
			if !sts.verifyAndRecoverBlockchain() {
				// There is more verification to be done, so loop
				continue
			}
		}

		logger.Debug("Replica %d has validated its blockchain to the genesis block", sts.pbft.id)

		// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
		blockSyncReq := <-sts.blockSyncReq
		sts.syncBlockchainToCheckpoint(blockSyncReq)
	}
}

// A thread to process state transfer
func (sts *stateTransferState) stateThread() {
	for {
		// Wait for state sync to become necessary
		mark := <-sts.initiateStateSync

		logger.Debug("Replica %d is initiating state transfer", sts.pbft.id)

		for {

			// If we are here, our state is currently bad, so get a new one
			currentStateBlockNumber, err := sts.syncStateSnapshot(mark.blockNumber, mark.replicaIds)

			if nil != err {
				// This is very bad, we had f+1 replicas unable to reply with a state above a block number they advertised in a checkpoint, should never happen
				logger.Error("Replica %d could not retrieve state as recent as advertised checkpoints above %d, indicates byzantine of f+1", sts.pbft.id, mark.blockNumber)
				mark = &syncMark{ // Let's try to just sync state from anyone, for any sequence number
					blockNumber: 0,
					replicaIds:  nil,
				}
				continue
			}

			logger.Debug("Replica %d completed state transfer to block %d", sts.pbft.id, currentStateBlockNumber)

			certChan := make(chan *chkptWeakCert)
			sts.chkptWeakCertReq <- &ckptWeakCertReq{
				blockNumber: currentStateBlockNumber,
				certChan:    certChan,
			}

			logger.Debug("Replica %d state transfer thread waiting for a new checkpoint weak certificate", sts.pbft.id)
			weakCert := <-certChan

			mark = &syncMark{ // We now know of a more recent state which f+1 nodes have achieved, if we fail, try from this set
				blockNumber: weakCert.blockNumber,
				replicaIds:  weakCert.replicaIds,
			}

			blockReplyChannel := make(chan *blockSyncReply)

			sts.blockSyncReq <- &blockSyncReq{
				syncMark:       *mark,
				reportOnBlock:  currentStateBlockNumber,
				replyChan:      blockReplyChannel,
				firstBlockHash: weakCert.blockHash,
			}

			logger.Debug("Replica %d state transfer thread waiting for block sync to complete", sts.pbft.id)
			blockSyncReply := <-blockReplyChannel

			if blockSyncReply.err != nil {
				// This is very bad, we had f+1 replicas unable to reply with blocks for a weak certificate they presented, this should never happen
				// Maybe we should panic here, but we can always try again
				logger.Error("Replica %d could not retrieve blocks as recent as advertised in checkpoints above %d, indicates byzantine of f+1", sts.pbft.id, mark.blockNumber)
				continue
			}

			stateHash, err := sts.ledger.GetCurrentStateHash()
			if nil != err {
				logger.Warning("Replica %d could not compute its current state hash: %s", sts.pbft.id, err)
				continue

			}

			if !bytes.Equal(stateHash, blockSyncReply.stateHash) {
				logger.Warning("Replica %d recovered to an incorrect state at block number %d, (%x %x) retrying", sts.pbft.id, currentStateBlockNumber, stateHash, blockSyncReply.stateHash)
				continue
			}

			if currentStateBlockNumber < weakCert.blockNumber {
				currentStateBlockNumber, err := sts.playStateUpToCheckpoint(currentStateBlockNumber+uint64(1), weakCert.blockNumber, weakCert.replicaIds)
				if nil != err {
					// This is unlikely, in the future, we may wish to play transactions forward rather than retry
					logger.Warning("Replica %d was unable to play the state from block number %d forward to block %d, retrying: %s", sts.pbft.id, currentStateBlockNumber, weakCert.blockNumber, err)
					continue
				}
			}

			sts.completeStateSync <- weakCert.blockNumber
			break
		}

	}
}

func (sts *stateTransferState) playStateUpToCheckpoint(fromBlockNumber, toBlockNumber uint64, replicaIds []uint64) (uint64, error) {
	currentBlock := fromBlockNumber
	err := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {

		deltaMessages, err := sts.ledger.GetRemoteStateDeltas(replicaId, currentBlock, toBlockNumber)
		if err != nil {
			return fmt.Errorf("Replica %d received an error while trying to get the state deltas for blocks %d through %d from replica %d", sts.pbft.id, fromBlockNumber, toBlockNumber, replicaId)
		}

		for {
			select {
			case deltaMessage, ok := <-deltaMessages:
				if !ok {
					if currentBlock == toBlockNumber+1 {
						return nil
					}
					return fmt.Errorf("Replica %d was only able to recover to block number %d when desired to recover to %d", sts.pbft.id, currentBlock, toBlockNumber)
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
					logger.Warning("Replica %d could not retrieve block %d, though it should be present", sts.pbft.id, deltaMessage.Range.End)
				} else {

					stateHash, err := sts.ledger.GetCurrentStateHash()
					if nil != err {
						logger.Warning("Replica %d could not compute its state hash for some reason: %s", sts.pbft.id, err)
					}
					logger.Debug("Replica %d has played state forward from replica %d to block %d with StateHash (%x), the corresponding block has StateHash (%x)",
						sts.pbft.id, replicaId, deltaMessage.Range.End, stateHash, testBlock.StateHash)

					if bytes.Equal(testBlock.StateHash, stateHash) {
						success = true
					}
				}

				if !success {
					for _, delta := range deltaMessage.Deltas {
						sts.ledger.ApplyStateDelta(delta, true)
					}

					// TODO, this is wrong, our state might not rewind correctly, will need a commit/rollback API for this
					return fmt.Errorf("Replica %d played state forward according to replica %d, but the state hash did not match", sts.pbft.id, replicaId)
				} else {
					currentBlock++
				}
			case <-time.After(sts.stateDeltaRequestTimeout):
				logger.Warning("Replica %d timed out during state delta recovery from from replica %d", sts.pbft.id, replicaId)
				return fmt.Errorf("Replica %d timed out during state delta recovery from from replica %d", sts.pbft.id, replicaId)

			}
		}

		return nil

	})
	return currentBlock, err
}

// This function will retrieve the current state from a replica.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (sts *stateTransferState) syncStateSnapshot(minBlockNumber uint64, replicaIds []uint64) (uint64, error) {

	currentStateBlock := uint64(0)

	ok := sts.tryOverReplicas(replicaIds, func(replicaId uint64) error {
		logger.Debug("Replica %d is initiating state recovery from from replica %d", sts.pbft.id, replicaId)

		sts.ledger.EmptyState()

		stateChan, err := sts.ledger.GetRemoteStateSnapshot(replicaId)

		if err != nil {
			sts.ledger.EmptyState()
			return err
		}

		timer := time.NewTimer(sts.stateSnapshotRequestTimeout)

		for {
			select {
			case piece, ok := <-stateChan:
				if !ok {
					return nil
				}
				sts.ledger.ApplyStateDelta(piece.Delta, false)
				currentStateBlock = piece.BlockNumber
			case <-timer.C:
				logger.Warning("Replica %d timed out during state recovery from replica %d", sts.pbft.id, replicaId)
				return fmt.Errorf("Replica %d timed out during state recovery from replica %d", sts.pbft.id, replicaId)
			}
		}

	})

	return currentStateBlock, ok
}

func (sts *stateTransferState) WitnessCheckpoint(chkpt *Checkpoint) {
	if sts.OutOfDate {
		// State transfer is already going on, no need to track this
		return
	}

	H := sts.pbft.h + sts.pbft.L

	// Track the last observed checkpoint sequence number if it exceeds our high watermark, keyed by replica to prevent unbounded growth
	if chkpt.SequenceNumber < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(sts.hChkpts, chkpt.ReplicaId)
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarilly high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		sts.hChkpts[chkpt.ReplicaId] = chkpt.SequenceNumber

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(sts.hChkpts) >= sts.pbft.f+1 {
			chkptSeqNumArray := make([]uint64, len(sts.hChkpts))
			index := 0
			for replicaId, hChkpt := range sts.hChkpts {
				chkptSeqNumArray[index] = hChkpt
				index++
				if hChkpt < H {
					delete(sts.hChkpts, replicaId)
				}
			}
			sort.Sort(sortableUint64Slice(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(sts.hChkpts)-(sts.pbft.f+1)]; m > H {
				logger.Warning("Replica %d is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", sts.pbft.id, chkpt.SequenceNumber, H)
				sts.OutOfDate = true
				sts.pbft.moveWatermarks(m + sts.pbft.K)

				furthestReplicaIds := make([]uint64, sts.pbft.f+1)
				i := 0
				for replicaId, hChkpt := range sts.hChkpts {
					if hChkpt >= m {
						furthestReplicaIds[i] = replicaId
						i++
					}
				}

				sts.initiateStateSync <- &syncMark{
					blockNumber: m,
					replicaIds:  furthestReplicaIds,
				}
			}

			return
		}
	}

}

func (sts *stateTransferState) WitnessCheckpointWeakCert(chkpt *Checkpoint) {
	select {
	default:
		return
	case certReq := <-sts.chkptWeakCertReq:
		logger.Debug("Replica %d detects a weak cert for checkpoint %d, and has a certificate request for checkpoint over %d", sts.pbft.id, chkpt.BlockNumber, certReq.blockNumber)
		if certReq.blockNumber > chkpt.BlockNumber {
			// There's a pending request, but this call didn't satisfy it, so put it back
			sts.chkptWeakCertReq <- certReq
			return
		}

		checkpointMembers := make([]uint64, sts.pbft.replicaCount)
		i := 0
		for testChkpt := range sts.pbft.checkpointStore {
			if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.BlockHash == chkpt.BlockHash {
				checkpointMembers[i] = testChkpt.ReplicaId
				i++
			}
		}

		logger.Debug("Replica %d replying to weak cert request with checkpoint %d (%s)", sts.pbft.id, chkpt.BlockNumber, chkpt.BlockHash)

		blockHashBytes, err := base64.StdEncoding.DecodeString(chkpt.BlockHash)

		if nil != err {
			logger.Error("Replica %d received a weak checkpoint cert for block %d which could not be decoded (%s)", sts.pbft.id, chkpt.BlockNumber, chkpt.BlockHash)
			return
		}

		certReq.certChan <- &chkptWeakCert{
			syncMark: syncMark{
				blockNumber: chkpt.BlockNumber,
				replicaIds:  checkpointMembers[0 : i-1],
			},
			blockHash: blockHashBytes,
		}
	}

}

func (sts *stateTransferState) RecoveryJustCompleted() (uint64, bool) {
	select {
	case blockNumber := <-sts.completeStateSync:
		return blockNumber, true
	default:
		return uint64(0), false
	}
}
