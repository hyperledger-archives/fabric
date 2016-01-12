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
	"sort"
	"sync"
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/statetransfer"
	"github.com/openblockchain/obc-peer/openchain/util"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
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
// custom interfaces and structure definitions
// =============================================================================

type innerCPI interface {
	broadcast(msgPayload []byte)
	unicast(msgPayload []byte, receiverID uint64) (err error)
	verify(txRaw []byte) error
	execute(txRaw []byte, opts ...interface{})
	viewChange(curView uint64)
}

type pbftCore struct {
	// internal data
	lock     sync.Mutex
	closed   bool
	consumer innerCPI

	// PBFT data
	activeView   bool              // view change happening
	byzantine    bool              // whether this node is intentionally acting as Byzantine; useful for debugging on the testnet
	f            int               // number of faults we can tolerate
	h            uint64            // low watermark
	id           uint64            // replica ID; PBFT `i`
	K            uint64            // checkpoint period
	L            uint64            // log size
	lastExec     uint64            // last request we executed
	replicaCount int               // number of replicas; PBFT `|R|`
	seqNo        uint64            // PBFT "n", strictly monotonic increasing sequence number
	view         uint64            // current view
	chkpts       map[uint64]string // state checkpoints; map lastExec to global hash
	pset         map[uint64]*ViewChange_PQ
	qset         map[qidx]*ViewChange_PQ

	ledger  consensus.Ledger                  // Used for blockchain related queries
	hChkpts map[uint64]uint64                 // highest checkpoint sequence number observed for each replica
	sts     *statetransfer.StateTransferState // Data structure which handles state transfer

	newViewTimer       *time.Timer         // timeout triggering a view change
	timerActive        bool                // is the timer running?
	requestTimeout     time.Duration       // progress timeout for requests
	newViewTimeout     time.Duration       // progress timeout for new views
	lastNewViewTimeout time.Duration       // last timeout we used during this view change
	outstandingReqs    map[string]*Request // track whether we are waiting for requests to execute

	missingReqs map[string]bool // for all the assigned, non-checkpointed requests we might be missing during view-change

	// implementation of PBFT `in`
	reqStore        map[string]*Request   // track requests
	certStore       map[msgID]*msgCert    // track quorum certificates for requests
	checkpointStore map[Checkpoint]bool   // track checkpoints as set
	viewChangeStore map[vcidx]*ViewChange // track view-change messages
	newViewStore    map[uint64]*NewView   // track last new-view we received or sent
}

type qidx struct {
	d string
	n uint64
}

type msgID struct { // our index through certStore
	v uint64
	n uint64
}

type msgCert struct {
	prePrepare  *PrePrepare
	sentPrepare bool
	prepare     []*Prepare
	sentCommit  bool
	commit      []*Commit
}

