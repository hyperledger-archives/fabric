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
	"sync"
	"time"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
)

// =============================================================================
// init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/statetransfer")
}

// =============================================================================
// public methods and structure definitions
// =============================================================================

type Listener interface {
	Initiated()                                                   // Called when the state transfer thread starts a new state transfer
	Errored(uint64, []byte, []*protos.PeerID, interface{}, error) // Called when an error is encountered during state transfer, only the error is guaranteed to be set, other fields will be set on a best effort basis
	Completed(uint64, []byte, []*protos.PeerID, interface{})      // Called when the state transfer is completed
}

// This provides a simple base implementation of a state transfer listener which implementors can extend anonymously
// Unset fields result in no action for that event
type ProtoListener struct {
	InitiatedImpl func()
	ErroredImpl   func(uint64, []byte, []*protos.PeerID, interface{}, error)
	CompletedImpl func(uint64, []byte, []*protos.PeerID, interface{})
}

func (pstl *ProtoListener) Initiated() {
	if nil != pstl.InitiatedImpl {
		pstl.InitiatedImpl()
	}
}

func (pstl *ProtoListener) Errored(bn uint64, bh []byte, pids []*protos.PeerID, m interface{}, e error) {
	if nil != pstl.ErroredImpl {
		pstl.ErroredImpl(bn, bh, pids, m, e)
	}
}

func (pstl *ProtoListener) Completed(bn uint64, bh []byte, pids []*protos.PeerID, m interface{}) {
	if nil != pstl.CompletedImpl {
		pstl.CompletedImpl(bn, bh, pids, m)
	}
}

type StateTransferState struct {
	ledger consensus.LedgerStack

	asynchronousTransferInProgress bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	id *protos.PeerID // Useful log messages and not attempting to recover from ourselves

	stateValid bool // Are we currently operating under the assumption that the state is valid?

	defaultPeerIDs []*protos.PeerID // This is a list of peers which will be queried when no explicit list is given

	blockVerifyChunkSize uint64        // The max block length to attempt to sync at once, this prevents state transfer from being delayed while the blockchain is validated
	validBlockRanges     []*blockRange // Used by the block thread to track which pieces of the blockchain have already been hashed
	RecoverDamage        bool          // Whether state transfer should ever modify or delete existing blocks if they are determined to be corrupted

	initiateStateSync chan *syncMark       // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	blockHashReceiver chan *blockHashReply // Used to process incoming valid block hashes, write only from the state thread
	blockSyncReq      chan *blockSyncReq   // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread

	blockThreadExit chan struct{} // Used to inform the block thread that we are shutting down
	stateThreadExit chan struct{} // Used to inform the state thread that we are shutting down

	BlockRequestTimeout         time.Duration // How long to wait for a peer to respond to a block request
	StateDeltaRequestTimeout    time.Duration // How long to wait for a peer to respond to a state delta request
	StateSnapshotRequestTimeout time.Duration // How long to wait for a peer to respond to a state snapshot request

	stateTransferListeners     []Listener  // A list of listeners to call when state transfer is initiated/errored/completed
	stateTransferListenersLock *sync.Mutex // Used to lock the above list when adding a listener
}

// Adds a target and blocks until that target's success or failure
// If peerIDs is nil, all peers will be considered sync candidates
// The function returns nil on success or error
// If state sync completes, but to a different target, this is still considered an error
func (sts *StateTransferState) BlockingAddTarget(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID) error {
	result := make(chan error)

	listener := struct{ ProtoListener }{}

	listener.ErroredImpl = func(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}, err error) {
		result <- err
	}

	listener.CompletedImpl = func(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}) {
		result <- nil
	}

	sts.RegisterListener(&listener)
	defer sts.UnregisterListener(&listener)

	sts.AddTarget(blockNumber, blockHash, peerIDs, nil)

	return <-result
}

