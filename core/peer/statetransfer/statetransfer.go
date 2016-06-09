/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package statetransfer

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	_ "github.com/hyperledger/fabric/core" // Logging format init

	"github.com/hyperledger/fabric/core/ledger/statemgmt"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
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

// PartialStack is a subset of peer.MessageHandlerCoordinator functionality which is necessary to perform state transfer
type PartialStack interface {
	peer.BlockChainAccessor
	peer.BlockChainModifier
	peer.BlockChainUtil
	GetPeers() (*protos.PeersMessage, error)
	GetPeerEndpoint() (*protos.PeerEndpoint, error)
	GetRemoteLedger(receiver *protos.PeerID) (peer.RemoteLedger, error)
}

// Listener is an interface which allows for other modules to register to receive events about the progress of state transfer
type Listener interface {
	Initiated(uint64, []byte, []*protos.PeerID, interface{})      // Called when the state transfer thread starts a new state transfe
	Errored(uint64, []byte, []*protos.PeerID, interface{}, error) // Called when an error is encountered during state transfer, only the error is guaranteed to be set, other fields will be set on a best effort basis
	Completed(uint64, []byte, []*protos.PeerID, interface{})      // Called when the state transfer is completed
}

// ProtoListener provides a simple base implementation of a state transfer listener which implementors can extend anonymously
// Unset fields result in no action for that event
type ProtoListener struct {
	InitiatedImpl func(uint64, []byte, []*protos.PeerID, interface{})
	ErroredImpl   func(uint64, []byte, []*protos.PeerID, interface{}, error)
	CompletedImpl func(uint64, []byte, []*protos.PeerID, interface{})
}

// Initiated invokes the corresponding InitiatedImpl if it has been overwritten
func (pstl *ProtoListener) Initiated(bn uint64, bh []byte, pids []*protos.PeerID, m interface{}) {
	if nil != pstl.InitiatedImpl {
		pstl.InitiatedImpl(bn, bh, pids, m)
	}
}

// Errored invokes the corresponding ErroredImpl if it has been overwritten
func (pstl *ProtoListener) Errored(bn uint64, bh []byte, pids []*protos.PeerID, m interface{}, e error) {
	if nil != pstl.ErroredImpl {
		pstl.ErroredImpl(bn, bh, pids, m, e)
	}
}

// Completed invokes the corresponding CompletedImpl if it has been overwritten
func (pstl *ProtoListener) Completed(bn uint64, bh []byte, pids []*protos.PeerID, m interface{}) {
	if nil != pstl.CompletedImpl {
		pstl.CompletedImpl(bn, bh, pids, m)
	}
}

// StateTransferState is the structure used to manage the state of state transfer
type StateTransferState struct {
	stack PartialStack

	DiscoveryThrottleTime time.Duration // The amount of time to wait after discovering there are no connected peers

	asynchronousTransferInProgress bool // To be used by the main consensus thread, not atomic, so do not use by other threads

	id *protos.PeerID // Useful log messages and not attempting to recover from ourselves

	stateValid bool // Are we currently operating under the assumption that the state is valid?

	blockVerifyChunkSize uint64        // The max block length to attempt to sync at once, this prevents state transfer from being delayed while the blockchain is validated
	validBlockRanges     []*blockRange // Used by the block thread to track which pieces of the blockchain have already been hashed
	RecoverDamage        bool          // Whether state transfer should ever modify or delete existing blocks if they are determined to be corrupted

	initiateStateSync chan *blockHashReply // Used to ensure only one state transfer at a time occurs, write to only from the main consensus thread
	blockHashReceiver chan *blockHashReply // Used to process incoming valid block hashes, write only from the state thread
	blockSyncReq      chan *blockSyncReq   // Used to request a block sync, new requests cause the existing request to abort, write only from the state thread

	threadExit chan struct{} // Used to inform the threads that we are shutting down

	blockThreadIdle bool // Used to check if the block thread is idle
	stateThreadIdle bool // Used to check if the state thread is idle

	blockThreadIdleChan chan struct{} // Used to block until the block thread is idle
	stateThreadIdleChan chan struct{} // Used to block until the state thread is idle

	BlockRequestTimeout         time.Duration // How long to wait for a peer to respond to a block request
	StateDeltaRequestTimeout    time.Duration // How long to wait for a peer to respond to a state delta request
	StateSnapshotRequestTimeout time.Duration // How long to wait for a peer to respond to a state snapshot request

	maxStateDeltas     int    // The maximum number of state deltas to attempt to retrieve before giving up and performing a full state snapshot retrieval
	maxBlockRange      uint64 // The maximum number blocks to attempt to retrieve at once, to prevent from overflowing the peer's buffer
	maxStateDeltaRange uint64 // The maximum number of state deltas to attempt to retrieve at once, to prevent from overflowing the peer's buffer

	stateTransferListeners     []Listener  // A list of listeners to call when state transfer is initiated/errored/completed
	stateTransferListenersLock *sync.Mutex // Used to lock the above list when adding a listener
}