type vcidx struct {
	v  uint64
	id uint64
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

// =============================================================================
// constructors
// =============================================================================

func newPbftCore(id uint64, config *viper.Viper, consumer innerCPI, ledger consensus.Ledger) *pbftCore {
	instance := &pbftCore{}
	instance.id = id
	instance.consumer = consumer
	instance.ledger = ledger

	// in dev/debugging mode you are expected to override the config values
	// with the environment variable OPENCHAIN_OBCPBFT_X_Y

	// read from the config file
	// you can override the config values with the environment variable prefix
	// OPENCHAIN_OBCPBFT, e.g. OPENCHAIN_OBCPBFT_BYZANTINE
	var err error
	instance.byzantine = config.GetBool("replica.byzantine")
	instance.f = config.GetInt("general.f")
	instance.K = uint64(config.GetInt("general.K"))
	instance.requestTimeout, err = time.ParseDuration(config.GetString("general.timeout.request"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse request timeout: %s", err))
	}
	instance.newViewTimeout, err = time.ParseDuration(config.GetString("general.timeout.viewchange"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse new view timeout: %s", err))
	}

	instance.activeView = true
	instance.L = 2 * instance.K // log size
	instance.replicaCount = 3*instance.f + 1

	// init the logs
	instance.certStore = make(map[msgID]*msgCert)
	instance.reqStore = make(map[string]*Request)
	instance.checkpointStore = make(map[Checkpoint]bool)
	instance.chkpts = make(map[uint64]string)
	instance.viewChangeStore = make(map[vcidx]*ViewChange)
	instance.pset = make(map[uint64]*ViewChange_PQ)
	instance.qset = make(map[qidx]*ViewChange_PQ)
	instance.newViewStore = make(map[uint64]*NewView)

	// initialize state transfer
	instance.hChkpts = make(map[uint64]uint64)
	instance.sts = statetransfer.NewStateTransferState(fmt.Sprintf("Replica %d", instance.id), config, ledger)

	// load genesis checkpoint
	genesisBlock, err := instance.ledger.GetBlock(0)
	if err != nil {
		panic(fmt.Errorf("Cannot load genesis block: %s", err))
	}
	genesisHash, err := ledger.HashBlock(genesisBlock)
	if err != nil {
		panic(fmt.Errorf("Cannot hash genesis block: %s", err))
	}
	instance.chkpts[0] = base64.StdEncoding.EncodeToString(genesisHash)

	// create non-running timer XXX ugly
	instance.newViewTimer = time.NewTimer(100 * time.Hour)
	instance.newViewTimer.Stop()
	instance.lastNewViewTimeout = instance.newViewTimeout
	instance.outstandingReqs = make(map[string]*Request)
	instance.missingReqs = make(map[string]bool)

	go instance.timerHander()

	return instance
}

// tear down resources opened by newPbftCore
func (instance *pbftCore) close() {
	instance.lock.Lock()
	defer instance.lock.Unlock()
	instance.closed = true
	instance.newViewTimer.Reset(0)
}

// allow the view-change protocol to kick-off when the timer expires
func (instance *pbftCore) timerHander() {
	for {
		select {
		case <-instance.newViewTimer.C:
			instance.lock.Lock()
			if instance.closed {
				instance.lock.Unlock()
				return
			}
			logger.Info("Replica %d view change timeout expired", instance.id)
			instance.sendViewChange()
			instance.lock.Unlock()
		}
	}
}

// =============================================================================
// helper functions for PBFT
// =============================================================================

// Given a certain view n, what is the expected primary?
func (instance *pbftCore) primary(n uint64) uint64 {
	return n % uint64(instance.replicaCount)
}

// Is the sequence number between watermarks?
func (instance *pbftCore) inW(n uint64) bool {
	return n-instance.h > 0 && n-instance.h <= instance.L
}

// Is the view right? And is the sequence number between watermarks?
func (instance *pbftCore) inWV(v uint64, n uint64) bool {
	return instance.view == v && instance.inW(n)
}

// Given a digest/view/seq, is there an entry in the certLog?
// If so, return it. If not, create it.
func (instance *pbftCore) getCert(v uint64, n uint64) (cert *msgCert) {
	idx := msgID{v, n}
	cert, ok := instance.certStore[idx]
	if ok {
		return
	}

	cert = &msgCert{}
	instance.certStore[idx] = cert
	return
}

// =============================================================================
// preprepare/prepare/commit quorum checks
// =============================================================================

func (instance *pbftCore) prePrepared(digest string, v uint64, n uint64) bool {
	_, mInLog := instance.reqStore[digest]

	if digest != "" && !mInLog {
		return false
	}

	if q, ok := instance.qset[qidx{digest, n}]; ok && q.View == v {
		return true
	}

	cert := instance.certStore[msgID{v, n}]
	if cert != nil {
		p := cert.prePrepare
		if p != nil && p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			return true
		}
	}
	logger.Debug("Replica %d does not have view=%d/seqNo=%d pre-prepared",
		instance.id, v, n)
	return false
}

func (instance *pbftCore) prepared(digest string, v uint64, n uint64) bool {
	if !instance.prePrepared(digest, v, n) {
		return false
	}

	if p, ok := instance.pset[n]; ok && p.View == v && p.Digest == digest {
		return true
	}

	quorum := 0
	cert := instance.certStore[msgID{v, n}]
	if cert == nil {
		return false
	}

	for _, p := range cert.prepare {
		if p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			quorum++
		}
	}

	logger.Debug("Replica %d prepare count for view=%d/seqNo=%d: %d",
		instance.id, v, n, quorum)

	return quorum >= 2*instance.f
}

func (instance *pbftCore) committed(digest string, v uint64, n uint64) bool {
	if !instance.prepared(digest, v, n) {
		return false
	}

	quorum := 0
	cert := instance.certStore[msgID{v, n}]
	if cert == nil {
		return false
	}

	for _, p := range cert.commit {
		if p.View == v && p.SequenceNumber == n {
			quorum++
		}
	}

	logger.Debug("Replica %d commit count for view=%d/seqNo=%d: %d",
		instance.id, v, n, quorum)

	return quorum >= 2*instance.f+1
}

// =============================================================================
// receive methods
// =============================================================================

// handle new consensus requests
func (instance *pbftCore) request(msgPayload []byte) error {
	msg := &Message{&Message_Request{&Request{Payload: msgPayload}}}
	instance.lock.Lock()
	defer instance.lock.Unlock()
	instance.recvMsgSync(msg)

	return nil
}

// handle internal consensus messages
func (instance *pbftCore) receive(msgPayload []byte) error {
	msg := &Message{}
	err := proto.Unmarshal(msgPayload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	instance.lock.Lock()
	defer instance.lock.Unlock()
	instance.recvMsgSync(msg)

	return nil
}

func (instance *pbftCore) recvMsgSync(msg *Message) (err error) {
	/*
		if blockNumber, ok := sts.AsynchronousRecoveryJustCompleted() ; ok {
			block, err := instance.ledger.GetBlock(block)
			if err != nil {

			} else {
				instance.lastExec = block.
			}
		}
	*/

	if req := msg.GetRequest(); req != nil {
		err = instance.recvRequest(req)
	} else if preprep := msg.GetPrePrepare(); preprep != nil {
		err = instance.recvPrePrepare(preprep)
	} else if prep := msg.GetPrepare(); prep != nil {
		err = instance.recvPrepare(prep)
	} else if commit := msg.GetCommit(); commit != nil {
		err = instance.recvCommit(commit)
	} else if chkpt := msg.GetCheckpoint(); chkpt != nil {
		err = instance.recvCheckpoint(chkpt)
	} else if vc := msg.GetViewChange(); vc != nil {
		err = instance.recvViewChange(vc)
	} else if nv := msg.GetNewView(); nv != nil {
		err = instance.recvNewView(nv)
	} else if fr := msg.GetFetchRequest(); fr != nil {
		fmt.Printf("Debug: replica %v receive fetch-request\n", instance.id)
		err = instance.recvFetchRequest(fr)
	} else if req := msg.GetReturnRequest(); req != nil {
		fmt.Printf("Debug: replica %v receive return-request\n", instance.id)
		err = instance.recvReturnRequest(req)
	} else {
		err = fmt.Errorf("Invalid message: %v", msg)
		logger.Error("%s", err)
	}

	return
}

func (instance *pbftCore) recvRequest(req *Request) error {
	digest := hashReq(req)
	logger.Debug("Replica %d received request: %s", instance.id, digest)

	if err := instance.consumer.verify(req.Payload); err != nil {
		logger.Warning("Request %s did not verify: %s", digest, err)
		return err
	}

	instance.reqStore[digest] = req
	instance.outstandingReqs[digest] = req
	if !instance.timerActive {
		instance.startTimer(instance.requestTimeout)
	}

	if instance.primary(instance.view) == instance.id && instance.activeView { // if we're primary of current view
		n := instance.seqNo + 1
		haveOther := false

		for _, cert := range instance.certStore { // check for other PRE-PREPARE for same digest, but different seqNo
			if p := cert.prePrepare; p != nil {
				if p.View == instance.view && p.SequenceNumber != n && p.RequestDigest == digest {
					logger.Debug("Other pre-prepare found with same digest but different seqNo: %d instead of %d", p.SequenceNumber, n)
					haveOther = true
					break
				}
			}
		}

		if instance.inWV(instance.view, n) && !haveOther {
			logger.Debug("Primary %d broadcasting pre-prepare for view=%d/seqNo=%d and digest %s",
				instance.id, instance.view, n, digest)
			instance.seqNo = n
			preprep := &PrePrepare{
				View:           instance.view,
				SequenceNumber: instance.seqNo,
				RequestDigest:  digest,
				Request:        req,
				ReplicaId:      instance.id,
			}
			cert := instance.getCert(instance.view, n)
			cert.prePrepare = preprep

			return instance.innerBroadcast(&Message{&Message_PrePrepare{preprep}}, false)
		}
	}

	return nil
}

func (instance *pbftCore) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("Replica %d received pre-prepare from replica %d for view=%d/seqNo=%d",
		instance.id, preprep.ReplicaId, preprep.View, preprep.SequenceNumber)

	if !instance.activeView {
		return nil
	}

	if instance.primary(instance.view) != preprep.ReplicaId {
		logger.Warning("Pre-prepare from other than primary: got %d, should be %d", preprep.ReplicaId, instance.primary(instance.view))
		return nil
	}

	if !instance.inWV(preprep.View, preprep.SequenceNumber) {
		logger.Warning("Pre-prepare view different, or sequence number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low-mark %d", preprep.View, instance.primary(instance.view), preprep.SequenceNumber, instance.h)
		return nil
	}

	cert := instance.getCert(preprep.View, preprep.SequenceNumber)
	if cert.prePrepare != nil && cert.prePrepare.RequestDigest != preprep.RequestDigest {
		logger.Warning("Pre-prepare found for same view/seqNo but different digest: received %s, stored %s", preprep.RequestDigest, cert.prePrepare.RequestDigest)
	} else {
		cert.prePrepare = preprep
	}

	// Store the request if, for whatever reason, haven't received it from an earlier broadcast.
	if _, ok := instance.reqStore[preprep.RequestDigest]; !ok {
		digest := hashReq(preprep.Request)
		if digest != preprep.RequestDigest {
			logger.Warning("Pre-prepare request and request digest do not match: request %s, digest %s",
				digest, preprep.RequestDigest)
			return nil
		}
		if err := instance.consumer.verify(preprep.Request.Payload); err != nil {
			logger.Warning("Request %s did not verify: %s", digest, err)
			return err
		}

		instance.reqStore[digest] = preprep.Request
		instance.outstandingReqs[digest] = preprep.Request
	}

	if instance.primary(instance.view) != instance.id && instance.prePrepared(preprep.RequestDigest, preprep.View, preprep.SequenceNumber) && !cert.sentPrepare {
		logger.Debug("Backup %d broadcasting prepare for view=%d/seqNo=%d",
			instance.id, preprep.View, preprep.SequenceNumber)

		prep := &Prepare{
			View:           preprep.View,
			SequenceNumber: preprep.SequenceNumber,
			RequestDigest:  preprep.RequestDigest,
			ReplicaId:      instance.id,
		}

		// TODO build this properly
		// https://github.com/openblockchain/obc-peer/issues/217
		if instance.byzantine {
			prep.RequestDigest = "foo"
		}

		cert.sentPrepare = true
		return instance.innerBroadcast(&Message{&Message_Prepare{prep}}, true)
	}

	return nil
}

func (instance *pbftCore) recvPrepare(prep *Prepare) error {
	logger.Debug("Replica %d received prepare from replica %d for view=%d/seqNo=%d",
		instance.id, prep.ReplicaId, prep.View, prep.SequenceNumber)

	if !(instance.primary(instance.view) != prep.ReplicaId && instance.inWV(prep.View, prep.SequenceNumber)) {
		logger.Warning("Ignoring invalid prepare")
		return nil
	}

	cert := instance.getCert(prep.View, prep.SequenceNumber)

	for _, prevPrep := range cert.prepare {
		if prevPrep.ReplicaId == prep.ReplicaId {
			logger.Warning("Ignoring duplicate prepare from %d", prep.ReplicaId)
			return nil
		}
	}
	cert.prepare = append(cert.prepare, prep)
	if instance.prepared(prep.RequestDigest, prep.View, prep.SequenceNumber) && !cert.sentCommit {
		logger.Debug("Replica %d broadcasting commit for view=%d/seqNo=%d",
			instance.id, prep.View, prep.SequenceNumber)

		commit := &Commit{
			View:           prep.View,
			SequenceNumber: prep.SequenceNumber,
			RequestDigest:  prep.RequestDigest,
			ReplicaId:      instance.id,
		}

		cert.sentCommit = true

		return instance.innerBroadcast(&Message{&Message_Commit{commit}}, true)
	}

	return nil
}

func (instance *pbftCore) recvCommit(commit *Commit) error {
	logger.Debug("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		instance.id, commit.ReplicaId, commit.View, commit.SequenceNumber)

	if instance.inWV(commit.View, commit.SequenceNumber) {
		cert := instance.getCert(commit.View, commit.SequenceNumber)
		for _, prevCommit := range cert.commit {
			if prevCommit.ReplicaId == commit.ReplicaId {
				logger.Warning("Ignoring duplicate commit from %d", commit.ReplicaId)
				return nil
			}
		}
		cert.commit = append(cert.commit, commit)

		// note that we can reach this point without
		// broadcasting a commit ourselves
		instance.executeOutstanding()
	} else {
		logger.Warning("Replica %d ignoring commit for view=%d/seqNo=%d: not in-wv",
			instance.id, commit.View, commit.SequenceNumber)
	}

	return nil
}

func (instance *pbftCore) executeOutstanding() error {
	// Do not attempt to execute requests while we know we are in a bad state
	if instance.sts.AsynchronousStateTransferInProgress() {
		return nil
	}

	for retry := true; retry; {
		retry = false
		for idx := range instance.certStore {
			if instance.executeOne(idx) {
				// range over the certStore again
				retry = true
				break
			}
		}
	}

	return nil
}

func (instance *pbftCore) executeOne(idx msgID) bool {
	cert := instance.certStore[idx]

	if idx.n != instance.lastExec+1 || cert == nil || cert.prePrepare == nil {
		return false
	}

	// we now have the right sequence number that doesn't create holes

	digest := cert.prePrepare.RequestDigest
	req := instance.reqStore[digest]

	if !instance.committed(digest, idx.v, idx.n) {
		return false
	}

	// we have a commit certificate for this request
	instance.lastExec = idx.n
	instance.stopTimer()
	instance.lastNewViewTimeout = instance.newViewTimeout

	// null request
	if digest == "" {
		logger.Info("Replica %d executing/committing null request for view=%d/seqNo=%d",
			instance.id, idx.v, idx.n)
	} else {
		logger.Info("Replica %d executing/committing request for view=%d/seqNo=%d and digest %s",
			instance.id, idx.v, idx.n, digest)

		instance.consumer.execute(req.Payload, idx.n)
		delete(instance.outstandingReqs, digest)
	}

	if len(instance.outstandingReqs) > 0 {
		instance.startTimer(instance.requestTimeout)
	}

	if instance.lastExec%instance.K == 0 {
		blockHeight, err := instance.ledger.GetBlockchainSize()
		if nil != err {
			panic("Could not determine block height, this is irrecoverable")
		}

		lastBlock, err := instance.ledger.GetBlock(blockHeight - 1)
		if nil != err {
			// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
			panic(fmt.Errorf("Just committed a block, but could not retrieve it : %s", err))
		}

		blockHashBytes, err := instance.ledger.HashBlock(lastBlock)

		if nil != err {
			// TODO this can maybe handled more gracefully, but seems likely to be irrecoverable
			panic(fmt.Errorf("Replica %d could not compute its own state hash, this indicates an irrecoverable situation: %s", instance.id, err))
		}

		blockHashAsString := base64.StdEncoding.EncodeToString(blockHashBytes)

		logger.Debug("Replica %d preparing checkpoint for view=%d/seqNo=%d and b64 block hash %s",
			instance.id, instance.view, instance.lastExec, blockHashAsString)

		chkpt := &Checkpoint{
			SequenceNumber: instance.lastExec,
			BlockHash:      blockHashAsString,
			ReplicaId:      instance.id,
			BlockNumber:    blockHeight - 1,
		}
		instance.chkpts[instance.lastExec] = chkpt.BlockHash
		instance.innerBroadcast(&Message{&Message_Checkpoint{chkpt}}, true)
	}

	return true
}

func (instance *pbftCore) moveWatermarks(h uint64) {
	for idx, cert := range instance.certStore {
		if idx.n <= h {
			logger.Debug("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				instance.id, idx.v, idx.n)
			delete(instance.reqStore, cert.prePrepare.RequestDigest)
			delete(instance.certStore, idx)
		}
	}

	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber <= h {
			logger.Debug("Replica %d cleaning checkpoint message from replica %d, seqNo %d, b64 block hash %s",
				instance.id, testChkpt.ReplicaId, testChkpt.SequenceNumber, testChkpt.BlockHash)
			delete(instance.checkpointStore, testChkpt)
		}
	}

	for n := range instance.pset {
		if n <= h {
			delete(instance.pset, n)
		}
	}

	for idx := range instance.qset {
		if idx.n <= h {
			delete(instance.qset, idx)
		}
	}

	for n := range instance.chkpts {
		if n < h {
			delete(instance.chkpts, n)
		}
	}

	instance.h = h

	logger.Debug("Replica %d updated low watermark to %d",
		instance.id, instance.h)
}

func (instance *pbftCore) witnessCheckpoint(chkpt *Checkpoint) {
	if instance.sts.AsynchronousStateTransferInProgress() {
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
			for replicaID, hChkpt := range instance.hChkpts {
				chkptSeqNumArray[index] = hChkpt
				index++
				if hChkpt < H {
					delete(instance.hChkpts, replicaID)
				}
			}
			sort.Sort(sortableUint64Slice(chkptSeqNumArray))

			// If f+1 nodes have issued checkpoints above our high water mark, then
			// we will never record 2f+1 checkpoints for that sequence number, we are out of date
			// (This is because all_replicas - missed - me = 3f+1 - f - 1 = 2f)
			if m := chkptSeqNumArray[len(instance.hChkpts)-(instance.f+1)]; m > H {
				logger.Warning("Replica %d is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", instance.id, chkpt.SequenceNumber, H)
				instance.moveWatermarks(m)

				furthestReplicaIds := make([]uint64, instance.f+1)
				i := 0
				for replicaID, hChkpt := range instance.hChkpts {
					if hChkpt >= m {
						furthestReplicaIds[i] = replicaID
						i++
					}
				}

				instance.sts.AsynchronousStateTransfer(m, furthestReplicaIds)
			}

			return
		}
	}

}

func (instance *pbftCore) witnessCheckpointWeakCert(chkpt *Checkpoint) {
	checkpointMembers := make([]uint64, instance.replicaCount)
	i := 0
	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.BlockHash == chkpt.BlockHash {
			checkpointMembers[i] = testChkpt.ReplicaId
			i++
		}
	}

	blockHashBytes, err := base64.StdEncoding.DecodeString(chkpt.BlockHash)
	if nil != err {
		logger.Error("Replica %d received a weak checkpoint cert for block %d which could not be decoded (%s)", instance.id, chkpt.BlockNumber, chkpt.BlockHash)
		return
	}
	logger.Debug("Replica %d witnessed a weak certificate for checkpoint %d, weak cert attested to by %d of %d", instance.id, chkpt.SequenceNumber, i, instance.replicaCount)
	instance.sts.AsynchronousStateTransferValidHash(chkpt.BlockNumber, blockHashBytes, checkpointMembers[0:i-1])
}

