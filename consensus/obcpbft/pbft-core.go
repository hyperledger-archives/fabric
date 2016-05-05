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
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/hyperledger/fabric/consensus"
	_ "github.com/hyperledger/fabric/core" // Needed for logging format init
	"github.com/hyperledger/fabric/core/util"

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

const (
	// An ugly thing, we need to create timers, then stop them before they expire, so use a large timeout
	UnreasonableTimeout = 100 * time.Hour
)

// =============================================================================
// custom interfaces and structure definitions
// =============================================================================

// Unless otherwise noted, all methods consume the PBFT thread, and should therefore
// not rely on PBFT accomplishing any work while that thread is being held
type innerStack interface {
	broadcast(msgPayload []byte)
	unicast(msgPayload []byte, receiverID uint64) (err error)
	execute(seqNo uint64, txRaw []byte) // This is invoked on a separate thread
	getState() []byte
	getLastSeqNo() (uint64, error)
	skipTo(seqNo uint64, snapshotID []byte, peers []uint64)
	validate(txRaw []byte) error
	viewChange(curView uint64)

	sign(msg []byte) ([]byte, error)
	verify(senderID uint64, signature []byte, message []byte) error

	consensus.StatePersistor
}

// This structure handles is used for incoming PBFT bound messages
type pbftMessage struct {
	sender uint64
	msg    *Message
}

type checkpointMessage struct {
	seqNo uint64
	id    []byte
}