// BlockingAddTarget Adds a target and blocks until that target's success or failure
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

// BlockingUntilSuccessAddTarget adds a target and blocks until that target's success
// If peerIDs is nil, all peers will be considered sync candidates
// This call should only be used in scenarios where an error should result in a panic, as this could otherwise cause a deadlock
func (sts *StateTransferState) BlockingUntilSuccessAddTarget(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID) {
	result := make(chan error)

	listener := struct{ ProtoListener }{}

	listener.CompletedImpl = func(bn uint64, bh []byte, pids []*protos.PeerID, md interface{}) {
		result <- nil
	}

	sts.RegisterListener(&listener)
	defer sts.UnregisterListener(&listener)

	sts.AddTarget(blockNumber, blockHash, peerIDs, nil)

	<-result
}

// AddTarget Informs the asynchronous sync of a new valid block hash, as well as a list of peers which should be capable of supplying that block
// If the peerIDs are nil, then all peers are assumed to have the given block.  If state transfer has not been initiated already,
// this will kick state transfer off
func (sts *StateTransferState) AddTarget(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID, metadata interface{}) {
	logger.Debug("%v informed of a new block hash for block number %d with peers %v", sts.id, blockNumber, peerIDs)
	bhr := &blockHashReply{
		syncMark: syncMark{
			blockNumber: blockNumber,
			peerIDs:     peerIDs,
		},
		blockHash: blockHash,
		metadata:  metadata,
	}

	sts.stateTransferListenersLock.Lock()
	if sts.asynchronousTransferInProgress {
		logger.Debug("%v state transfer already in progress, not kicking off", sts.id)
		sts.stateTransferListenersLock.Unlock()
	} else {
		sts.asynchronousTransferInProgress = true
		sts.stateTransferListenersLock.Unlock()
		select {
		case sts.initiateStateSync <- &blockHashReply{
			syncMark: syncMark{
				blockNumber: 0,
				peerIDs:     peerIDs,
			},
			blockHash: blockHash,
			metadata:  metadata,
		}:
			logger.Debug("%v initiating a new state transfer request", sts.id)
		case <-sts.threadExit:
			logger.Debug("%v state transfer has been told to exit", sts.id)
			return
		}
	}

	for {
		select {
		// This channel has a buffer of one, so this loop should always exit eventually
		case sts.blockHashReceiver <- bhr:
			logger.Debug("%v block hash reply for block %d queued for state transfer", sts.id, blockNumber)
			return

		case lastHash := <-sts.blockHashReceiver:
			logger.Debug("%v block hash reply for block %d discarded", sts.id, lastHash.blockNumber)
		}
	}

}

// RegisterListener registers a listener which will be invoked whenever state transfer is initiated or completed, or encounters an error
func (sts *StateTransferState) RegisterListener(listener Listener) {
	sts.stateTransferListenersLock.Lock()
	defer func() {
		sts.stateTransferListenersLock.Unlock()
	}()

	sts.stateTransferListeners = append(sts.stateTransferListeners, listener)
}

// UnregisterListener unregisters a listener so that it no longer receive state transfer updates
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

// CompletionChannel is a simple convenience method, for listeners who wish only to be notified that the state transfer has completed, without any additional information
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

// InProgress returns whether state transfer is currently occurring.  Note, this is not a thread safe call, it is expected
// that the caller synchronizes around state transfer if it is to be accessed in a non-serial fashion
func (sts *StateTransferState) InProgress() bool {
	return sts.asynchronousTransferInProgress
}

// InvalidateState informs state transfer that the current state is invalid.  This will trigger an immediate full state snapshot sync
// when state transfer is initiated
func (sts *StateTransferState) InvalidateState() {
	sts.stateValid = false
}

// Stop sends a signal to any running threads to stop, regardless of whether they are stopped
// It will never block, and if called before threads start, they will exit at startup
func (sts *StateTransferState) Stop() {
	select {
	case <-sts.threadExit:
	default:
		close(sts.threadExit)
	}
}

// =============================================================================
// constructors
// =============================================================================