func (instance *pbftCore) recvCheckpoint(chkpt *Checkpoint) error {
	logger.Debug("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		instance.id, chkpt.ReplicaId, chkpt.SequenceNumber, chkpt.BlockHash)

	instance.witnessCheckpoint(chkpt) // State transfer tracking

	if !instance.inW(chkpt.SequenceNumber) {
		// If the instance is performing a state transfer, sequence numbers outside the watermarks is expected
		if !instance.sts.AsynchronousStateTransferInProgress() {
			logger.Warning("Checkpoint sequence number outside watermarks: seqNo %d, low-mark %d", chkpt.SequenceNumber, instance.h)
		}
		return nil
	}

	instance.checkpointStore[*chkpt] = true

	matching := 0
	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.BlockHash == chkpt.BlockHash {
			matching++
		}
	}
	logger.Debug("Replica %d found %d matching checkpoints for seqNo %d, digest %s, blocknumber %d",
		instance.id, matching, chkpt.SequenceNumber, chkpt.BlockHash, chkpt.BlockNumber)

	if instance.sts.AsynchronousStateTransferInProgress() && matching >= instance.f+1 {
		// We do have a weak cert
		instance.witnessCheckpointWeakCert(chkpt)
	}

	if matching <= instance.f*2 {
		// We do not have a quorum yet
		return nil
	}

	// It is actually just fine if we do not have this checkpoint
	// and should not trigger a state transfer
	// Imagine we are executing sequence number k-1 and we are slow for some reason
	// then everyone else finishes executing k, and we receive a checkpoint quorum
	// which we will agree with very shortly, but do not move our watermarks until
	// we have reached this checkpoint
	// Note, this is not divergent from the paper, as the paper requires that
	// the quorum certificate must contain 2f+1 messages, including its own
	if _, ok := instance.chkpts[chkpt.SequenceNumber]; !ok {
		logger.Debug("Replica %d found checkpoint quorum for seqNo %d, digest %s, but it has not reached this checkpoint itself yet",
			instance.id, chkpt.SequenceNumber, chkpt.BlockHash)
		return nil
	}

	logger.Debug("Replica %d found checkpoint quorum for seqNo %d, digest %s",
		instance.id, chkpt.SequenceNumber, chkpt.BlockHash)

	instance.moveWatermarks(chkpt.SequenceNumber)

	return instance.processNewView()
}