// Starts the state sync process, without blocking
// For the sync to complete, a call to AddTarget(hash, peerIDs) must be made
// If peerIDs is nil, all peer will be considered sync candidates
func (sts *StateTransferState) Initiate(peerIDs []*protos.PeerID) {
	sts.initiateStateSync <- &syncMark{
		blockNumber: 0,
		peerIDs:     peerIDs,
	}
	sts.asynchronousTransferInProgress = true // To prevent a race this needs to be done in the initiating thread
}

// Informs the asynchronous sync of a new valid block hash, as well as a list of peers which should be capable of supplying that block
// If the peerIDs are nil, then all peers are assumed to have the given block
func (sts *StateTransferState) AddTarget(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID, metadata interface{}) {
	logger.Debug("%v informed of a new block hash for block number %d with peers %v", sts.id, blockNumber, peerIDs)
	blockHashReply := &blockHashReply{
		syncMark: syncMark{
			blockNumber: blockNumber,
			peerIDs:     peerIDs,
		},
		blockHash: blockHash,
		metadata:  metadata,
	}

	for {
		select {
		// This channel has a buffer of one, so this loop should always exit eventually
		case sts.blockHashReceiver <- blockHashReply:
			logger.Debug("%v block hash reply for block %d queued for state transfer", sts.id, blockNumber)
			return

		case lastHash := <-sts.blockHashReceiver:
			logger.Debug("%v block hash reply for block %d discarded", sts.id, lastHash.blockNumber)
		}
	}

}

// The registered interface implementation will be invoked whenever state transfer is initiated or completed, or encounters an error
func (sts *StateTransferState) RegisterListener(listener Listener) {
	sts.stateTransferListenersLock.Lock()
	defer func() {
		sts.stateTransferListenersLock.Unlock()
	}()

	sts.stateTransferListeners = append(sts.stateTransferListeners, listener)
}

// No longer receive state transfer updates sent to the given function.
// Listeners must be comparable in order to be unregistered
func (sts *StateTransferState) UnregisterListener(listener Listener) {
	sts.stateTransferListenersLock.Lock()
	defer func() {
		sts.stateTransferListenersLock.Unlock()
	}()

	for i, l := range sts.stateTransferListeners {
		if listener == l {
			for j := i + 1; j < len(sts.stateTransferListeners); j++ {
				sts.stateTransferListeners[j-1] = sts.stateTransferListeners[j]
			}
			sts.stateTransferListeners = sts.stateTransferListeners[:len(sts.stateTransferListeners)-1]
			return
		}
	}
}

// This is a simple convenience method, for listeners who wish only to be notified that the state transfer has completed, without any additional information
// For more sophisticated information, use the RegisterListener call.  This channel never closes but receives a message every time transfer completes
func (sts *StateTransferState) CompletionChannel() chan struct{} {
	complete := make(chan struct{}, 1)
	listener := struct{ ProtoListener }{}
	listener.CompletedImpl = func(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}) {
		select {
		case complete <- struct{}{}:
		default:
		}
	}
	sts.RegisterListener(&listener)
	return complete
}

// Whether state transfer is currently occuring.  Note, this is not a thread safe call, it is expected
// that the caller synchronizes around state transfer if it is to be accessed in a non-serial fashion
func (sts *StateTransferState) InProgress() bool {
	return sts.asynchronousTransferInProgress
}

// Inform state transfer that the current state is invalid.  This will trigger an immediate full state snapshot sync
// when state transfer is initiated
func (sts *StateTransferState) InvalidateState() {
	sts.stateValid = false
}

// This will send a signal to any running threads to stop, regardless of whether they are stopped
// It will never block, and if called before threads start, they will exit at startup
// Attempting to start threads after this call may fail once
func (sts *StateTransferState) Stop() {
outer:
	for {
		select {
		case sts.blockThreadExit <- struct{}{}:
		case sts.stateThreadExit <- struct{}{}:
		default:
			break outer
		}
	}
}

// =============================================================================
// constructors
// =============================================================================

