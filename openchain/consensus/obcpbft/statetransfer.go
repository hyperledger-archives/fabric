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
	"fmt"
	"math/rand"
	"sort"

	"github.com/openblockchain/obc-peer/protos"
)

type stLedger interface {
	getBlockchainSize() uint64
	getBlock(id uint64) (*protos.Block, error)
	getRemoteBlocks(replicaId uint64, start, finish uint64) (<-chan *protos.SyncBlocks, error)
	getRemoteStateSnapshot(replicaId uint64) (<-chan *protos.SyncStateSnapshot, error)
	getRemoteStateDeltas(replicaId uint64, start, finish uint64) (<-chan *protos.SyncStateDeltas, error)
	hashBlock(block *protos.Block) ([]byte, error)
	putBlock(blockNumber uint64, block *protos.Block)
	applyStateDelta(delta []byte, unapply bool)
	emptyState()
	getCurrentStateHash() []byte
	verifyBlockChain(start, finish uint64) (uint64, error)
}

type stBlock interface {
	getHash() ([]byte, error)
}

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

type stateTransferState struct {
	pbft   *pbftCore
	ledger stLedger

	OutOfDate bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	hChkpts map[uint64]uint64 // highest checkpoint sequence number observed for each replica

	initiateStateSync chan *syncMark        // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	chkptWeakCertReq  chan *ckptWeakCertReq // Used to wait for a relevant checkpoint weak certificate, write only from the state thread
	blockSyncReq      chan *blockSyncReq    // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread
	completeStateSync chan uint64           // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread
}

func newStateTransferState(pbft *pbftCore, ledger stLedger) *stateTransferState {
	sts := &stateTransferState{}

	sts.pbft = pbft
	sts.ledger = ledger

	sts.OutOfDate = false

	sts.hChkpts = make(map[uint64]uint64)

	sts.initiateStateSync = make(chan *syncMark)
	sts.chkptWeakCertReq = make(chan *ckptWeakCertReq, 1) // May have the reading thread queue a request, so buffer of 1 is required to prevent deadlock
	sts.blockSyncReq = make(chan *blockSyncReq)
	sts.completeStateSync = make(chan uint64)

	go sts.stateThread()
	go sts.blockThread()

	return sts
}

// Executes a func trying each replica included in replicaIds until successful
// Attempts to execute over all replicas if replicaIds is nil
func (sts *stateTransferState) tryOverReplicas(replicaIds []uint64, do func(replicaId uint64) bool) bool {
	startIndex := uint64(rand.Int() % sts.pbft.replicaCount)
	if nil == replicaIds {
		for i := 0; i < sts.pbft.replicaCount; i++ {
			// TODO, this is broken, need some way to get a list of all replicaIds, cannot assume they are sequential from 0
			if do(uint64(0)) {
				return true
			}
		}
	} else {
		numReplicas := uint64(len(replicaIds))

		for i := uint64(0); i < uint64(numReplicas); i++ {
			if do((replicaIds[i] + startIndex) % numReplicas) {
				return true
			}
		}
	}

	return false
}

// Attempts to complete a blockSyncReq using the supplied replicas
// Will return the last block attempted to sync and success/failure
func (sts *stateTransferState) syncBlocks(blockSyncReq *blockSyncReq, endBlock uint64) (uint64, bool) {
	validBlockHash := blockSyncReq.firstBlockHash
	blockCursor := blockSyncReq.blockNumber

	if !sts.tryOverReplicas(blockSyncReq.replicaIds, func(replicaId uint64) bool {
		blockChan, err := sts.ledger.getRemoteBlocks(replicaId, blockCursor, endBlock)
		if nil != err {
			logger.Warning("Replica %d failed to get blocks from %d to %d from replica %d: %s",
				sts.pbft.id, blockCursor, endBlock, replicaId, err)
			return false
		}
		for {
			syncBlockMessage, ok := <-blockChan

			if !ok {
				return false
			}

			if syncBlockMessage.Range.Start < syncBlockMessage.Range.End {
				continue
			}

			for i, block := range syncBlockMessage.Blocks {
				// It is possible to get duplication or out of range blocks due to an implementation detail, we must check for them
				if syncBlockMessage.Range.Start-uint64(i) != blockCursor {
					continue
				}

				testHash, err := sts.ledger.hashBlock(block)
				if nil != err {
					logger.Warning("Replica %d got a block %d which could not hash from replica %d: %s",
						sts.pbft.id, blockCursor, replicaId, err)
					return false
				}

				if !bytes.Equal(testHash, validBlockHash) {
					logger.Warning("Replica %d got a block %d which did not hash correctly (%x != %x) from replica %d",
						sts.pbft.id, blockCursor, testHash, validBlockHash, replicaId)
					return false
				}

				validBlockHash = block.PreviousBlockHash

				sts.ledger.putBlock(blockCursor, block)

				if nil != blockSyncReq.replyChan && blockCursor == blockSyncReq.reportOnBlock {
					blockSyncReq.replyChan <- &blockSyncReply{
						blockNumber: blockCursor,
						stateHash:   block.StateHash,
						err:         nil,
					}
				}

				if blockCursor == endBlock {
					return true
				}
				blockCursor--
				logger.Debug("Replica %d successfully synced from block %d to block %d", sts.pbft.id, blockSyncReq.blockNumber, endBlock)

			}

			return false

		}

	}) {
		if nil != blockSyncReq.replyChan && blockCursor >= blockSyncReq.reportOnBlock {
			blockSyncReq.replyChan <- &blockSyncReply{
				blockNumber: blockCursor,
				stateHash:   nil,
				err:         fmt.Errorf("Failed to retrieve blocks to reportOnBlock %d", blockSyncReq.reportOnBlock),
			}
		}
		return blockCursor, false
	}

	return blockCursor, true
}