type pbftCore struct {
	// internal data
	internalLock     sync.Mutex
	executing        bool                    // signals that application is executing
	closed           chan struct{}           // informs the main thread to exit (never written to, only closed)
	incomingChan     chan *pbftMessage       // informs the main thread of new messages
	stateUpdateChan  chan *checkpointMessage // informs the main thread the state has updated (via state transfer)
	execCompleteChan chan struct{}           // informs the main thread an execution has finished

	idleChan   chan struct{} // Used to detect idleness for testing
	injectChan chan func()   // Used as a hack to inject work onto the PBFT thread, to be removed eventually

	consumer innerStack

	// PBFT data
	activeView    bool              // view change happening
	byzantine     bool              // whether this node is intentionally acting as Byzantine; useful for debugging on the testnet
	f             int               // max. number of faults we can tolerate
	N             int               // max.number of validators in the network
	h             uint64            // low watermark
	id            uint64            // replica ID; PBFT `i`
	K             uint64            // checkpoint period
	logMultiplier uint64            // use this value to calculate log size : k*logMultiplier
	L             uint64            // log size
	lastExec      uint64            // last request we executed
	replicaCount  int               // number of replicas; PBFT `|R|`
	seqNo         uint64            // PBFT "n", strictly monotonic increasing sequence number
	view          uint64            // current view
	chkpts        map[uint64]string // state checkpoints; map lastExec to global hash
	pset          map[uint64]*ViewChange_PQ
	qset          map[qidx]*ViewChange_PQ

	skipInProgress bool              // Set when we have detected a fall behind scenario until we pick a new starting point
	hChkpts        map[uint64]uint64 // highest checkpoint sequence number observed for each replica

	currentExec        *uint64             // currently executing request
	newViewTimer       *time.Timer         // timeout triggering a view change
	timerActive        bool                // is the timer running?
	requestTimeout     time.Duration       // progress timeout for requests
	newViewTimeout     time.Duration       // progress timeout for new views
	lastNewViewTimeout time.Duration       // last timeout we used during this view change
	outstandingReqs    map[string]*Request // track whether we are waiting for requests to execute
	newViewTimerReason string              // what triggered the timer

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
	digest      string
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

type stateTransferMetadata struct {
	sequenceNumber uint64
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

func newPbftCore(id uint64, config *viper.Viper, consumer innerStack) *pbftCore {
	var err error
	instance := &pbftCore{}
	instance.id = id
	instance.consumer = consumer
	instance.closed = make(chan struct{})
	instance.incomingChan = make(chan *pbftMessage)
	instance.stateUpdateChan = make(chan *checkpointMessage)
	instance.execCompleteChan = make(chan struct{})
	instance.idleChan = make(chan struct{})
	instance.injectChan = make(chan func())

	instance.N = config.GetInt("general.N")
	instance.f = config.GetInt("general.f")
	if instance.f*3+1 > instance.N {
		panic(fmt.Sprintf("need at least %d enough replicas to tolerate %d byzantine faults, but only %d replicas configured", instance.f*3+1, instance.f, instance.N))
	}

	instance.K = uint64(config.GetInt("general.K"))

	instance.logMultiplier = uint64(config.GetInt("general.logmultiplier"))
	if instance.logMultiplier < 2 {
		panic("Log multiplier must be greater than or equal to 2")
	}
	instance.L = instance.logMultiplier * instance.K // log size

	instance.byzantine = config.GetBool("general.byzantine")

	instance.requestTimeout, err = time.ParseDuration(config.GetString("general.timeout.request"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse request timeout: %s", err))
	}
	instance.newViewTimeout, err = time.ParseDuration(config.GetString("general.timeout.viewchange"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse new view timeout: %s", err))
	}

	instance.activeView = true
	instance.replicaCount = instance.N

	logger.Info("PBFT type = %T", instance.consumer)
	logger.Info("PBFT Max number of validating peers (N) = %v", instance.N)
	logger.Info("PBFT Max number of failing peers (f) = %v", instance.f)
	logger.Info("PBFT byzantine flag = %v", instance.byzantine)
	logger.Info("PBFT request timeout = %v", instance.requestTimeout)
	logger.Info("PBFT view change timeout = %v", instance.newViewTimeout)
	logger.Info("PBFT Checkpoint period (K) = %v", instance.K)
	logger.Info("PBFT Log multiplier = %v", instance.logMultiplier)
	logger.Info("PBFT log size (L) = %v", instance.L)

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

	instance.chkpts[0] = "XXX GENESIS"

	instance.lastNewViewTimeout = instance.newViewTimeout
	instance.outstandingReqs = make(map[string]*Request)
	instance.missingReqs = make(map[string]bool)

	instance.stopTimer()
	instance.restoreState()

	go instance.main()

	return instance
}

// close tears down resources opened by newPbftCore
func (instance *pbftCore) close() {
	select { // Prevent closing multiple times
	case <-instance.closed:
	default:
		close(instance.closed)
	}
	instance.stopTimer()
}

// allow the view-change protocol to kick-off when the timer expires
func (instance *pbftCore) main() {

	for {
		logger.Debug("Replica %d service thread looping", instance.id)

		select {
		case <-instance.closed:
			close(instance.idleChan)
			return
		case <-instance.newViewTimer.C:
			logger.Info("Replica %d view change timer expired, sending view change", instance.id)
			instance.sendViewChange()
		case msg := <-instance.incomingChan:
			logger.Debug("Replica %d received incoming message from %v", instance.id, msg.sender)
			instance.recvMsg(msg.msg, msg.sender)
		case update := <-instance.stateUpdateChan:
			seqNo := update.seqNo
			logger.Info("Replica %d application caught up via state transfer, lastExec now %d", instance.id, seqNo)
			// XXX create checkpoint
			instance.lastExec = seqNo
			instance.moveWatermarks(instance.lastExec) // XXX should be checkpoint, not lastExec
			instance.skipInProgress = false
			instance.executeOutstanding()
		case <-instance.execCompleteChan:
			instance.execDoneSync()
		case work := <-instance.injectChan:
			work() // Used to allow the caller to steal use of the main thread
		case instance.idleChan <- struct{}{}:
			// Used to detect when this thread is idle for testing
		}
	}
}

// Allows the caller to inject work onto the main thread
// This is useful when the caller wants to safely manipulate PBFT state
func (instance *pbftCore) inject(work func()) {
	instance.injectChan <- work
	<-instance.idleChan
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

// intersectionQuorum returns the number of replicas that have to
// agree to guarantee that at least one correct replica is shared by
// two intersection quora
func (instance *pbftCore) intersectionQuorum() int {
	return (instance.N + instance.f + 2) / 2
}

// allCorrectReplicasQuorum returns the number of correct replicas (N-f)
func (instance *pbftCore) allCorrectReplicasQuorum() int {
	return (instance.N - instance.f)
}

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

	return quorum >= instance.intersectionQuorum()-1
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

	return quorum >= instance.intersectionQuorum()
}

// =============================================================================
// receive methods
// =============================================================================

// handle new consensus requests
func (instance *pbftCore) request(msgPayload []byte, senderID uint64) error {
	msg := &Message{&Message_Request{&Request{Payload: msgPayload,
		ReplicaId: senderID}}}
	instance.incomingChan <- &pbftMessage{
		sender: senderID,
		msg:    msg,
	}
	return nil
}

// handle internal consensus messages
func (instance *pbftCore) receive(msgPayload []byte, senderID uint64) error {
	msg := &Message{}
	err := proto.Unmarshal(msgPayload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	instance.incomingChan <- &pbftMessage{
		msg:    msg,
		sender: senderID,
	}

	return nil
}

// stateUpdate is an event telling us that the application fast-forwarded its state
func (instance *pbftCore) stateUpdate(seqNo uint64, id []byte) {
	logger.Debug("Replica %d queueing message that it has caught up via state transfer", instance.id)
	instance.stateUpdateChan <- &checkpointMessage{
		seqNo: seqNo,
		id:    id,
	}
}

// TODO, this should not return an error
func (instance *pbftCore) recvMsgSync(msg *Message, senderID uint64) (err error) {
	instance.incomingChan <- &pbftMessage{
		msg:    msg,
		sender: senderID,
	}
	return nil
}

func (instance *pbftCore) recvMsg(msg *Message, senderID uint64) (err error) {

	if req := msg.GetRequest(); req != nil {
		if senderID != req.ReplicaId {
			err = fmt.Errorf("Sender ID included in request message (%v) doesn't match ID corresponding to the receiving stream (%v)", req.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvRequest(req)
	} else if preprep := msg.GetPrePrepare(); preprep != nil {
		if senderID != preprep.ReplicaId {
			err = fmt.Errorf("Sender ID included in pre-prepare message (%v) doesn't match ID corresponding to the receiving stream (%v)", preprep.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvPrePrepare(preprep)
	} else if prep := msg.GetPrepare(); prep != nil {
		if senderID != prep.ReplicaId {
			err = fmt.Errorf("Sender ID included in prepare message (%v) doesn't match ID corresponding to the receiving stream (%v)", prep.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvPrepare(prep)
	} else if commit := msg.GetCommit(); commit != nil {
		if senderID != commit.ReplicaId {
			err = fmt.Errorf("Sender ID included in commit message (%v) doesn't match ID corresponding to the receiving stream (%v)", commit.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvCommit(commit)
	} else if chkpt := msg.GetCheckpoint(); chkpt != nil {
		if senderID != chkpt.ReplicaId {
			err = fmt.Errorf("Sender ID included in checkpoint message (%v) doesn't match ID corresponding to the receiving stream (%v)", chkpt.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvCheckpoint(chkpt)
	} else if vc := msg.GetViewChange(); vc != nil {
		if senderID != vc.ReplicaId {
			err = fmt.Errorf("Sender ID included in view-change message (%v) doesn't match ID corresponding to the receiving stream (%v)", vc.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvViewChange(vc)
	} else if nv := msg.GetNewView(); nv != nil {
		if senderID != nv.ReplicaId {
			err = fmt.Errorf("Sender ID included in new-view message (%v) doesn't match ID corresponding to the receiving stream (%v)", nv.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvNewView(nv)
	} else if fr := msg.GetFetchRequest(); fr != nil {
		if senderID != fr.ReplicaId {
			err = fmt.Errorf("Sender ID included in fetch-request message (%v) doesn't match ID corresponding to the receiving stream (%v)", fr.ReplicaId, senderID)
			logger.Warning(err.Error())
			return
		}
		err = instance.recvFetchRequest(fr)
	} else if req := msg.GetReturnRequest(); req != nil {
		// it's ok for sender ID and replica ID to differ; we're sending the original request message
		err = instance.recvReturnRequest(req)
	} else {
		err = fmt.Errorf("Invalid message: %v", msg)
		logger.Error(err.Error())
	}

	return
}

func (instance *pbftCore) recvRequest(req *Request) error {
	digest := hashReq(req)
	logger.Debug("Replica %d received request: %s", instance.id, digest)

	if err := instance.consumer.validate(req.Payload); err != nil {
		logger.Warning("Request %s did not verify: %s", digest, err)
		return err
	}

	instance.reqStore[digest] = req
	instance.outstandingReqs[digest] = req
	instance.persistRequest(digest)
	if !instance.timerActive && instance.activeView {
		instance.startTimer(instance.requestTimeout, fmt.Sprintf("new request %s", digest))
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

		// If we are the primary, have not already processed this request, and are within the first half of the log
		if instance.inWV(instance.view, n) && !haveOther && n <= instance.h+instance.L/2 {
			logger.Debug("Primary %d broadcasting pre-prepare for view=%d/seqNo=%d and digest %s",
				instance.id, instance.view, n, digest)
			instance.seqNo = n
			preprep := &PrePrepare{
				View:           instance.view,
				SequenceNumber: n,
				RequestDigest:  digest,
				Request:        req,
				ReplicaId:      instance.id,
			}
			cert := instance.getCert(instance.view, n)
			cert.prePrepare = preprep
			cert.digest = digest
			instance.persistQSet()

			instance.innerBroadcast(&Message{&Message_PrePrepare{preprep}})
			return instance.maybeSendCommit(digest, instance.view, n)
		}
	}

	return nil
}

func (instance *pbftCore) resubmitRequests() {
	if instance.primary(instance.view) != instance.id {
		return
	}

outer:
	for d, req := range instance.outstandingReqs {
		for _, cert := range instance.certStore {
			if cert.digest == d {
				continue outer
			}
		}

		// This is a request that has not been pre-prepared yet
		// Trigger request processing again.
		instance.recvRequest(req)
	}
}

func (instance *pbftCore) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("Replica %d received pre-prepare from replica %d for view=%d/seqNo=%d",
		instance.id, preprep.ReplicaId, preprep.View, preprep.SequenceNumber)

	if !instance.activeView {
		logger.Debug("Replica %d ignoring pre-prepare as we in a view change", instance.id)
		return nil
	}

	if instance.primary(instance.view) != preprep.ReplicaId {
		logger.Warning("Pre-prepare from other than primary: got %d, should be %d", preprep.ReplicaId, instance.primary(instance.view))
		return nil
	}

	if !instance.inWV(preprep.View, preprep.SequenceNumber) {
		if preprep.SequenceNumber != instance.h && !instance.skipInProgress {
			logger.Warning("Replica %d pre-prepare view different, or sequence number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low-mark %d", instance.id, preprep.View, instance.primary(instance.view), preprep.SequenceNumber, instance.h)
		} else {
			// This is perfectly normal
			logger.Debug("Replica %d pre-prepare view different, or sequence number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low-mark %d", instance.id, preprep.View, instance.primary(instance.view), preprep.SequenceNumber, instance.h)
		}

		return nil
	}

	cert := instance.getCert(preprep.View, preprep.SequenceNumber)
	if cert.digest != "" && cert.digest != preprep.RequestDigest {
		logger.Warning("Pre-prepare found for same view/seqNo but different digest: received %s, stored %s", preprep.RequestDigest, cert.digest)
		instance.sendViewChange()
	} else {
		cert.prePrepare = preprep
		cert.digest = preprep.RequestDigest
		instance.persistQSet()
	}

	// Store the request if, for whatever reason, haven't received it from an earlier broadcast.
	if _, ok := instance.reqStore[preprep.RequestDigest]; !ok {
		digest := hashReq(preprep.Request)
		if digest != preprep.RequestDigest {
			logger.Warning("Pre-prepare request and request digest do not match: request %s, digest %s",
				digest, preprep.RequestDigest)
			return nil
		}
		if err := instance.consumer.validate(preprep.Request.Payload); err != nil {
			logger.Warning("Request %s did not verify: %s", digest, err)
			return err
		}

		instance.reqStore[digest] = preprep.Request
		instance.outstandingReqs[digest] = preprep.Request
		instance.persistRequest(digest)
	}

	if !instance.timerActive {
		instance.startTimer(instance.requestTimeout, fmt.Sprintf("new pre-prepare for %s", preprep.RequestDigest))
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

		cert.sentPrepare = true
		instance.recvPrepare(prep)
		return instance.innerBroadcast(&Message{&Message_Prepare{prep}})
	}

	return nil
}

func (instance *pbftCore) recvPrepare(prep *Prepare) error {
	logger.Debug("Replica %d received prepare from replica %d for view=%d/seqNo=%d",
		instance.id, prep.ReplicaId, prep.View, prep.SequenceNumber)

	if instance.primary(prep.View) == prep.ReplicaId {
		logger.Warning("Replica %d received prepare from primary, ignoring", instance.id)
		return nil
	}

	if !instance.inWV(prep.View, prep.SequenceNumber) {
		if prep.SequenceNumber != instance.h && !instance.skipInProgress {
			logger.Warning("Replica %d ignoring prepare for view=%d/seqNo=%d: not in-wv, in view %d, low water mark %d", instance.id, prep.View, prep.SequenceNumber, instance.view, instance.h)
		} else {
			// This is perfectly normal
			logger.Debug("Replica %d ignoring prepare for view=%d/seqNo=%d: not in-wv, in view %d, low water mark %d", instance.id, prep.View, prep.SequenceNumber, instance.view, instance.h)
		}
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
	instance.persistPSet()

	return instance.maybeSendCommit(prep.RequestDigest, prep.View, prep.SequenceNumber)
}

func (instance *pbftCore) maybeSendCommit(digest string, v uint64, n uint64) error {
	cert := instance.getCert(v, n)

	if instance.prepared(digest, v, n) && !cert.sentCommit {
		logger.Debug("Replica %d broadcasting commit for view=%d/seqNo=%d",
			instance.id, v, n)

		commit := &Commit{
			View:           v,
			SequenceNumber: n,
			RequestDigest:  digest,
			ReplicaId:      instance.id,
		}

		cert.sentCommit = true

		instance.recvCommit(commit)
		return instance.innerBroadcast(&Message{&Message_Commit{commit}})
	}

	return nil
}

func (instance *pbftCore) recvCommit(commit *Commit) error {
	logger.Debug("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		instance.id, commit.ReplicaId, commit.View, commit.SequenceNumber)

	if !instance.inWV(commit.View, commit.SequenceNumber) {
		if commit.SequenceNumber != instance.h && !instance.skipInProgress {
			logger.Warning("Replica %d ignoring commit for view=%d/seqNo=%d: not in-wv, in view %d, high water mark %d", instance.id, commit.View, commit.SequenceNumber, instance.view, instance.h)
		} else {
			// This is perfectly normal
			logger.Debug("Replica %d ignoring commit for view=%d/seqNo=%d: not in-wv, in view %d, high water mark %d", instance.id, commit.View, commit.SequenceNumber, instance.view, instance.h)
		}
		return nil
	}

	cert := instance.getCert(commit.View, commit.SequenceNumber)
	for _, prevCommit := range cert.commit {
		if prevCommit.ReplicaId == commit.ReplicaId {
			logger.Warning("Ignoring duplicate commit from %d", commit.ReplicaId)
			return nil
		}
	}
	cert.commit = append(cert.commit, commit)

	if instance.committed(commit.RequestDigest, commit.View, commit.SequenceNumber) {
		instance.stopTimer()
		instance.lastNewViewTimeout = instance.newViewTimeout
		delete(instance.outstandingReqs, commit.RequestDigest)
		instance.startTimerIfOutstandingRequests()

		instance.executeOutstanding()
	}

	return nil
}

func (instance *pbftCore) executeOutstanding() {
	if instance.currentExec != nil {
		return
	}

	for idx := range instance.certStore {
		if instance.executeOne(idx) {
			break
		}
	}

	logger.Debug("replica %d certstore %+v", instance.id, instance.certStore)

	return
}

func (instance *pbftCore) executeOne(idx msgID) bool {
	cert := instance.certStore[idx]

	if idx.n != instance.lastExec+1 || cert == nil || cert.prePrepare == nil {
		return false
	}

	if instance.skipInProgress {
		logger.Debug("Replica %d currently picking a starting point to resume, will not execute", instance.id)
		return false
	}

	// we now have the right sequence number that doesn't create holes

	digest := cert.digest
	req := instance.reqStore[digest]

	if !instance.committed(digest, idx.v, idx.n) {
		return false
	}

	// we have a commit certificate for this request
	currentExec := idx.n
	instance.currentExec = &currentExec

	// null request
	if digest == "" {
		logger.Info("Replica %d executing/committing null request for view=%d/seqNo=%d",
			instance.id, idx.v, idx.n)
		instance.execDoneSync()
	} else {
		logger.Info("Replica %d executing/committing request for view=%d/seqNo=%d and digest %s",
			instance.id, idx.v, idx.n, digest)

		// asynchronously execute
		go func() {
			instance.consumer.execute(idx.n, req.Payload)
		}()
	}
	return true
}

func (instance *pbftCore) Checkpoint(seqNo uint64, id []byte) {
	if seqNo%instance.K != 0 {
		logger.Error("Attempted to checkpoint a sequence number (%d) which is not a multiple of the checkpoint interval (%d)", seqNo, instance.K)
		return
	}

	idAsString := base64.StdEncoding.EncodeToString(id)

	logger.Debug("Replica %d preparing checkpoint for view=%d/seqNo=%d and b64 id of %s",
		instance.id, instance.view, seqNo, idAsString)

	chkpt := &Checkpoint{
		SequenceNumber: seqNo,
		ReplicaId:      instance.id,
		Id:             idAsString,
	}
	instance.chkpts[seqNo] = idAsString

	instance.persistCheckpoint(seqNo, id)
	instance.recvCheckpoint(chkpt)
	instance.innerBroadcast(&Message{&Message_Checkpoint{chkpt}})
}

// execDone is an event telling us that the last execution has completed
func (instance *pbftCore) execDone() {
	instance.execCompleteChan <- struct{}{}
}

func (instance *pbftCore) execDoneSync() {
	logger.Info("Replica %d finished execution %d, trying next", instance.id, *instance.currentExec)

	instance.lastExec = *instance.currentExec
	instance.currentExec = nil

	if instance.lastExec%instance.K == 0 {
		instance.Checkpoint(instance.lastExec, instance.consumer.getState())
	}

	instance.executeOutstanding()
}

func (instance *pbftCore) moveWatermarks(n uint64) {
	// round down n to previous low watermark
	h := n / instance.K * instance.K

	for idx, cert := range instance.certStore {
		if idx.n <= h {
			logger.Debug("Replica %d cleaning quorum certificate for view=%d/seqNo=%d",
				instance.id, idx.v, idx.n)
			instance.persistDelRequest(cert.digest)
			delete(instance.reqStore, cert.digest)
			delete(instance.certStore, idx)
		}
	}

	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber <= h {
			logger.Debug("Replica %d cleaning checkpoint message from replica %d, seqNo %d, b64 snapshot id %s",
				instance.id, testChkpt.ReplicaId, testChkpt.SequenceNumber, testChkpt.Id)
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
			instance.persistDelCheckpoint(n)
		}
	}

	instance.h = h

	logger.Debug("Replica %d updated low watermark to %d",
		instance.id, instance.h)

	instance.resubmitRequests()
}

func (instance *pbftCore) weakCheckpointSetOutOfRange(chkpt *Checkpoint) bool {

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
			if m := chkptSeqNumArray[len(chkptSeqNumArray)-(instance.f+1)]; m > H {
				logger.Warning("Replica %d is out of date, f+1 nodes agree checkpoint with seqNo %d exists but our high water mark is %d", instance.id, chkpt.SequenceNumber, H)
				instance.reqStore = make(map[string]*Request) // Discard all our requests, as we will never know which were executed, to be addressed in #394
				instance.persistDelAllRequests()
				instance.moveWatermarks(m)
				instance.outstandingReqs = make(map[string]*Request)
				instance.skipInProgress = true
				instance.stopTimer()

				// TODO, reprocess the already gathered checkpoints, this will make recovery faster, though it is presently correct

				return true
			}
		}
	}

	return false
}

func (instance *pbftCore) witnessCheckpointWeakCert(chkpt *Checkpoint) {
	checkpointMembers := make([]uint64, instance.f+1) // Only ever invoked for the first weak cert, so guaranteed to be f+1
	i := 0
	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.Id == chkpt.Id {
			checkpointMembers[i] = testChkpt.ReplicaId
			logger.Debug("Replica %d adding replica %d (handle %v) to weak cert", instance.id, testChkpt.ReplicaId, checkpointMembers[i])
			i++
		}
	}

	snapshotID, err := base64.StdEncoding.DecodeString(chkpt.Id)
	if nil != err {
		err = fmt.Errorf("Replica %d received a weak checkpoint cert which could not be decoded (%s)", instance.id, chkpt.Id)
		logger.Error(err.Error())
		return
	}

	if instance.skipInProgress {
		logger.Debug("Replica %d is catching up and witnessed a weak certificate for checkpoint %d, weak cert attested to by %d of %d (%v)",
			instance.id, chkpt.SequenceNumber, i, instance.replicaCount, checkpointMembers)
		instance.moveWatermarks(chkpt.SequenceNumber)
		instance.lastExec = chkpt.SequenceNumber
		instance.activeView = true // TODO, verify this with experts
		instance.consumer.skipTo(chkpt.SequenceNumber, snapshotID, checkpointMembers)
	}
}

func (instance *pbftCore) recvCheckpoint(chkpt *Checkpoint) error {
	logger.Debug("Replica %d received checkpoint from replica %d, seqNo %d, digest %s",
		instance.id, chkpt.ReplicaId, chkpt.SequenceNumber, chkpt.Id)

	if instance.weakCheckpointSetOutOfRange(chkpt) {
		return nil
	}

	if !instance.inW(chkpt.SequenceNumber) {
		if chkpt.SequenceNumber != instance.h && !instance.skipInProgress {
			// It is perfectly normal that we receive checkpoints for the watermark we just raised, as we raise it after 2f+1, leaving f replies left
			logger.Warning("Checkpoint sequence number outside watermarks: seqNo %d, low-mark %d", chkpt.SequenceNumber, instance.h)
		} else {
			logger.Debug("Checkpoint sequence number outside watermarks: seqNo %d, low-mark %d", chkpt.SequenceNumber, instance.h)
		}
		return nil
	}

	instance.checkpointStore[*chkpt] = true

	matching := 0
	for testChkpt := range instance.checkpointStore {
		if testChkpt.SequenceNumber == chkpt.SequenceNumber && testChkpt.Id == chkpt.Id {
			matching++
		}
	}
	logger.Debug("Replica %d found %d matching checkpoints for seqNo %d, digest %s",
		instance.id, matching, chkpt.SequenceNumber, chkpt.Id)

	if matching == instance.f+1 {
		// We do have a weak cert
		instance.witnessCheckpointWeakCert(chkpt)
	}

	if matching < instance.intersectionQuorum() {
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
			instance.id, chkpt.SequenceNumber, chkpt.Id)
		return nil
	}

	logger.Debug("Replica %d found checkpoint quorum for seqNo %d, digest %s",
		instance.id, chkpt.SequenceNumber, chkpt.Id)

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
		instance.innerBroadcast(msg)
	}

	return
}

func (instance *pbftCore) recvFetchRequest(fr *FetchRequest) (err error) {
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
	if _, ok := instance.missingReqs[digest]; !ok {
		return nil // either the wrong digest, or we got it already from someone else
	}

	instance.reqStore[digest] = req
	delete(instance.missingReqs, digest)
	instance.persistRequest(digest)

	return instance.processNewView()
}

// =============================================================================
// Misc. methods go here
// =============================================================================

// Marshals a Message and hands it to the Stack. If toSelf is true,
// the message is also dispatched to the local instance's RecvMsgSync.
func (instance *pbftCore) innerBroadcast(msg *Message) error {
	msgRaw, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("[innerBroadcast] Cannot marshal message: %s", err)
	}

	doByzantine := false
	if instance.byzantine {
		rand1 := rand.New(rand.NewSource(time.Now().UnixNano()))
		doIt := rand1.Intn(3) // go byzantine about 1/3 of the time
		if doIt == 1 {
			doByzantine = true
		}
	}

	// testing byzantine fault.
	if doByzantine {
		rand2 := rand.New(rand.NewSource(time.Now().UnixNano()))
		ignoreidx := rand2.Intn(instance.N)
		for i := 0; i < instance.N; i++ {
			if i != ignoreidx && uint64(i) != instance.id { //Pick a random replica and do not send message
				instance.consumer.unicast(msgRaw, uint64(i))
			} else {
				logger.Debug("PBFT byzantine: not broadcasting to replica %v", i)
			}
		}
	} else {
		instance.consumer.broadcast(msgRaw)
	}
	return nil
}

func (instance *pbftCore) startTimerIfOutstandingRequests() {
	if instance.timerActive {
		return
	}

	if len(instance.outstandingReqs) > 0 {
		reqs := func() []string {
			var r []string
			for s := range instance.outstandingReqs {
				r = append(r, s)
			}
			return r
		}()
		instance.startTimer(instance.requestTimeout, fmt.Sprintf("outstanding requests %v", reqs))
	}
}

func (instance *pbftCore) startTimer(timeout time.Duration, reason string) {
	instance.newViewTimer = time.NewTimer(timeout) // Create a new timer, if it has already fired, it will still read on the channel
	logger.Debug("Replica %d starting new view timer for %s: %s", instance.id, timeout, reason)
	instance.newViewTimerReason = reason
	instance.timerActive = true
}

func (instance *pbftCore) stopTimer() {
	instance.newViewTimer = time.NewTimer(UnreasonableTimeout) // Create a new timer, then stop it, if it has already fired, it will still read on the channel
	instance.newViewTimer.Stop()
	logger.Debug("Replica %d stopping a running new view timer", instance.id)
	instance.timerActive = false
}

func hashReq(req *Request) (digest string) {
	reqRaw, _ := proto.Marshal(req)
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(reqRaw))
}