func ThreadlessNewStateTransferState(id *protos.PeerID, config *viper.Viper, ledger consensus.LedgerStack, defaultPeerIDs []*protos.PeerID) *StateTransferState {
	sts := &StateTransferState{}

	sts.stateTransferListenersLock = &sync.Mutex{}

	sts.ledger = ledger
	sts.id = id

	sts.asynchronousTransferInProgress = false

	logger.Debug("%v assigning %v to defaultPeerIDs", id, defaultPeerIDs)
	sts.defaultPeerIDs = defaultPeerIDs

	sts.RecoverDamage = config.GetBool("statetransfer.recoverdamage")

	sts.stateValid = true // Assume our starting state is correct unless told otherwise

	sts.validBlockRanges = make([]*blockRange, 0)
	sts.blockVerifyChunkSize = uint64(config.GetInt("statetransfer.blocksperrequest"))
	if sts.blockVerifyChunkSize == 0 {
		panic(fmt.Errorf("Must set statetransfer.blocksperrequest to be nonzero"))
	}

	sts.initiateStateSync = make(chan *syncMark)
	sts.blockHashReceiver = make(chan *blockHashReply, 1)
	sts.blockSyncReq = make(chan *blockSyncReq)

	sts.blockThreadExit = make(chan struct{}, 1)
	sts.stateThreadExit = make(chan struct{}, 1)

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

func NewStateTransferState(id *protos.PeerID, config *viper.Viper, ledger consensus.LedgerStack, defaultPeerIDs []*protos.PeerID) *StateTransferState {
	sts := ThreadlessNewStateTransferState(id, config, ledger, defaultPeerIDs)

	go sts.stateThread()
	go sts.blockThread()

	return sts
}

// =============================================================================
// custom interfaces and structure definitions
// =============================================================================

type StateTransferUpdate int

const (
	Initiated StateTransferUpdate = iota
	Errored
	Completed
)

type syncMark struct {
	blockNumber uint64
	peerIDs     []*protos.PeerID
}

type blockHashReply struct {
	syncMark
	blockHash []byte
	metadata  interface{}
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

// Executes a func trying each peer included in peerIDs until successful
// Attempts to execute over all peers if peerIDs is nil
func (sts *StateTransferState) tryOverPeers(passedPeerIDs []*protos.PeerID, do func(peerID *protos.PeerID) error) (err error) {

	peerIDs := passedPeerIDs

	if nil == passedPeerIDs {
		logger.Debug("tryOverPeers, no peerIDs given, using default")
		peerIDs = sts.defaultPeerIDs
	}

	logger.Debug("%v in tryOverPeers, using peerIDs: %v", sts.id, peerIDs)

	if 0 == len(peerIDs) {
		return fmt.Errorf("Cannot tryOverPeers with no peers specified")
	}

	numReplicas := len(peerIDs)
	startIndex := rand.Int() % numReplicas

	for i := 0; i < numReplicas; i++ {
		index := (i + startIndex) % numReplicas
		err = do(peerIDs[index])
		if err == nil {
			break
		} else {
			logger.Warning("%v in tryOverPeers loop trying %v : %s", sts.id, peerIDs[index], err)
		}
	}

	return err

}

// Attempts to complete a blockSyncReq using the supplied peers
// Will return the last block number attempted to sync, and the last block successfully synced (or nil) and error on failure
// This means on failure, the returned block corresponds to 1 higher than the returned block number
func (sts *StateTransferState) syncBlocks(highBlock, lowBlock uint64, highHash []byte, peerIDs []*protos.PeerID) (uint64, *protos.Block, error) {
	logger.Debug("%v syncing blocks from %d to %d", sts.id, highBlock, lowBlock)
	validBlockHash := highHash
	blockCursor := highBlock
	var block *protos.Block

	err := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {
		blockChan, err := sts.ledger.GetRemoteBlocks(peerID, blockCursor, lowBlock)
		if nil != err {
			logger.Warning("%v failed to get blocks from %d to %d from %v: %s",
				sts.id, blockCursor, lowBlock, peerID, err)
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
						return fmt.Errorf("%v got a block %d which could not hash from %v: %s",
							sts.id, blockCursor, peerID, err)
					}

					if !bytes.Equal(testHash, validBlockHash) {
						return fmt.Errorf("%v got block %d from %v with hash %x, was expecting hash %x",
							sts.id, blockCursor, peerID, testHash, validBlockHash)
					}

					logger.Debug("%v putting block %d to with PreviousBlockHash %x and StateHash %x", sts.id, blockCursor, block.PreviousBlockHash, block.StateHash)
					if !sts.RecoverDamage {

						// If we are not supposed to be destructive in our recovery, check to make sure this block doesn't already exist
						if oldBlock, err := sts.ledger.GetBlock(blockCursor); err == nil && oldBlock != nil {
							oldBlockHash, err := sts.ledger.HashBlock(oldBlock)
							if nil == err {
								if !bytes.Equal(oldBlockHash, validBlockHash) {
									panic("The blockchain is corrupt and the configuration has specified that bad blocks should not be deleted/overridden")
								}
							} else {
								logger.Error("%v could not compute the hash of block %d", sts.id, blockCursor)
								panic("The blockchain is corrupt and the configuration has specified that bad blocks should not be deleted/overridden")
							}
							logger.Debug("%v not actually putting block %d to with PreviousBlockHash %x and StateHash %x, as it already exists", sts.id, blockCursor, block.PreviousBlockHash, block.StateHash)
						} else {
							sts.ledger.PutBlock(blockCursor, block)
						}
					} else {
						sts.ledger.PutBlock(blockCursor, block)
					}

					validBlockHash = block.PreviousBlockHash

					if blockCursor == lowBlock {
						logger.Debug("%v successfully synced from block %d to block %d", sts.id, highBlock, lowBlock)
						return nil
					}
					blockCursor--

				}
			case <-time.After(sts.BlockRequestTimeout):
				return fmt.Errorf("%v had block sync request to %v time out", sts.id, peerID)
			}
		}
	})

	if nil != block {
		logger.Debug("%v returned from sync with block %d and hash %x", sts.id, blockCursor, block.StateHash)
	} else {
		logger.Debug("%v returned from sync with no new blocks", sts.id)
	}

	return blockCursor, block, err

}