// threadlessNewStateTransferState constructs StateTransferState, but does not start any of its threads
// this is useful primarily for testing
func threadlessNewStateTransferState(stack PartialStack) *StateTransferState {
	var err error
	sts := &StateTransferState{}

	sts.stateTransferListenersLock = &sync.Mutex{}

	sts.stack = stack
	ep, err := stack.GetPeerEndpoint()

	if nil != err {
		logger.Debug("Error resolving our own PeerID, this shouldn't happen")
		sts.id = &protos.PeerID{"ERROR_RESOLVING_ID"}
	}

	sts.id = ep.ID

	sts.RecoverDamage = viper.GetBool("statetransfer.recoverdamage")

	sts.stateValid = true // Assume our starting state is correct unless told otherwise

	sts.validBlockRanges = make([]*blockRange, 0)
	sts.blockVerifyChunkSize = uint64(viper.GetInt("statetransfer.blocksperrequest"))
	if sts.blockVerifyChunkSize == 0 {
		panic(fmt.Errorf("Must set statetransfer.blocksperrequest to be nonzero"))
	}

	sts.initiateStateSync = make(chan *blockHashReply)
	sts.blockHashReceiver = make(chan *blockHashReply, 1)
	sts.blockSyncReq = make(chan *blockSyncReq)

	sts.threadExit = make(chan struct{})

	sts.blockThreadIdle = true
	sts.stateThreadIdle = true

	sts.blockThreadIdleChan = make(chan struct{})
	sts.stateThreadIdleChan = make(chan struct{})

	sts.DiscoveryThrottleTime = 1 * time.Second // TODO make this configurable

	sts.BlockRequestTimeout, err = time.ParseDuration(viper.GetString("statetransfer.timeout.singleblock"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singleblock timeout: %s", err))
	}
	sts.StateDeltaRequestTimeout, err = time.ParseDuration(viper.GetString("statetransfer.timeout.singlestatedelta"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.singlestatedelta timeout: %s", err))
	}
	sts.StateSnapshotRequestTimeout, err = time.ParseDuration(viper.GetString("statetransfer.timeout.fullstate"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse statetransfer.timeout.fullstate timeout: %s", err))
	}

	sts.maxStateDeltas = viper.GetInt("statetransfer.maxdeltas")
	if sts.maxStateDeltas <= 0 {
		panic(fmt.Errorf("sts.maxdeltas must be greater than 0"))
	}

	tmp := viper.GetInt("peer.sync.blocks.channelSize")
	if tmp <= 0 {
		panic(fmt.Errorf("peer.sync.blocks.channelSize must be greater than 0"))
	}
	sts.maxBlockRange = uint64(tmp)

	tmp = viper.GetInt("peer.sync.state.deltas.channelSize")
	if tmp <= 0 {
		panic(fmt.Errorf("peer.sync.state.deltas.channelSize must be greater than 0"))
	}
	sts.maxStateDeltaRange = uint64(tmp)

	return sts
}

// NewStateTransferState constructs a new state transfer state, including its maintenance threads
func NewStateTransferState(stack PartialStack) *StateTransferState {
	sts := threadlessNewStateTransferState(stack)

	go sts.stateThread()
	go sts.blockThread()

	return sts
}

// =============================================================================
// custom interfaces and structure definitions
// =============================================================================

type stateTransferUpdate int

const (
	initiated stateTransferUpdate = iota
	errored
	completed
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
		return a[i].lowBlock < a[j].lowBlock
	}
	return a[i].highBlock > a[j].highBlock
}

// =============================================================================
// helper functions for state transfer
// =============================================================================

// Executes a func trying each peer included in peerIDs until successful
// Attempts to execute over all peers if peerIDs is nil
func (sts *StateTransferState) tryOverPeers(passedPeerIDs []*protos.PeerID, do func(peerID *protos.PeerID) error) (err error) {

	peerIDs := passedPeerIDs

	if nil == passedPeerIDs {
		logger.Debug("%v tryOverPeers, no peerIDs given, discovering", sts.id)

		peersMsg, err := sts.stack.GetPeers()
		if err != nil {
			return fmt.Errorf("Couldn't retrieve list of peers: %v", err)
		}
		peers := peersMsg.GetPeers()
		for _, endpoint := range peers {
			if endpoint.Type == protos.PeerEndpoint_VALIDATOR {
				if endpoint.ID.Name == sts.id.Name {
					continue
				}
				peerIDs = append(peerIDs, endpoint.ID)
			}
		}

		logger.Debug("%v discovered %d peerIDs", sts.id, len(peerIDs))
	}

	logger.Debug("%v in tryOverPeers, using peerIDs: %v", sts.id, peerIDs)

	if 0 == len(peerIDs) {
		logger.Error("%v invoked tryOverPeers with no peers specified, throttling thread", sts.id)
		// Unless we throttle here, this condition will likely cause a tight loop which will adversely affect the rest of the system
		time.Sleep(sts.DiscoveryThrottleTime)
		return fmt.Errorf("No peers available to try over")
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
	logger.Debug("%v syncing blocks from %d to %d with head hash of %x", sts.id, highBlock, lowBlock, highHash)
	validBlockHash := highHash
	blockCursor := highBlock
	var block *protos.Block
	var goodRange *blockRange

	err := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {
		for {
			intermediateBlock := blockCursor + 1
			var blockChan <-chan *protos.SyncBlocks
			var err error
			for {

				if intermediateBlock == blockCursor+1 {
					if sts.maxBlockRange > blockCursor {
						// Don't underflow
						intermediateBlock = 0
					} else {
						intermediateBlock = blockCursor - sts.maxBlockRange
					}
					if intermediateBlock < lowBlock {
						intermediateBlock = lowBlock
					}
					logger.Debug("%v requesting block range from %d to %d", sts.id, blockCursor, intermediateBlock)
					blockChan, err = sts.GetRemoteBlocks(peerID, blockCursor, intermediateBlock)
				}

				if nil != err {
					logger.Warning("%v failed to get blocks from %d to %d from %v: %s",
						sts.id, blockCursor, lowBlock, peerID, err)
					return err
				}

				select {
				case syncBlockMessage, ok := <-blockChan:

					if !ok {
						return fmt.Errorf("Channel closed before we could finish reading")
					}

					if syncBlockMessage.Range.Start < syncBlockMessage.Range.End {
						// If the message is not replying with blocks backwards, we did not ask for it
						return fmt.Errorf("%v received a block with wrong (increasing) order from %v, aborting", sts.id, peerID)
					}

					var i int
					for i, block = range syncBlockMessage.Blocks {
						// It no longer correct to get duplication or out of range blocks, so we treat this as an error
						if syncBlockMessage.Range.Start-uint64(i) != blockCursor {
							return fmt.Errorf("%v received a block out of order, indicating a buffer overflow or other corruption: start=%d, end=%d, wanted %d", sts.id, syncBlockMessage.Range.Start, syncBlockMessage.Range.End, blockCursor)
						}

						testHash, err := sts.stack.HashBlock(block)
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
							if oldBlock, err := sts.stack.GetBlockByNumber(blockCursor); err == nil && oldBlock != nil {
								oldBlockHash, err := sts.stack.HashBlock(oldBlock)
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
								sts.stack.PutBlock(blockCursor, block)
							}
						} else {
							sts.stack.PutBlock(blockCursor, block)
						}

						goodRange = &blockRange{
							highBlock:   highBlock,
							lowBlock:    blockCursor,
							lowNextHash: block.PreviousBlockHash,
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
		}
	})

	if nil != block {
		logger.Debug("%v returned from sync with block %d and state hash %x", sts.id, blockCursor, block.StateHash)
	} else {
		logger.Debug("%v returned from sync with no new blocks", sts.id)
	}

	if goodRange != nil {
		goodRange.lowNextHash = block.PreviousBlockHash
		sts.validBlockRanges = append(sts.validBlockRanges, goodRange)
	}

	return blockCursor, block, err

}

func (sts *StateTransferState) syncBlockchainToCheckpoint(blockSyncReq *blockSyncReq) {

	logger.Debug("%v is processing a blockSyncReq to block %d", sts.id, blockSyncReq.blockNumber)

	blockchainSize := sts.stack.GetBlockchainSize()

	if blockSyncReq.blockNumber+1 < blockchainSize {
		if !sts.RecoverDamage {
			panic("The blockchain height is higher than advertised by consensus, the configuration has specified that bad blocks should not be deleted/overridden, so we cannot proceed")
		} else {
			// TODO For now, unimplemented because we have no way to delete blocks
			panic("Our blockchain is already higher than a sync target, this is unlikely, but unimplemented")
		}
	} else {

		_, _, err := sts.syncBlocks(blockSyncReq.blockNumber, blockSyncReq.reportOnBlock, blockSyncReq.firstBlockHash, blockSyncReq.peerIDs)

		if nil != blockSyncReq.replyChan {
			logger.Debug("%v replying to blockSyncReq on reply channel with : %s", sts.id, err)
			blockSyncReq.replyChan <- err
		}
	}
}

func (sts *StateTransferState) verifyAndRecoverBlockchain() bool {

	if 0 == len(sts.validBlockRanges) {
		size := sts.stack.GetBlockchainSize()
		if 0 == size {
			logger.Warning("%v has no blocks in its blockchain, including the genesis block", sts.id)
			return false
		}

		block, err := sts.stack.GetBlockByNumber(size - 1)
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

	logger.Debug("%v validating existing blockchain, highest validated block is %d, valid through %d", sts.id, sts.validBlockRanges[0].highBlock, lowBlock)

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
				block, err := sts.stack.GetBlockByNumber(lowBlock - 1) // Subtraction is safe here, lowBlock > 0
				if nil != err {
					logger.Warning("%v could not retrieve block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
				} else {
					if blockHash, err := sts.stack.HashBlock(block); nil != err {
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
			for j := 1; j < len(sts.validBlockRanges)-1; j++ {
				sts.validBlockRanges[j] = (sts.validBlockRanges)[j+1]
			}
			sts.validBlockRanges = sts.validBlockRanges[:len(sts.validBlockRanges)-1]
			logger.Debug("Deleted from validBlockRanges, new length %d", len(sts.validBlockRanges))
			return false
		}

		// Ranges are distinct and not adjacent
		targetBlock = sts.validBlockRanges[1].highBlock
	}

	if targetBlock+sts.blockVerifyChunkSize > lowBlock {
		// The sync range is small enough
	} else {
		// targetBlock >=0, targetBlock+blockVerifyChunkSize <= lowBlock --> lowBlock - blockVerifyChunkSize >= 0
		targetBlock = lowBlock - sts.blockVerifyChunkSize
	}

	badBlockNumber, err := sts.stack.VerifyBlockchain(lowBlock, targetBlock)

	if nil == err {
		logger.Debug("%v validated chain from %d to %d", sts.id, lowBlock, targetBlock)

		sts.validBlockRanges[0].lowBlock = targetBlock

		block, err := sts.stack.GetBlockByNumber(targetBlock)
		if nil != err {
			logger.Warning("%v could not retrieve block %d which it believed to be valid: %s", sts.id, lowBlock-1, err)
			return false
		}

		sts.validBlockRanges[0].lowNextHash = block.PreviousBlockHash
		return false
	}

	_, _, err = sts.syncBlocks(badBlockNumber-1, targetBlock, lowNextHash, nil)

	// valid block range accounting now in syncBlocks

	return false
}

func (sts *StateTransferState) blockThread() {

	sts.blockThreadIdle = false
	for {

		select {
		case blockSyncReq := <-sts.blockSyncReq:
			sts.syncBlockchainToCheckpoint(blockSyncReq)
		case <-sts.threadExit:
			logger.Debug("Received request for block transfer thread to exit (1)")
			return
		default:
			// If there is no checkpoint to sync to, make sure the rest of the chain is valid
			if !sts.verifyAndRecoverBlockchain() {
				// There is more verification to be done, so loop
				continue
			}
		}

		logger.Debug("%v has validated its blockchain to the genesis block", sts.id)

		for {
			sts.blockThreadIdle = true
			select {
			// If we make it this far, the whole blockchain has been validated, so we only need to watch for checkpoint sync requests
			case blockSyncReq := <-sts.blockSyncReq:
				sts.blockThreadIdle = false
				logger.Debug("Block thread received request for block transfer thread to sync")
				sts.syncBlockchainToCheckpoint(blockSyncReq)
				break
			case sts.blockThreadIdleChan <- struct{}{}:
				logger.Debug("%v block thread reporting as idle to unblock someone", sts.id)
				continue
			case <-sts.threadExit:
				logger.Debug("Block thread received request for block transfer thread to exit (2)")
				sts.blockThreadIdle = true
				return
			}
		}

	}
}

func (sts *StateTransferState) attemptStateTransfer(currentStateBlockNumber *uint64, mark **blockHashReply, blockHReply **blockHashReply, blocksValid *bool) error {
	var err error

	if !sts.stateValid {
		// Our state is currently bad, so get a new one
		*currentStateBlockNumber, err = sts.syncStateSnapshot((*mark).blockNumber, (*mark).peerIDs)

		if nil != err {
			*mark = &blockHashReply{ // Let's try to just sync state from anyone, for any sequence number
				syncMark: syncMark{
					blockNumber: 0,
					peerIDs:     nil,
				},
			}
			return fmt.Errorf("%v could not retrieve state as recent as %d from any of specified peers", sts.id, (*mark).blockNumber)
		}

		logger.Debug("%v completed state transfer to block %d", sts.id, *currentStateBlockNumber)
	} else {
		*currentStateBlockNumber = sts.stack.GetBlockchainSize() - 1 // The block height is one more than the latest block number
	}

	// TODO, eventually we should allow lower block numbers and rewind transactions as needed
	if nil == *blockHReply || (*blockHReply).blockNumber < *currentStateBlockNumber {

		if nil == *blockHReply {
			logger.Debug("%v has no valid block hash to validate the blockchain with yet, waiting for a known valid block hash", sts.id)
		} else {
			logger.Debug("%v already has valid blocks through %d but needs to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
		}

	outer:
		for {
			select {
			case *blockHReply = <-sts.blockHashReceiver:
				if (*blockHReply).blockNumber < *currentStateBlockNumber {
					logger.Debug("%v received a block hash reply for block number %d, which is not high enough", sts.id, (*blockHReply).blockNumber)
				} else {
					break outer
				}
			case <-sts.threadExit:
				logger.Debug("Received request for state thread to exit while waiting for block hash")
				return fmt.Errorf("Interrupted with request to exit while in state transfer.")
			}
		}
		logger.Debug("%v received a block hash reply for block %d with sync sources %v", sts.id, (*blockHReply).blockNumber, (*blockHReply).syncMark.peerIDs)
		*blocksValid = false // We retrieved a new hash, we will need to sync to a new block

	}

	if !*blocksValid {
		(*mark) = *blockHReply // We now know of a more recent block hash

		blockReplyChannel := make(chan error)

		req := &blockSyncReq{
			syncMark:       (*mark).syncMark,
			reportOnBlock:  *currentStateBlockNumber,
			replyChan:      blockReplyChannel,
			firstBlockHash: (*blockHReply).blockHash,
		}

		select {
		case sts.blockSyncReq <- req:
		case <-sts.threadExit:
			logger.Debug("Received request for state thread to exit while waiting for block sync reply")
			return fmt.Errorf("Interrupted with request to exit while waiting for block sync reply.")
		}

		logger.Debug("%v state transfer thread waiting for block sync to complete", sts.id)
		err = <-blockReplyChannel // TODO, double check we can't get stuck here in shutdown
		logger.Debug("%v state transfer thread continuing", sts.id)

		if err != nil {
			return fmt.Errorf("%v could not retrieve all blocks as recent as %d as the block hash advertised", sts.id, (*mark).blockNumber)
		}

		*blocksValid = true
	} else {
		logger.Debug("%v already has valid blocks through %d necessary to validate the state for block %d", sts.id, (*blockHReply).blockNumber, *currentStateBlockNumber)
	}

	if *currentStateBlockNumber+uint64(sts.maxStateDeltas) < (*blockHReply).blockNumber {
		return fmt.Errorf("%v has a state for block %d which is too far out of date to play forward to block %d, max deltas are %d, invalidating",
			sts.id, *currentStateBlockNumber, (*blockHReply).blockNumber, sts.maxStateDeltas)
	}

	stateHash, err := sts.stack.GetCurrentStateHash()
	if nil != err {
		sts.stateValid = false
		return fmt.Errorf("%v could not compute its current state hash: %x", sts.id, err)

	}

	block, err := sts.stack.GetBlockByNumber(*currentStateBlockNumber)
	if nil != err {
		*blocksValid = false
		return fmt.Errorf("%v believed its state for block %d to be valid, but it could not retrieve it : %s", sts.id, *currentStateBlockNumber, err)
	}

	if !bytes.Equal(stateHash, block.StateHash) {
		if sts.stateValid {
			sts.stateValid = false
			return fmt.Errorf("%v believed its state for block %d to be valid, but its hash (%x) did not match the recovered blockchain's (%x)", sts.id, (*currentStateBlockNumber), stateHash, block.StateHash)
		}
		return fmt.Errorf("%v recovered to an incorrect state at block number %d, (%x %x) retrying", sts.id, *currentStateBlockNumber, stateHash, block.StateHash)
	}

	logger.Debug("%v state is now valid", sts.id)

	sts.stateValid = true

	if *currentStateBlockNumber < (*blockHReply).blockNumber {
		*currentStateBlockNumber, err = sts.playStateUpToBlockNumber(*currentStateBlockNumber+uint64(1), (*blockHReply).blockNumber, (*blockHReply).peerIDs)
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
		sts.stateThreadIdle = true
		select {
		// Wait for state sync to become necessary
		case mark := <-sts.initiateStateSync:
			sts.informListeners(mark.blockNumber, mark.blockHash, mark.peerIDs, mark.metadata, nil, initiated)
			sts.stateThreadIdle = false

			logger.Debug("%v is initiating state transfer", sts.id)

			var currentStateBlockNumber uint64
			var blockHReply *blockHashReply
			blocksValid := false

			for {
				if err := sts.attemptStateTransfer(&currentStateBlockNumber, &mark, &blockHReply, &blocksValid); err != nil {
					logger.Error("%s", err)
					sts.informListeners(0, nil, mark.peerIDs, nil, err, errored)
					select {
					case <-sts.threadExit:
						logger.Debug("Received request for state thread to exit, aborting state transfer")
						return
					default:
						// Do nothing

					}
					continue
				}

				break
			}

			logger.Debug("%v is completing state transfer", sts.id)

			sts.asynchronousTransferInProgress = false

			sts.informListeners(blockHReply.blockNumber, blockHReply.blockHash, blockHReply.peerIDs, blockHReply.metadata, nil, completed)
		case sts.stateThreadIdleChan <- struct{}{}:
			logger.Debug("%v state thread reporting as idle to unblock someone", sts.id)
			continue
		case <-sts.threadExit:
			logger.Debug("Received request for state thread to exit")
			sts.stateThreadIdle = true
			return
		}
	}
}

// blockUntilIdle makes a best effort to block until the state transfer is idle
// This is not an atomic operation, and no locking is performed
// so it is possible in rare cases that this may unblock prematurely
func (sts *StateTransferState) blockUntilIdle() {
	logger.Debug("%v caller requesting to block until idle", sts.id)
	for i := 0; i < 3; i++ {
		// The goal is that both threads are simulatenously idle
		// but checking idle-ness is not atomic, so checking 3 times
		// increases the likelihood that things are in a consistent state
		select {
		case <-sts.blockThreadIdleChan:
		case <-sts.threadExit: // In case the block thread is gone
			return
		}
		select {
		case <-sts.stateThreadIdleChan:
		case <-sts.threadExit: // In case the state thread is gone
			return
		}
	}
}

// isIdle determines whether state transfer is currently doing nothing
// Note, this is different from state transfer in progress, as for instance
// block recovery can be being performed in the background
func (sts *StateTransferState) isIdle() bool {
	return sts.stateThreadIdle && sts.blockThreadIdle
}

func (sts *StateTransferState) informListeners(blockNumber uint64, blockHash []byte, peerIDs []*protos.PeerID, metadata interface{}, err error, update stateTransferUpdate) {
	sts.stateTransferListenersLock.Lock()
	defer func() {
		sts.stateTransferListenersLock.Unlock()
	}()

	for _, listener := range sts.stateTransferListeners {
		switch update {
		case initiated:
			listener.Initiated(blockNumber, blockHash, peerIDs, metadata)
		case errored:
			listener.Errored(blockNumber, blockHash, peerIDs, metadata, err)
		case completed:
			listener.Completed(blockNumber, blockHash, peerIDs, metadata)
		}
	}
}

func (sts *StateTransferState) playStateUpToBlockNumber(fromBlockNumber, toBlockNumber uint64, peerIDs []*protos.PeerID) (uint64, error) {
	logger.Debug("%v attempting to play state forward from %v to block %d", sts.id, peerIDs, toBlockNumber)
	currentBlock := fromBlockNumber
	err := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {

		intermediateBlock := currentBlock - 1 // Underflow is okay here, as we immediately overflow, and assign
		var deltaMessages <-chan *protos.SyncStateDeltas
		for {

			if intermediateBlock+1 == currentBlock {
				intermediateBlock = currentBlock + sts.maxStateDeltaRange
				if intermediateBlock > toBlockNumber {
					intermediateBlock = toBlockNumber
				}
				logger.Debug("%v requesting state delta range from %d to %d", sts.id, currentBlock, intermediateBlock)
				var err error
				deltaMessages, err = sts.GetRemoteStateDeltas(peerID, currentBlock, intermediateBlock)

				if err != nil {
					return fmt.Errorf("%v received an error while trying to get the state deltas for blocks %d through %d from %v", sts.id, currentBlock, intermediateBlock, peerID)
				}
			}

			select {
			case deltaMessage, ok := <-deltaMessages:
				if !ok {
					return fmt.Errorf("%v was only able to recover to block number %d when desired to recover to %d", sts.id, currentBlock-1, toBlockNumber)
				}

				if deltaMessage.Range.Start != currentBlock || deltaMessage.Range.End < deltaMessage.Range.Start || deltaMessage.Range.End > toBlockNumber {
					return fmt.Errorf("%v received a state delta from %v either in the wrong order (backwards) or not next in sequence, aborting, start=%d, end=%d", sts.id, peerID, deltaMessage.Range.Start, deltaMessage.Range.End)
				}

				for _, delta := range deltaMessage.Deltas {
					umDelta := &statemgmt.StateDelta{}
					if err := umDelta.Unmarshal(delta); nil != err {
						return fmt.Errorf("%v received a corrupt state delta from %v : %s", sts.id, peerID, err)
					}
					sts.stack.ApplyStateDelta(deltaMessage, umDelta)
				}

				success := false

				testBlock, err := sts.stack.GetBlockByNumber(deltaMessage.Range.End)

				if nil != err {
					logger.Warning("%v could not retrieve block %d, though it should be present", sts.id, deltaMessage.Range.End)
				} else {

					stateHash, err := sts.stack.GetCurrentStateHash()
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
					if nil != sts.stack.RollbackStateDelta(deltaMessage) {
						sts.InvalidateState()
						return fmt.Errorf("%v played state forward according to %v, but the state hash did not match, failed to roll back, invalidated state", sts.id, peerID)
					}
					return fmt.Errorf("%v played state forward according to %v, but the state hash did not match, rolled back", sts.id, peerID)

				}

				if nil != sts.stack.CommitStateDelta(deltaMessage) {
					sts.InvalidateState()
					return fmt.Errorf("%v played state forward according to %v, hashes matched, but failed to commit, invalidated state", sts.id, peerID)
				}

				if currentBlock == toBlockNumber {
					return nil
				}
				currentBlock++

			case <-time.After(sts.StateDeltaRequestTimeout):
				logger.Warning("%v timed out during state delta recovery from %v", sts.id, peerID)
				return fmt.Errorf("%v timed out during state delta recovery from %v", sts.id, peerID)
			}
		}

	})
	return currentBlock, err
}

// This function will retrieve the current state from a peer.
// Note that no state verification can occur yet, we must wait for the next checkpoint, so it is important
// not to consider this state as valid
func (sts *StateTransferState) syncStateSnapshot(minBlockNumber uint64, peerIDs []*protos.PeerID) (uint64, error) {

	logger.Debug("%v attempting to retrieve state snapshot from recovery from %v", sts.id, peerIDs)

	currentStateBlock := uint64(0)

	ok := sts.tryOverPeers(peerIDs, func(peerID *protos.PeerID) error {
		logger.Debug("%v is initiating state recovery from %v", sts.id, peerID)

		if err := sts.stack.EmptyState(); nil != err {
			logger.Error("Could not empty the current state: %s", err)
		}

		stateChan, err := sts.GetRemoteStateSnapshot(peerID)

		if err != nil {
			return err
		}

		timer := time.NewTimer(sts.StateSnapshotRequestTimeout)
		counter := 0

		for {
			select {
			case piece, ok := <-stateChan:
				if !ok {
					return fmt.Errorf("%v had state snapshot channel close prematurely after %d deltas: %s", sts.id, counter, err)
				}
				if 0 == len(piece.Delta) {
					stateHash, err := sts.stack.GetCurrentStateHash()
					if nil != err {
						sts.stateValid = false
						return fmt.Errorf("%v could not compute its current state hash: %x", sts.id, err)

					}

					logger.Debug("%v received final piece of state snapshot from %v after %d deltas, now has hash %x", sts.id, peerID, counter, stateHash)
					return nil
				}
				umDelta := &statemgmt.StateDelta{}
				if err := umDelta.Unmarshal(piece.Delta); nil != err {
					return fmt.Errorf("%v received a corrupt delta from %v after %d deltas : %s", sts.id, peerID, counter, err)
				}
				sts.stack.ApplyStateDelta(piece, umDelta)
				currentStateBlock = piece.BlockNumber
				if err := sts.stack.CommitStateDelta(piece); nil != err {
					return fmt.Errorf("%v could not commit state delta from %v after %d deltas: %s", sts.id, counter, peerID, err)
				}
				counter++
			case <-timer.C:
				return fmt.Errorf("%v timed out during state recovery from %v", sts.id, peerID)
			}
		}

	})

	return currentStateBlock, ok
}

// The below were stolen from helper.go, they should eventually be removed there, and probably made private here

// GetRemoteBlocks will return a channel to stream blocks from the desired replicaID
func (sts *StateTransferState) GetRemoteBlocks(replicaID *protos.PeerID, start, finish uint64) (<-chan *protos.SyncBlocks, error) {
	remoteLedger, err := sts.stack.GetRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestBlocks(&protos.SyncBlockRange{
		Start: start,
		End:   finish,
	})
}

// GetRemoteStateSnapshot will return a channel to stream a state snapshot from the desired replicaID
func (sts *StateTransferState) GetRemoteStateSnapshot(replicaID *protos.PeerID) (<-chan *protos.SyncStateSnapshot, error) {
	remoteLedger, err := sts.stack.GetRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateSnapshot()
}

// GetRemoteStateDeltas will return a channel to stream a state snapshot deltas from the desired replicaID
func (sts *StateTransferState) GetRemoteStateDeltas(replicaID *protos.PeerID, start, finish uint64) (<-chan *protos.SyncStateDeltas, error) {
	remoteLedger, err := sts.stack.GetRemoteLedger(replicaID)
	if nil != err {
		return nil, err
	}
	return remoteLedger.RequestStateDeltas(&protos.SyncBlockRange{
		Start: start,
		End:   finish,
	})
}