// used in view-change to fetch missing assigned, non-checkpointed requests
func (instance *pbftCore) fetchRequests() (err error) {
	var msg *Message
	for digest := range instance.missingReqs {
		msg = &Message{&Message_FetchRequest{&FetchRequest{
			RequestDigest: digest,
			ReplicaId:     instance.id,
		}}}
		instance.innerBroadcast(msg, false)
	}

	return
}

func (instance *pbftCore) recvFetchRequest(fr *FetchRequest) (err error) {
	fmt.Printf("Debug: replica %v recvFetchRequest\n", instance.id)
	digest := fr.RequestDigest
	if _, ok := instance.reqStore[digest]; !ok {
		return nil // we don't have it either
	}

	req := instance.reqStore[digest]
	msg := &Message{&Message_ReturnRequest{ReturnRequest: req}}
	msgPacked, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Error marshalling return-request message: %v", err)
	}

	receiver := fr.ReplicaId
	err = instance.consumer.unicast(msgPacked, receiver)

	return
}

func (instance *pbftCore) recvReturnRequest(req *Request) (err error) {
	digest := hashReq(req)
	fmt.Printf("Debug: replica %v recvReturnRequest (missing %d requests, receiving digest %v)\n", instance.id, len(instance.missingReqs), digest)
	if _, ok := instance.missingReqs[digest]; !ok {
		return nil // either the wrong digest, or we got it already from someone else
	}

	instance.reqStore[digest] = req
	delete(instance.missingReqs, digest)

	return instance.processNewView()
}

