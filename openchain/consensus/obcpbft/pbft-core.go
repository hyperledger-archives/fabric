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
	"sync"
	"time"

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
	verify(txRaw []byte) error
	execute(txRaw []byte)
	viewChange(curView uint64)

	getStateHash(blockNumber ...uint64) (stateHash []byte, err error)
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

	newViewTimer       *time.Timer         // timeout triggering a view change
	timerActive        bool                // is the timer running?
	requestTimeout     time.Duration       // progress timeout for requests
	newViewTimeout     time.Duration       // progress timeout for new views
	lastNewViewTimeout time.Duration       // last timeout we used during this view change
	outstandingReqs    map[string]*Request // track whether we are waiting for requests to execute

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

// =============================================================================
// constructors
// =============================================================================

func newPbftCore(id uint64, config *viper.Viper, consumer innerCPI) *pbftCore {
	instance := &pbftCore{}
	instance.id = id
	instance.consumer = consumer

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

	// load genesis checkpoint
	stateHash, err := instance.consumer.getStateHash(0)
	if err != nil {
		panic(fmt.Errorf("Cannot load genesis block: %s", err))
	}
	instance.chkpts[0] = base64.StdEncoding.EncodeToString(stateHash)

	// create non-running timer XXX ugly
	instance.newViewTimer = time.NewTimer(100 * time.Hour)
	instance.newViewTimer.Stop()
	instance.lastNewViewTimeout = instance.newViewTimeout
	instance.outstandingReqs = make(map[string]*Request)

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
	} else {
		err = fmt.Errorf("Invalid message: %v", msg)
		logger.Error("%s", err)
	}

	return err
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

		instance.consumer.execute(req.Payload)
		delete(instance.outstandingReqs, digest)
	}

	if len(instance.outstandingReqs) > 0 {
		instance.startTimer(instance.requestTimeout)
	}

	if instance.lastExec%instance.K == 0 {
		stateHashBytes, _ := instance.consumer.getStateHash()
		stateHash := base64.StdEncoding.EncodeToString(stateHashBytes)

		logger.Debug("Replica %d preparing checkpoint for view=%d/seqNo=%d and state digest %s",
			instance.id, instance.view, instance.lastExec, stateHash)

		chkpt := &Checkpoint{
			SequenceNumber: instance.lastExec,
			StateDigest:    stateHash,
			ReplicaId:      instance.id,
		}
		instance.chkpts[instance.lastExec] = stateHash
		instance.innerBroadcast(&Message{&Message_Checkpoint{chkpt}}, true)
	}

	return true
}

func (instance *pbftCore) recvCheckpoint(chkpt *Checkpoint) error {
	logger.Debug("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		instance.id, chkpt.ReplicaId, chkpt.SequenceNumber, chkpt.StateDigest)

	if !instance.inW(chkpt.SequenceNumber) {
		logger.Warning("Checkpoint sequence number outside watermarks: seqNo %d, low-mark %d", chkpt.SequenceNumber, instance.h)
		return nil
	}

	instance.checkpointStore[*chkpt] = true

	quorum := 0
	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.StateDigest == chkpt.StateDigest {
			quorum++
		}
	}

	if quorum <= instance.f*2 {
		return nil
	}

	// If we do not have this checkpoint locally, we should not clear our state.
	// PBFT: This diverges from the paper.
	if _, ok := instance.chkpts[chkpt.SequenceNumber]; !ok {
		// XXX fetch checkpoint from other replica
		return nil
	}

	logger.Debug("Replica %d found checkpoint quorum for seqNo %d, digest %s",
		instance.id, chkpt.SequenceNumber, chkpt.StateDigest)

	for idx, cert := range instance.certStore {
		if idx.n <= chkpt.SequenceNumber {
			logger.Debug("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				instance.id, idx.v, idx.n)
			delete(instance.reqStore, cert.prePrepare.RequestDigest)
			delete(instance.certStore, idx)
		}
	}

	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber <= chkpt.SequenceNumber {
			logger.Debug("Replica %d cleaning checkpoint message from replica %d, seqNo %d, state digest %s",
				instance.id, testChkpt.ReplicaId,
				testChkpt.SequenceNumber, testChkpt.StateDigest)
			delete(instance.checkpointStore, testChkpt)
		}
	}

	for n := range instance.pset {
		if n <= chkpt.SequenceNumber {
			delete(instance.pset, n)
		}
	}

	for idx := range instance.qset {
		if idx.n <= chkpt.SequenceNumber {
			delete(instance.qset, idx)
		}
	}

	instance.h = 0
	for n := range instance.chkpts {
		if n < chkpt.SequenceNumber {
			delete(instance.chkpts, n)
		} else {
			if instance.h == 0 || n < instance.h {
				instance.h = n
			}
		}
	}

	logger.Debug("Replica %d updated low watermark to %d",
		instance.id, instance.h)

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