func (sts *stateTransferState) blockThread() {
	blockVerifyChunkSize := uint64(20) // TODO make this configurable
	lowestValidBlock := sts.ledger.getBlockchainSize()

	syncBlockchainToCheckpoint := func(blockSyncReq *blockSyncReq) {

		logger.Debug("Replica %d is processing a blockSyncReq to block %d", sts.pbft.id, blockSyncReq.blockNumber)

		blockchainSize := sts.ledger.getBlockchainSize()

		if blockSyncReq.blockNumber < blockchainSize {
			// TODO this is a troubling scenario, we think we're out of date, but we just got asked to sync to a block that's older than our current chain
			// this could be malicious behavior from byzantine nodes attempting to slow the network down, but we should still catch up
			// XXX For now, unimplemented because we have no way to delete blocks
			panic("Our blockchain is already higher than a sync target, this is unlikely, but unimplemented")
		} else {
			if blockNumber, ok := sts.syncBlocks(blockSyncReq, blockchainSize); ok {
				if blockNumber > 0 {
					// The sync succeeded, chain added up to old head+1
					lastBlock, err := sts.ledger.getBlock(blockNumber)

					if nil != err {
						logger.Warning("Replica %d just recovered block %d but cannot find it: %s", sts.pbft.id, blockNumber, err)
						lowestValidBlock = sts.ledger.getBlockchainSize()
						return
					}

					// Make sure the old blockchain head ties into the new blockchain we just received, if not, mark lowestValidBlock
					// to the just recovered blocks so that the rest of the chain will be recovered
					oldHeadBlock, err := sts.ledger.getBlock(blockNumber - 1)

					if nil != err {
						lowestValidBlock = blockNumber
						return
					}

					oldHeadBlockHash, err := sts.ledger.hashBlock(oldHeadBlock)

					if nil != err || !bytes.Equal(lastBlock.PreviousBlockHash, oldHeadBlockHash) {
						lowestValidBlock = blockNumber
						return
					}
				} else {
					lowestValidBlock = 0
				}
			} else {
				// The sync failed after recovering up to blockNumber + 1
				lowestValidBlock = blockNumber + 1
			}
		}
	}

	for {

		select {
		case blockSyncReq := <-sts.blockSyncReq:
			syncBlockchainToCheckpoint(blockSyncReq)
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
		}

		if lowestValidBlock > 0 {
			verifyToBlock := uint64(0)
			if lowestValidBlock > blockVerifyChunkSize {
				verifyToBlock = lowestValidBlock - blockVerifyChunkSize
			}
			badSource, err := sts.ledger.verifyBlockChain(lowestValidBlock, verifyToBlock)

			if nil == err && badSource == 0 {
				lowestValidBlock = verifyToBlock
				continue

			}

			logger.Warning("Replica %d has found a block whose PreviousBlockHash does not match the previous block's hash at %d", sts.pbft.id, badSource)

			if badSource == 0 {
				panic("The blockchain cannot claim the genesis block's previous block hash does not match.")
			}

			if nil != err {
				logger.Warning("Replica %d encountered an error at block %d while verifying its blockchain, attempting to sync", sts.pbft.id, badSource)
			}

			goodBlock, err := sts.ledger.getBlock(badSource)

			if nil != err {
				panic(fmt.Sprintf("The blockchain just informed us that the previous block hash from block %d did not match, but claims the block does not exist: %s", badSource, err))
			}

			if lastBlock, ok := sts.syncBlocks(&blockSyncReq{
				syncMark: syncMark{
					replicaIds:  nil, // Use all replicas
					blockNumber: badSource - 1,
				},
				reportOnBlock:  0,   // Unused because no replyChan
				replyChan:      nil, // We will wait syncMarkhronously
				firstBlockHash: goodBlock.PreviousBlockHash,
			}, verifyToBlock); ok {
				lowestValidBlock = lastBlock
			} else {
				lowestValidBlock = lastBlock + 1
			}

			continue
		}

		// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
		select {
		case blockSyncReq := <-sts.blockSyncReq:
			syncBlockchainToCheckpoint(blockSyncReq)
		}
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
			currentStateBlockNumber, ok := sts.syncStateSnapshot(mark.blockNumber, mark.replicaIds)

			if !ok {
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

			blockSyncReply := <-blockReplyChannel

			if blockSyncReply.err != nil {
				// This is very bad, we had f+1 replicas unable to reply with blocks for a weak certificate they presented, this should never happen
				// Maybe we should panic here, but we can always try again
				logger.Error("Replica %d could not retrieve blocks as recent as advertised in checkpoints above %d, indicates byzantine of f+1", sts.pbft.id, mark.blockNumber)
				continue
			}

			if !bytes.Equal(sts.ledger.getCurrentStateHash(), blockSyncReply.stateHash) {
				logger.Warning("Replica %d recovered to an incorrect state at block number %d, retrying", sts.pbft.id, currentStateBlockNumber)
				continue
			}

			if currentStateBlockNumber < weakCert.blockNumber {
				currentStateBlockNumber, ok = sts.playStateUpToCheckpoint(currentStateBlockNumber+uint64(1), weakCert.blockNumber, weakCert.replicaIds)
				if !ok {
					// This is unlikely, in the future, we may wish to play transactions forward rather than retry
					logger.Warning("Replica %d was unable to play the state from block number %d forward to block %d, retrying", sts.pbft.id, currentStateBlockNumber, weakCert.blockNumber)
					continue
				}
			}

			sts.completeStateSync <- weakCert.blockNumber
			break
		}

	}
}

