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
	"sort"
	"sync"
)

type stLedger interface {
	getBlockchainSize() uint64
	getBlock(id uint64) (*protos.Block, error)
	getRemoteBlocks(replicaId int, start, finish uint64) (<-chan *pb.SyncBlocks, error)
	getRemoteStateSnapshot(replicaId int) (<-chan *pb.SyncState, error)
	getRemoteStateDeltas(replicaId int, start, finish uint64) (<-chan *pb.SyncState, error)
	putBlock(block *protos.Block)
	applyStateDelta(delta []byte)
	emptyState()
	getCurrentStateHash() string
}

type stWeakChkptCert struct {
	blockNumber uint64
	blockHash   string
	replicaIds  []int
}

type stWeakChkptReq struct {
	blockNumber uint64
	certChan    <-chan *stWeakChkptCert
}

type stSync struct {
	blockNumber uint64
	replicaIds  []int
}

type stState struct {
	outOfDate        bool                   // To be used by the main consensus thread
	initiateSync     <-chan *stSync         // Used to ensure only one state transfer at a time occurs
	chkptWeakCertReq <-chan *stWeakChkptReq // Used to wait for a relevant checkpoint weak certificate
}

func (st stState) New(ledger stLedger) {
	st.outOfDate = false
	initiateSync = make(chan *stSync)
	chkptWeakCertReq = make(chan *stWeakChkptReq)

	go instance.stThread()
}

func (instance *pbftCore) stBlockThread() {
	select {
	case chkptBlock := <-blockSyncQueue:
		instance.ledger
	}
}

// A thread to process state transfer
func (instance *pbftCore) stThread() {
	for {
		// Wait for state sync to become necessary

		select {
		case stSync := <-instance.stState.initiateSync:
			logger.Debug("Replica %d is initiating state transfer", instance.id)
		}

		for {
			// If we are here, our state is currently bad, so get a new one
			instance.stState.currentStateBlockNumber, ok = instance.stSyncStateSnapshot(stSync.blockNumber, stSync.replicas)

			if !ok {
				// This is very bad, we had f+1 replicas unable to reply with a state above a block number they advertised in a checkpoint, should never happen
				logger.Error("Replica %d could not retrieve state as recent as advertised checkpoints above %d, indicates byzantine of f+1", instance.id, stSync.blockNumber)
				stSync = &StSync{ // Let's try to just sync state from anyone, for any sequence number
					blockNumber: 0,
					replicaIds:  nil,
				}
				continue
			}

			certChan := make(chan *stWeakChkptCert)
			instance.stState.chkptWeakCertReq <- &stChkptWeakCertReq{
				blockNumber: currentStateBlockNumber,
				certChan:    certChan,
			}

			select {
			case weakCert := <-certChan:
				stSync = &StSync{ // We now know of a more recent state which f+1 nodes have achieved, if we fail, try from this set
					blockNumber: weakCert.blockNumber,
					replicaIds:  weakCert.replicaIds,
				}

				if !instance.stState.stSyncBlockRange(weakCert.blockNumber, currentStateBlockNumber, weakCert.blockHash, weakCert.replicaIds) {
					// This is very bad, we had f+1 replicas unable to reply with blocks for a weak certificate they presented, this should never happen
					logger.Error("Replica %d could not retrieve blocks as recent as advertised in checkpoints above %d, indicates byzantine of f+1", instance.id, stSync.blockNumber)
					continue
				}

				currentStateBlock, err := instance.ledger.getBlock(currentStateBlockNumber)
				if err || nil == currentStateBlock {
					logger.Error("Replica %d just recovered blocks including %d, but cannot find it", instance.id, currentStateBlockNumber)
					continue
				}

				// XXX this is a stupidly wrong place holder, we need to compute the current state block hash
				if instance.ledger.getCurrentStateHash != currentStateBlock.previousBlockHash {
					logger.Warning("Replica %d recovered to an incorrect state at block number %d, retrying", instance.id, currentStateBlockNumber)
					continue
				}

				if !instance.stState.stPlayStateUpToCheckpoint(currentStateBlock, weakCert.blockNumber, weakCert.replicaIds) {
					// This is unlikely, in the future, we may wish to play transactions forward rather than retry
					logger.Warning("Replica %d was unable to play the state from block number %d forward to block %d, retrying", instance.id, currentStateBlockNumber, weakCert.blockNumber)
					continue
				}
			}

		}

	}
}

