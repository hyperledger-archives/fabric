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
	getRemoteBlocks(replicaId int, start, finish uint64) (<-chan *protos.SyncBlocks, error)
	getRemoteStateSnapshot(replicaId int) (<-chan *protos.SyncState, error)
	getRemoteStateDeltas(replicaId int, start, finish uint64) (<-chan *protos.SyncState, error)
	putBlock(block *protos.Block)
	applyStateDelta(delta []byte)
	emptyState()
	getCurrentStateHash() []byte
	verifyBlockChain(start, finish uint64) (uint64, error)
}

type syncMark struct {
	blockNumber uint64
	replicaIds  []int
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

type stState struct {
	pbft   *pbftCore
	ledger stLedger

	OutOfDate bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	hChkpts map[int]uint64 // highest checkpoint sequence number observed for each replica

	initiateStateSync chan *syncMark        // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	chkptWeakCertReq  chan *ckptWeakCertReq // Used to wait for a relevant checkpoint weak certificate, write only from the state thread
	blockSyncReq      chan *blockSyncReq    // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread
}

func newStState(pbft *pbftCore, ledger stLedger) *stState {
	stState := &stState{}

	stState.pbft = pbft
	stState.ledger = ledger

	stState.OutOfDate = false

	stState.hChkpts = make(map[int]uint64)

	stState.initiateStateSync = make(chan *syncMark)
	stState.chkptWeakCertReq = make(chan *ckptWeakCertReq, 1) // May have the reading thread queue a request, so buffer of 1 is required to prevent deadlock
	stState.blockSyncReq = make(chan *blockSyncReq)

	go stState.stateThread()
	go stState.blockThread()

	return stState
}

// Executes a func trying each replica included in replicaIds until successful
// Attempts to execute over all replicas if replicaIds is nil
func (stState *stState) tryOverReplicas(replicaIds []int, do func(replicaId int) bool) bool {
	startIndex := rand.Int() % stState.pbft.replicaCount
	if nil == replicaIds {
		for i := 0; i < stState.pbft.replicaCount; i++ {
			if do(i) {
				return true
			}
		}
	} else {
		numReplicas := len(replicaIds)

		for i := 0; i < numReplicas; i++ {
			if do((replicaIds[i] + startIndex) % numReplicas) {
				return true
			}
		}
	}

	return false
}

// Attempts to complete a blockSyncReq using the supplied replicas
// Will return the last block attempted to sync and success/failure
func (stState *stState) syncBlocks(blockSyncReq *blockSyncReq, endBlock uint64) (uint64, bool) {
	validBlockHash := blockSyncReq.firstBlockHash
	blockCursor := blockSyncReq.blockNumber

	if !stState.tryOverReplicas(blockSyncReq.replicaIds, func(replicaId int) bool {
		blockChan, err := stState.ledger.getRemoteBlocks(replicaId, blockCursor, endBlock)
		if nil != err {
			logger.Warning("Replica %d failed to get blocks from %d to %d from replica %d: %s",
				stState.pbft.id, blockCursor, endBlock, replicaId, err)
			return false
		}
		for {
			select {
			case syncBlockMessage, ok := <-blockChan:
				if !ok {
					break
				}
				if syncBlockMessage.Range.Start < syncBlockMessage.Range.End {
					continue
				}

				for i, block := range syncBlockMessage.Blocks {

					// It is possible to get duplication or out of range blocks due to an implementation detail, we must check for them
					if syncBlockMessage.Range.Start-uint64(i) != blockCursor {
						continue
					}

					testHash, err := block.GetHash()
					if nil != err {
						logger.Warning("Replica %d got a block %d which could not hash from replica %d: %s",
							stState.pbft.id, blockCursor, replicaId, err)
						return false
					}

					if !bytes.Equal(testHash, validBlockHash) {
						logger.Warning("Replica %d got a block %d which did not hash correctly from replica %d",
							stState.pbft.id, blockCursor, replicaId)
						return false
					}

					stState.ledger.putBlock(block)

					if nil != blockSyncReq.replyChan && blockCursor == blockSyncReq.reportOnBlock {
						blockSyncReq.replyChan <- &blockSyncReply{
							blockNumber: blockCursor,
							stateHash:   block.StateHash,
							err:         nil,
						}
					}

					blockCursor--
				}
			}
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

func (stState *stState) blockThread() {
	blockVerifyChunkSize := uint64(20) // TODO make this configurable
	lowPriorityBlockSyncReply := make(chan *blockSyncReply)
	lowPriorityBlockSyncRequest := make(chan *blockSyncReq, 1)
	lowestValidBlock := stState.ledger.getBlockchainSize()

	syncBlockchainToCheckpoint := func(blockSyncReq *blockSyncReq) {

		blockchainSize := stState.ledger.getBlockchainSize()

		if blockSyncReq.blockNumber < blockchainSize {
			// TODO this is a troubling scenario, we think we're out of date, but we just got asked to sync to a block that's older than our current chain
			// this could be malicious behavior from byzantine nodes attempting to slow the network down, but we should still catch up
			// XXX For now, unimplemented because we have no way to delete blocks
			panic("Our blockchain is already higher than a sync target, this is unlikely, but unimplemented")
		} else {
			if blockNumber, ok := stState.syncBlocks(blockSyncReq, blockchainSize); ok {
				if blockNumber > 0 {
					// The sync succeeded, chain added up to old head+1
					lastBlock, err := stState.ledger.getBlock(blockNumber)

					if nil != err {
						logger.Warning("Replica %d just recovered block %d but cannot find it: %s", stState.pbft.id, blockNumber, err)
						lowestValidBlock = stState.ledger.getBlockchainSize()
						return
					}

					// Make sure the old blockchain head ties into the new blockchain we just received, if not, mark lowestValidBlock
					// to the just recovered blocks so that the rest of the chain will be recovered
					oldHeadBlock, err := stState.ledger.getBlock(blockNumber - 1)

					if nil != err {
						lowestValidBlock := blockNumber
						return
					}

					oldHeadBlockHash, err := oldHeadBlock.GetHash()

					if nil != err || !bytes.Equal(lastBlock.PreviousBlockHash, oldHeadBlockHash) {
						lowestValidBlock := blockNumber
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
		case blockSyncReq := <-stState.blockSyncReq:
			syncBlockchainToCheckpoint(blockSyncReq)
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
		}

		if lowestValidBlock > 0 {
			verifyToBlock := uint64(0)
			if tmp := lowestValidBlock - blockVerifyChunkSize; tmp > 0 {
				verifyToBlock = tmp
			}
			badSource, err := stState.ledger.verifyBlockChain(lowestValidBlock, verifyToBlock)

			if nil == err && badSource == 0 {
				lowestValidBlock := verifyToBlock
				continue

			}

			if badSource == 0 {
				panic("The blockchain cannot claim the genesis block's previous block hash does not match.")
			}

			if nil != err {
				logger.Warning("Replica %d encountered an error at block %d while verifying its blockchain, attempting to sync", stState.pbft.id, badSource)
			}

			goodBlock, err := stState.ledger.getBlock(badSource)

			if nil != err {
				panic(fmt.Sprintf("The blockchain just informed us that the previous block hash from block %d did not match, but claims the block does not exist: %s", badSource, err))
			}

			if lastBlock, ok := stState.syncBlocks(&blockSyncReq{
				syncMark: syncMark{
					replicaIds:  nil, // Use all replicas
					blockNumber: badSource - 1,
				},
				reportOnBlock:  0,   // Unused because no replyChan
				replyChan:      nil, // We will wait syncMarkhronously
				firstBlockHash: goodBlock.PreviousBlockHash,
			}, verifyToBlock); ok {
				lowestValidBlock := lastBlock
			} else {
				lowestValidBlock := lastBlock + 1
			}

			continue
		}

		// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
		select {
		case blockSyncReq := <-stState.blockSyncReq:
			syncBlockchainToCheckpoint(blockSyncReq)
		}
	}
}

// A thread to process state transfer
func (stState *stState) stateThread() {
	for {
		// Wait for state sync to become necessary
		mark := <-stState.initiateStateSync

		logger.Debug("Replica %d is initiating state transfer", stState.pbft.id)

		for {

			// If we are here, our state is currently bad, so get a new one
			currentStateBlockNumber, ok := stState.syncStateSnapshot(mark.blockNumber, mark.replicaIds)

			if !ok {
				// This is very bad, we had f+1 replicas unable to reply with a state above a block number they advertised in a checkpoint, should never happen
				logger.Error("Replica %d could not retrieve state as recent as advertised checkpoints above %d, indicates byzantine of f+1", stState.pbft.id, mark.blockNumber)
				mark = &syncMark{ // Let's try to just sync state from anyone, for any sequence number
					blockNumber: 0,
					replicaIds:  nil,
				}
				continue
			}

			certChan := make(chan *chkptWeakCert)
			stState.chkptWeakCertReq <- &ckptWeakCertReq{
				blockNumber: currentStateBlockNumber,
				certChan:    certChan,
			}

			weakCert := <-certChan

			mark = &syncMark{ // We now know of a more recent state which f+1 nodes have achieved, if we fail, try from this set
				blockNumber: weakCert.blockNumber,
				replicaIds:  weakCert.replicaIds,
			}

			blockReplyChannel := make(chan *blockSyncReply)

			stState.blockSyncReq <- &blockSyncReq{
				syncMark:       *mark,
				reportOnBlock:  currentStateBlockNumber,
				replyChan:      blockReplyChannel,
				firstBlockHash: weakCert.blockHash,
			}

			blockSyncReply := <-blockReplyChannel

			if blockSyncReply.err != nil {
				// This is very bad, we had f+1 replicas unable to reply with blocks for a weak certificate they presented, this should never happen
				// Maybe we should panic here, but we can always try again
				logger.Error("Replica %d could not retrieve blocks as recent as advertised in checkpoints above %d, indicates byzantine of f+1", stState.pbft.id, mark.blockNumber)
				continue
			}

			if !bytes.Equal(stState.ledger.getCurrentStateHash(), blockSyncReply.stateHash) {
				logger.Warning("Replica %d recovered to an incorrect state at block number %d, retrying", stState.pbft.id, currentStateBlockNumber)
				continue
			}

			if !stState.playStateUpToCheckpoint(currentStateBlockNumber, weakCert.blockNumber, weakCert.replicaIds) {
				// This is unlikely, in the future, we may wish to play transactions forward rather than retry
				logger.Warning("Replica %d was unable to play the state from block number %d forward to block %d, retrying", stState.pbft.id, currentStateBlockNumber, weakCert.blockNumber)
				continue
			}
		}

	}
}

func (stState *stState) playStateUpToCheckpoint(fromBlockNumber, toBlockNumber uint64, replicaIds []int) bool {
	return false
}

// This function will retrieve the current state from a replica.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (stState *stState) syncStateSnapshot(minBlockNumber uint64, replicaIds []int) (uint64, bool) {

	currentStateBlock := uint64(0)

	ok := stState.tryOverReplicas(replicaIds, func(replicaId int) bool {
		logger.Debug("Replica %d is initiating state recovery from from replica %d", stState.pbft.id, replicaId)

		stState.ledger.emptyState()

		stateChan, err := stState.ledger.getRemoteStateSnapshot(replicaId)

		if err != nil {
			stState.ledger.emptyState()
			return false
		}

		// TODO add timeout mechanism
		for piece := range stateChan {
			stState.ledger.applyStateDelta(piece.delta)
			currentStateBlock = piece.BlockNumber
		}

		return true

	})

	return currentStateBlock, ok
}

func (stState *stState) WitnessCheckpoint(chkpt *Checkpoint) {
	if stState.OutOfDate {
		// State transfer is already going on, no need to track this
		return
	}

	H := stState.pbft.h + stState.pbft.L

	// Track the last observed checkpoint sequence number if it exceeds our high watermark, keyed by replica to prevent unbounded growth
	if chkpt.SequenceNumber < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(stState.hChkpts, chkpt.ReplicaId)
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarilly high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		stState.hChkpts[chkpt.ReplicaId] = chkpt.SequenceNumber

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(stState.hChkpts) >= stState.pbft.f+1 {
			chkptSeqNumArray := make([]uint64, len(stState.hChkpts))
			index := 0
			for replicaId, hChkpt := range stState.hChkpts {
				chkptSeqNumArray[index] = hChkpt
				index++
				if hChkpt < H {
					delete(stState.hChkpts, replicaId)
				}
			}
			sort.Sort(sortableUint64Array(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(stState.hChkpts)-(stState.pbft.f+1)]; m > H {
				logger.Warning("Replica is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", chkpt.SequenceNumber, H)
				stState.OutOfDate = true
				stState.pbft.moveWatermarks(m + stState.pbft.K)

				furthestReplicaIds := make([]int, stState.pbft.f+1)
				i := 0
				for replicaId, hChkpt := range stState.hChkpts {
					if hChkpt >= m {
						furthestReplicaIds[i] = replicaId
						i++
					}
				}

				stState.initiateStateSync <- &syncMark{
					blockNumber: m,
					replicaIds:  furthestReplicaIds,
				}
			}

			return
		}
	}

}

func (stState *stState) WitnessCheckpointWeakCert(matching int, chkpt *Checkpoint) {
	select {
	default:
		return
	case certReq := <-stState.chkptWeakCertReq:
		if certReq.blockNumber > chkpt.BlockNumber {
			// There's a pending request, but this call didn't satisfy it, so put it back
			stState.chkptWeakCertReq <- certReq
			return
		}

		checkpointMembers := make([]int, matching)
		i := 0
		for testChkpt := range stState.pbft.checkpointStore {
			if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.StateDigest == chkpt.StateDigest {
				checkpointMembers[i] = testChkpt.ReplicaId
				i++
			}
		}

		certReq.certChan <- &chkptWeakCert{
			syncMark: syncMark{
				blockNumber: chkpt.BlockNumber,
				replicaIds:  checkpointMembers,
			},
			blockHash: chkpt.StateHash,
		}
	}

}