func (sts *stateTransferState) playStateUpToCheckpoint(fromBlockNumber, toBlockNumber uint64, replicaIds []uint64) (uint64, bool) {
	currentBlock := fromBlockNumber
	ok := sts.tryOverReplicas(replicaIds, func(replicaId uint64) bool {
		deltaMessages, err := sts.ledger.getRemoteStateDeltas(replicaId, currentBlock, toBlockNumber)
		if err != nil {
			logger.Warning("Replica %d received an error while trying to get the state deltas for blocks %d through %d from replica %d", sts.pbft.id, fromBlockNumber, toBlockNumber, replicaId)
			return false
		}

		for deltaMessage := range deltaMessages {
			if deltaMessage.Range.Start != currentBlock || deltaMessage.Range.End < deltaMessage.Range.Start || deltaMessage.Range.End > toBlockNumber {
				continue // this is an unfortunately normal case, as we can get duplicates, just ignore it
			}

			for _, delta := range deltaMessage.Deltas {
				sts.ledger.applyStateDelta(delta, false)
			}

			success := false

			testBlock, err := sts.ledger.getBlock(deltaMessage.Range.End)

			if nil != err {
				logger.Warning("Replica %d could not retrieve block %d, though it should be present", sts.pbft.id, deltaMessage.Range.End)
			} else {
				if bytes.Equal(testBlock.StateHash, sts.ledger.getCurrentStateHash()) {
					success = true
				} else {
					logger.Warning("Replica %d played state forward according to replica %d, but the state hash did not match", sts.pbft.id, replicaId)
				}
			}

			if !success {
				for _, delta := range deltaMessage.Deltas {
					sts.ledger.applyStateDelta(delta, true)
				}

				return false
			}
		}

		return currentBlock == toBlockNumber
	})
	return currentBlock, ok
}

// This function will retrieve the current state from a replica.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (sts *stateTransferState) syncStateSnapshot(minBlockNumber uint64, replicaIds []uint64) (uint64, bool) {

	currentStateBlock := uint64(0)

	ok := sts.tryOverReplicas(replicaIds, func(replicaId uint64) bool {
		logger.Debug("Replica %d is initiating state recovery from from replica %d", sts.pbft.id, replicaId)

		sts.ledger.emptyState()

		stateChan, err := sts.ledger.getRemoteStateSnapshot(replicaId)

		if err != nil {
			sts.ledger.emptyState()
			return false
		}

		// TODO add timeout mechanism
		for piece := range stateChan {
			sts.ledger.applyStateDelta(piece.Delta, false)
			currentStateBlock = piece.BlockNumber
		}

		return true

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
			sort.Sort(sortableUint64Array(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(sts.hChkpts)-(sts.pbft.f+1)]; m > H {
				logger.Warning("Replica is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", chkpt.SequenceNumber, H)
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
			if testChkpt.SequenceNumber == chkpt.SequenceNumber && bytes.Equal(testChkpt.BlockHash, chkpt.BlockHash) {
				checkpointMembers[i] = testChkpt.ReplicaId
				i++
			}
		}

		logger.Debug("Replica %d replying to weak cert request with checkpoint %d (%x)", sts.pbft.id, chkpt.BlockNumber, chkpt.BlockHash)
		certReq.certChan <- &chkptWeakCert{
			syncMark: syncMark{
				blockNumber: chkpt.BlockNumber,
				replicaIds:  checkpointMembers[0 : i-1],
			},
			blockHash: chkpt.BlockHash,
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