// =============================================================================
// Misc. methods go here
// =============================================================================

// Marshals a Message and hands it to the CPI. If toSelf is true,
// the message is also dispatched to the local instance's RecvMsgSync.
func (instance *pbftCore) innerBroadcast(msg *Message, toSelf bool) error {
	msgRaw, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("[innerBroadcast] Cannot marshal message: %s", err)
	}
	instance.consumer.broadcast(msgRaw)

	// We call ourselves synchronously, so that testing can run
	// synchronous.
	if toSelf {
		instance.recvMsgSync(msg)
	}
	return nil
}

func (instance *pbftCore) startTimer(timeout time.Duration) {
	instance.newViewTimer.Reset(timeout)
	logger.Debug("Replica %d starting new view timer for %s",
		instance.id, timeout)
	instance.timerActive = true
}

func (instance *pbftCore) stopTimer() {
	// remove timeouts that may have raced, to prevent additional view change
	instance.newViewTimer.Stop()
	logger.Debug("Replica %d stopping new view timer", instance.id)
	instance.timerActive = false
	if instance.closed {
		return
	}
	// XXX draining here does not help completely, because the
	// timer handler goroutine may already have consumed the
	// timeout and now is blocked on Lock()
loopNewView:
	for {
		select {
		case <-instance.newViewTimer.C:
		default:
			break loopNewView
		}
	}
}

func hashReq(req *Request) (digest string) {
	reqRaw, _ := proto.Marshal(req)
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(reqRaw))
}