// This function will retrieve the current state from a replica.
func (instance *pbftCore) stSyncStateSnapshot(minBlockNumber uint64, replicas []int) {
	logger.Debug("Replica %d is initiating state recovery from from replica %d", instance.id, replicaId)

	if true { // XXX Without a ledger implementation none of this works, yet
		return
	}

	instance.ledger.emptyState()

	stateChan, err := instance.ledger.getRemoteCurrentState(replicaId)

	if err != nil {
		instance.ledger.emptyState()
		// TODO, this should trigger recovery from another replica
		return
	}

	for piece := range stateChan {
		instance.ledger.applyStateDelta(piece)
	}

	// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
	// not to consider this state as valid
}

func (instance *pbftCore) stSyncBlockRange(startBlockNumber, endBlockNumber uint64, blockHash string, replicaIds []int) bool {
	return true
}

func (instance *pbftCore) stPlayStateUpToCheckpoint(startBlockNumber, endBlockNumber uint64, blockHash string, replicaIds []int) bool {
	return true
}

func (instance *pbftCore) stRecvCheckpoint(chkpt *Checkpoint) {
	if instance.stState.outOfDate {
		// State transfer is already going on, no need to track this
		return
	}

	H := instance.h + instance.L

	// Track the last observed checkpoint sequence number if it exceeds our high watermark, keyed by replica to prevent unbounded growth
	if chkpt.SequenceNumber < H {
		// For non-byzantine nodes, the checkpoint sequence number increases monotonically
		delete(instance.hChkpts, chkpt.ReplicaId)
	} else {
		// We do not track the highest one, as a byzantine node could pick an arbitrarilly high sequence number
		// and even if it recovered to be non-byzantine, we would still believe it to be far ahead
		instance.hChkpts[chkpt.ReplicaId] = chkpt.SequenceNumber

		// If f+1 other replicas have reported checkpoints that were (at one time) outside our watermarks
		// we need to check to see if we have fallen behind.
		if len(instance.hChkpts) >= instance.f+1 {
			chkptSeqNumArray := make([]uint64, len(instance.hChkpts))
			index := 0
			for replicaId, hChkpt := range instance.hChkpts {
				chkptSeqNumArray[index] = hChkpt
				index++
				if hChkpt < H {
					delete(instance.hChkpts, replicaId)
				}
			}
			sort.Sort(sortableUint64Array(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(instance.hChkpts)-(instance.f+1)]; m > H {
				logger.Warning("Replica is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", chkpt.SequenceNumber, H)
				instance.stState.outOfDate = true
				instance.moveWatermarks(m + instance.K)

				// Pick some replica whose last checkpoint was above the f+1th highest checkpoint
				// but still within a checkpoint.  Guaranteed to be at least one such because
				// the f+1th highest is exactly m
				for replicaId, hChkpt := range instance.hChkpts {
					if hChkpt >= m && hChkpt < m+k {
						go stRecoverState(replicaId)
					}
				}
			}

			return
		}
	}

}

func (instance *pbftCore) stCheckpointWeakCert(matching int, chkpt *Checkpoint) {
	select {
	default:
		return
	case certReq := <-instance.stState.chkptWeakCertReq:
		if certReq.blockNumber > chkpt.BlockNumber {
			// There's a pending request, but this call didn't satisfy it, so put it back
			instance.stState.chkptWeakCertReq <- certReq
			return
		}

		checkpointMembers := make([]int, matching)
		i := 0
		for testChkpt := range instance.checkpointStore {
			if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.StateDigest == chkpt.StateDigest {
				checkpointMembers = i
			}
		}

		certReq.certChan <- &stWeakChkptCert{
			blockNumber: chkpt.BlockNumber,
			blockHash:   chkpt.stateHash,
			replicaIds:  checkpointMembers,
		}
	}

}

// A thread to process state validation and manipulation
func (instance *pbftCore) isOutOfDate() {

}

// A thread to process state validation and manipulation
func (instance *pbftCore) stStateThread() {

}

/*
// This function will retrieve blocks up to the checkpoint sequence number
// then verify that the the blockchain is correct, verify that the state is
// correct for its current block, and finally attempt to play the state forward
// via state deltas, or via blocks if that fails.  On successful execution,
// the instance.outOfDate will be set to false
func (instance *pbftCore) stVerifyAndPlayStateUpToDate(chkpt *Checkpoint, replicaIds []int) {
	logger.Debug("Replica %d is attempting to verify recovered state and come up to date", instance.id)

	if true { // XXX Without a ledger implementation none of this works, yet
		return
	}

	currentStateBlock, ok := instance.ledger.getCurrentStateBlockNumber()
	if !ok {
		logger.Warning("Replica %d current state is bad, initiating state transfer", replicaId)
		// TODO, handle picking another replica to try to get the state from again
		return
	}

	if currentStateBlock < chkpt.BlockNumber-instance.K {
		logger.Warning("Replica %d current state is at block number %d but just observed checkpoint quorum for block %d, state is too far behind, initiating transfer",
			replicaId, chkpt.blockNumber, currentStateBlock)
		// TODO, handle picking another replica to try to get the state from again
		return
	}

	if currentStateBlock > chkpt.BlockNumber+instance.L {
		// We know only that the currentStateBlock should be ahead of the witnessed checkpoint
		// Still, we have a problem if a byzantine replica gives us state for a block that is supposedly very far


// This function will retrieve blocks up to the checkpoint sequence number
// then verify that the the blockchain is correct, verify that the state is
// correct for its current block, and finally attempt to play the state forward
// via state deltas, or via blocks if that fails.  On successful execution,
// the instance.outOfDate will be set to false
func (instance *pbftCore) stVerifyAndPlayStateUpToDate(chkpt *Checkpoint, replicaIds []int) {
	logger.Debug("Replica %d is attempting to verify recovered state and come up to date", instance.id)

	if true { // XXX Without a ledger implementation none of this works, yet
		return
	}

	currentStateBlock, ok := instance.ledger.getCurrentStateBlockNumber()
	if !ok {
		logger.Warning("Replica %d current state is bad, initiating state transfer", replicaId)
		// TODO, handle picking another replica to try to get the state from again
		return
	}

	if currentStateBlock < chkpt.BlockNumber-instance.K {
		logger.Warning("Replica %d current state is at block number %d but just observed checkpoint quorum for block %d, state is too far behind, initiating transfer",
			replicaId, chkpt.blockNumber, currentStateBlock)
		// TODO, handle picking another replica to try to get the state from again
		return
	}

	if currentStateBlock > chkpt.BlockNumber+instance.L {
		// We know only that the currentStateBlock should be ahead of the witnessed checkpoint
		// Still, we have a problem if a byzantine replica gives us state for a block that is supposedly very far
		// in the future.  We assume that we can retrieve the current state within L new blocks, but
		// if this is not the case, we may never be able to catch up
		logger.Warning("Replica %d current state is at block number %d but just observed checkpoint quorum for block %d, state is too far ahead, initiating transfer",
			replicaId, chkpt.blockNumber, currentStateBlock)
		// TODO, handle picking another replica to try to get the state from again
		return
	}

	oldLatestBlock := instance.ledger.getBlockchainSize() - 1

	if currentStateBlock > chckpt.BlockNumber {
		// Our state is ahead of the checkpoint, but not too far ahead, we will wait for the checkpoints to catch up
		// But in the meantime, we'll catch our blockchain up and verify it
		go instance.recoverBlockRange(replicaIds, chkpt.BlockNumber, oldLatestBlock, chkpt.StateDigest)
		return
	}

	// At this point, we are guaranteed that the current state is older than, but within k of the checkpoint block
	// So blocking to synchronizing blocks should be okay
	instance.recoverBlockRange(replicaIds, chkpt.BlockNumber, currentStateBlock, chkpt.StateDigest)

	if currentStateBlock > oldLatestBlock {
		// Recover the chain through our previously known block
		// this could be quite large, so necessary to do asynchronously
		go instance.recoverBlockRange(replicaIds, currentStateBlock, oldLatestBlock, chkpt.StateDigest)
	}

	stateHash := instance.ledger.getCurrentStateHash()

	if stateHash != instance.ledger.getBlock(currentStateBlock).StateHash {
		// Our blocks are up to date, but we were given a bad state
		// TODO, pick another replica and get the state again
		return
	}

	playStateUpToDate(replicaIds) // This should not be possible to fail

	instance.outOfDate = false
	instance.executeOutstanding()
}
*/