func (sts *StateTransferState) syncBlockchainToCheckpoint(blockSyncReq *blockSyncReq) {

	logger.Debug("%v is processing a blockSyncReq to block %d", sts.id, blockSyncReq.blockNumber)

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

		blockNumber, block, err := sts.syncBlocks(blockSyncReq.blockNumber, blockSyncReq.reportOnBlock, blockSyncReq.firstBlockHash, blockSyncReq.peerIDs)

		goodRange := &blockRange{
			highBlock: blockSyncReq.blockNumber,
		}

		if nil != blockSyncReq.replyChan {
			logger.Debug("%v replying to blockSyncReq on reply channel with : %s", sts.id, err)
			blockSyncReq.replyChan <- err
			goodRange.lowBlock = blockNumber
		}

		if nil == err {
			goodRange.lowNextHash = block.PreviousBlockHash
			sts.validBlockRanges = append(sts.validBlockRanges, goodRange)
		}
	}
}

// This function should never be called directly, its public visibility is purely a side effect
// of the package scoping of go test
func (sts *StateTransferState) VerifyAndRecoverBlockchain() bool {

	if 0 == len(sts.validBlockRanges) {
		size, err := sts.ledger.GetBlockchainSize()
		if nil != err {
			panic("We cannot determine how long our blockchain is, this is irrecoverable")
		}
		if 0 == size {
			logger.Warning("%v has no blocks in its blockchain, including the genesis block", sts.id)
			return false
		}

		block, err := sts.ledger.GetBlock(size - 1)
		if nil != err {
			logger.Warning("%v could not retrieve its head block %d: %s", sts.id, size, err)
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
					logger.Warning("%v could not retrieve block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
				} else {
					if blockHash, err := sts.ledger.HashBlock(block); nil != err {
						if bytes.Equal(blockHash, lowNextHash) {
							// The chains connect, no need to validate all the way down
							sts.validBlockRanges[0].lowBlock = sts.validBlockRanges[1].lowBlock
							sts.validBlockRanges[0].lowNextHash = sts.validBlockRanges[1].lowNextHash
						} else {
							logger.Warning("%v detected that a block range starting at %d previously believed to be valid did not hash correctly", sts.id, lowBlock-1)
						}
					} else {
						logger.Warning("%v could not hash block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
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

	blockNumber, block, err := sts.syncBlocks(lowBlock-1, targetBlock, lowNextHash, nil)

	if blockNumber == lowBlock-1 || nil == block {
		logger.Warning("%v unable to recover any blocks : %s", sts.id, err)
		return false
	}

	sts.validBlockRanges[0].lowNextHash = block.PreviousBlockHash

	if nil == err {
		sts.validBlockRanges[0].lowBlock = blockNumber
	} else {
		sts.validBlockRanges[0].lowBlock = blockNumber + 1
		logger.Warning("%v unable to recover block %d : %s", sts.id, blockNumber, err)
	}

	return false
}

func (sts *StateTransferState) blockThread() {

	for {

		select {
		case blockSyncReq := <-sts.blockSyncReq:
			sts.syncBlockchainToCheckpoint(blockSyncReq)
		case <-sts.blockThreadExit:
			logger.Debug("Received request for block transfer thread to exit (1)")
			return
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
			if !sts.VerifyAndRecoverBlockchain() {
				// There is more verification to be done, so loop
				continue
			}
		}

		logger.Debug("%v has validated its blockchain to the genesis block", sts.id)

		select {
		// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
		case blockSyncReq := <-sts.blockSyncReq:
			sts.syncBlockchainToCheckpoint(blockSyncReq)
		case <-sts.blockThreadExit:
			logger.Debug("Received request for block transfer thread to exit (2)")
			return
		}

	}
}

func (sts *StateTransferState) attemptStateTransfer(currentStateBlockNumber *uint64, mark **syncMark, blockHReply **blockHashReply, blocksValid *bool) error {
	var err error

	if !sts.stateValid {
		// Our state is currently bad, so get a new one
		*currentStateBlockNumber, err = sts.syncStateSnapshot((*mark).blockNumber, (*mark).peerIDs)

		if nil != err {
			*mark = &syncMark{ // Let's try to just sync state from anyone, for any sequence number
				blockNumber: 0,
				peerIDs:     nil,
			}
			return fmt.Errorf("%v could not retrieve state as recent as %d from any of specified peers", sts.id, (*mark).blockNumber)
		}

		logger.Debug("%v completed state transfer to block %d", sts.id, *currentStateBlockNumber)
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
			logger.Debug("%v has no valid block hash to validate the blockchain with yet, waiting for a known valid block hash", sts.id)
		} else {
			logger.Debug("%v already has valid blocks through %d but needs to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
		}

		for {
			if *blockHReply = <-sts.blockHashReceiver; (*blockHReply).blockNumber < *currentStateBlockNumber {
				logger.Debug("%v received a block hash reply for block number %d, which is not high enough", sts.id, (*blockHReply).blockNumber)
			} else {
				break
			}
		}
		logger.Debug("%v received a block hash reply for block %d with sync sources %v", sts.id, (*blockHReply).blockNumber, (*blockHReply).syncMark.peerIDs)
		*blocksValid = false // We retrieved a new hash, we will need to sync to a new block
	}

	if !*blocksValid {
		(*mark) = &((*blockHReply).syncMark) // We now know of a more recent block hash

		blockReplyChannel := make(chan error)

		sts.blockSyncReq <- &blockSyncReq{
			syncMark:       *(*mark),
			reportOnBlock:  *currentStateBlockNumber,
			replyChan:      blockReplyChannel,
			firstBlockHash: (*blockHReply).blockHash,
		}

		logger.Debug("%v state transfer thread waiting for block sync to complete", sts.id)
		err = <-blockReplyChannel
		logger.Debug("%v state transfer thread continuing", sts.id)

		if err != nil {
			return fmt.Errorf("%v could not retrieve blocks as recent as %d as the block hash advertised", sts.id, (*mark).blockNumber)
		}

		*blocksValid = true
	} else {
		logger.Debug("%v already has valid blocks through %d necessary to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
	}

	stateHash, err := sts.ledger.GetCurrentStateHash()
	if nil != err {
		sts.stateValid = false
		return fmt.Errorf("%v could not compute its current state hash: %x", sts.id, err)

	}

	block, err := sts.ledger.GetBlock(*currentStateBlockNumber)
	if nil != err {
		*blocksValid = false
		return fmt.Errorf("%v believed its state for block %d to be valid, but it could not retrieve it : %s", sts.id, *currentStateBlockNumber, err)
	}

	if !bytes.Equal(stateHash, block.StateHash) {
		if sts.stateValid {
			sts.stateValid = false
			return fmt.Errorf("%v believed its state for block %d to be valid, but its hash (%x) did not match the recovered blockchain's (%x)", sts.id, (*currentStateBlockNumber), stateHash, block.StateHash)
		} else {
			return fmt.Errorf("%v recovered to an incorrect state at block number %d, (%x %x) retrying", sts.id, *currentStateBlockNumber, stateHash, block.StateHash)
		}
	} else {
		logger.Debug("%v state is now valid", sts.id)
	}

	sts.stateValid = true

	if *currentStateBlockNumber < (*blockHReply).blockNumber {
		*currentStateBlockNumber, err = sts.playStateUpToCheckpoint(*currentStateBlockNumber+uint64(1), (*blockHReply).blockNumber, (*blockHReply).peerIDs)
		if nil != err {
			// This is unlikely, in the future, we may wish to play transactions forward rather than retry
			sts.stateValid = false
			return fmt.Errorf("%v was unable to play the state from block number %d forward to block %d, retrying with new state : %s", sts.id, *currentStateBlockNumber, (*blockHReply).blockNumber, err)
		}
	}

	return nil
}

// A thread to process state transfer
func (sts *StateTransferState) stateThread() {
	for {
		select {
		// Wait for state sync to become necessary
		case mark := <-sts.initiateStateSync:

			sts.informListeners(0, nil, mark.peerIDs, nil, nil, Initiated)

			logger.Debug("%v is initiating state transfer", sts.id)

			var currentStateBlockNumber uint64
			var blockHReply *blockHashReply
			blocksValid := false

			for {
				if err := sts.attemptStateTransfer(&currentStateBlockNumber, &mark, &blockHReply, &blocksValid); err != nil {
					logger.Error("%s", err)
					sts.informListeners(0, nil, mark.peerIDs, nil, err, Errored)
					continue
				}

				break
			}

			logger.Debug("%v is completing state transfer", sts.id)

			sts.asynchronousTransferInProgress = false

			sts.informListeners(blockHReply.blockNumber, blockHReply.blockHash, blockHReply.peerIDs, blockHReply.metadata, nil, Completed)

		case <-sts.stateThreadExit:
			logger.Debug("Received request for state thread to exit")
			return
		}
	}
}

func (sts *StateTransferState) informListeners(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID, metadata interface{}, err error, update StateTransferUpdate) {
	sts.stateTransferListenersLock.Lock()
	defer func() {
		sts.stateTransferListenersLock.Unlock()
	}()

	for _, listener := range sts.stateTransferListeners {
		switch update {
		case Initiated:
			listener.Initiated()
		case Errored:
			listener.Errored(blockNumber, blockHash, peerIDs, metadata, err)
		case Completed:
			listener.Completed(blockNumber, blockHash, peerIDs, metadata)
		}
	}
}

func (sts *StateTransferState) playStateUpToCheckpoint(fromBlockNumber, toBlockNumber uint64, peerIDs []*protos.PeerID) (uint64, error) {
	logger.Debug("%v attempting to play state forward from %v to block %d", sts.id, peerIDs, toBlockNumber)
	currentBlock := fromBlockNumber
	err := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {

		deltaMessages, err := sts.ledger.GetRemoteStateDeltas(peerID, currentBlock, toBlockNumber)
		if err != nil {
			return fmt.Errorf("%v received an error while trying to get the state deltas for blocks %d through %d from %d", sts.id, fromBlockNumber, toBlockNumber, peerID)
		}

		for {
			select {
			case deltaMessage, ok := <-deltaMessages:
				if !ok {
					if currentBlock == toBlockNumber+1 {
						return nil
					}
					return fmt.Errorf("%v was only able to recover to block number %d when desired to recover to %d", sts.id, currentBlock, toBlockNumber)
				}

				if deltaMessage.Range.Start != currentBlock || deltaMessage.Range.End < deltaMessage.Range.Start || deltaMessage.Range.End > toBlockNumber {
					continue // this is an unfortunately normal case, as we can get duplicates, just ignore it
				}

				for _, delta := range deltaMessage.Deltas {
					umDelta := &statemgmt.StateDelta{}
					if err := umDelta.Unmarshal(delta); nil != err {
						return fmt.Errorf("%v received a corrupt state delta from %v : %s", sts.id, peerID, err)
					}
					sts.ledger.ApplyStateDelta(deltaMessage, umDelta)
				}

				success := false

				testBlock, err := sts.ledger.GetBlock(deltaMessage.Range.End)

				if nil != err {
					logger.Warning("%v could not retrieve block %d, though it should be present", sts.id, deltaMessage.Range.End)
				} else {

					stateHash, err := sts.ledger.GetCurrentStateHash()
					if nil != err {
						logger.Warning("%v could not compute its state hash for some reason: %s", sts.id, err)
					}
					logger.Debug("%v has played state forward from %v to block %d with StateHash (%x), the corresponding block has StateHash (%x)",
						sts.id, peerID, deltaMessage.Range.End, stateHash, testBlock.StateHash)

					if bytes.Equal(testBlock.StateHash, stateHash) {
						success = true
					}
				}

				if !success {
					if nil != sts.ledger.RollbackStateDelta(deltaMessage) {
						sts.InvalidateState()
						return fmt.Errorf("%v played state forward according to %v, but the state hash did not match, failed to roll back, invalidated state", sts.id, peerID)
					} else {
						return fmt.Errorf("%v played state forward according to %v, but the state hash did not match, rolled back", sts.id, peerID)
					}

				} else {
					currentBlock++
					if nil != sts.ledger.CommitStateDelta(deltaMessage) {
						sts.InvalidateState()
						return fmt.Errorf("%v played state forward according to %v, hashes matched, but failed to commit, invalidated state", sts.id, peerID)
					}
				}
			case <-time.After(sts.StateDeltaRequestTimeout):
				logger.Warning("%v timed out during state delta recovery from %v", sts.id, peerID)
				return fmt.Errorf("%v timed out during state delta recovery from %v", sts.id, peerID)
			}
		}

		return nil

	})
	return currentBlock, err
}

// This function will retrieve the current state from a peer.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (sts *StateTransferState) syncStateSnapshot(minBlockNumber uint64, peerIDs []*protos.PeerID) (uint64, error) {

	currentStateBlock := uint64(0)

	ok := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {
		logger.Debug("%v is initiating state recovery from %v", sts.id, peerID)

		sts.ledger.EmptyState()

		stateChan, err := sts.ledger.GetRemoteStateSnapshot(peerID)

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
				umDelta := &statemgmt.StateDelta{}
				if err := umDelta.Unmarshal(piece.Delta); nil != err {
					return fmt.Errorf("%v received a corrupt delta from %v : %s", sts.id, peerID, err)
				}
				sts.ledger.ApplyStateDelta(piece, umDelta)
				currentStateBlock = piece.BlockNumber
				if nil != sts.ledger.CommitStateDelta(piece) {
					return fmt.Errorf("%v could not commit state delta from %v", sts.id, peerID)
				}
			case <-timer.C:
				return fmt.Errorf("%v timed out during state recovery from %v", sts.id, peerID)
			}
		}

	})

	return currentStateBlock, ok
}
