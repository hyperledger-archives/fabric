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

package pbft

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// =============================================================================
// Constants
// =============================================================================

const configPrefix = "OPENCHAIN_PBFT"

// =============================================================================
// Init
// =============================================================================

var logger *logging.Logger // package-level logger
var pluginInstance *Plugin // the Plugin is a singleton

func init() {
	logger = logging.MustGetLogger("consensus/pbft")
}

// =============================================================================
// Custom structure definitions go here
// =============================================================================

// Plugin carries fields related to the consensus algorithm implementation.
type Plugin struct {
	// internal data
	config *viper.Viper  // link to the plugin config file
	cpi    consensus.CPI // link to the CPI
	c      chan *Message // serialization of incoming messages

	// PBFT data
	activeView   bool              // view change happening
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

	// Implementation of PBFT `in`
	certStore       map[msgID]*msgCert    // track quorum certificates for requests
	reqStore        map[string]*Request   // track requests
	checkpointStore map[Checkpoint]bool   // track checkpoints as set
	viewChangeStore map[vcidx]*ViewChange // track view-change messages
	lastNewView     NewView               // track last new-view we received or sent
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
// Constructors go here
// =============================================================================

// GetPlugin returns the handle to the Plugin singleton and updates the CPI if necessary.
func GetPlugin(c consensus.CPI) *Plugin {
	if pluginInstance == nil {
		pluginInstance = New(c)
	} else {
		pluginInstance.cpi = c // otherwise, just update the CPI
	}
	return pluginInstance
}

// New creates an plugin-specific structure that acts as the ConsenusHandler's consenter.
func New(c consensus.CPI) *Plugin {
	instance := &Plugin{}
	instance.cpi = c

	// setup the link to the config file
	instance.config = viper.New()

	// for environment variables
	instance.config.SetEnvPrefix(configPrefix)
	instance.config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	instance.config.SetEnvKeyReplacer(replacer)

	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	instance.config.AddConfigPath("./openchain/consensus/pbft/")
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// In dev/debugging mode you are expected to override the config values
	// with the environment variable OPENCHAIN_PBFT_X_Y

	// replica ID
	paramID, err := instance.getParam("replica.id")
	if err != nil {
		panic(fmt.Errorf("No ID assigned to the replica: %s", err))
	}
	instance.id, err = strconv.ParseUint(paramID, 10, 64)
	if err != nil {
		panic(fmt.Errorf("Cannot convert config ID to uint64: %s", err))
	}
	// byzantine nodes
	paramF, err := instance.getParam("general.f")
	if err != nil {
		panic(fmt.Errorf("No f defined in the config file: %s", err))
	}
	f, err := strconv.ParseUint(paramF, 10, 0)
	if err != nil {
		panic(fmt.Errorf("Cannot convert config f to uint64: %s", err))
	}
	instance.f = int(f)
	// replica count
	instance.replicaCount = 3*instance.f + 1
	// checkpoint period
	paramK, err := instance.getParam("general.K")
	if err != nil {
		panic(fmt.Errorf("Checkpoint period is not defined: %s", err))
	}
	instance.K, err = strconv.ParseUint(paramK, 10, 64)
	if err != nil {
		panic(fmt.Errorf("Cannot convert config checkpoint period to uint64: %s", err))
	}
	paramRequestTimeout, err := instance.getParam("general.timeout.request")
	if err != nil {
		panic(fmt.Errorf("No request timeout defined"))
	}
	instance.requestTimeout, err = time.ParseDuration(paramRequestTimeout)
	if err != nil {
		panic(fmt.Errorf("Cannot parse request timeout: %s", err))
	}
	paramNewViewTimeout, err := instance.getParam("general.timeout.request")
	if err != nil {
		panic(fmt.Errorf("No new view timeout defined"))
	}
	instance.newViewTimeout, err = time.ParseDuration(paramNewViewTimeout)
	if err != nil {
		panic(fmt.Errorf("Cannot parse new view timeout: %s", err))
	}

	// log size
	instance.L = 2 * instance.K

	instance.activeView = true

	// init the logs
	instance.certStore = make(map[msgID]*msgCert)
	instance.reqStore = make(map[string]*Request)
	instance.checkpointStore = make(map[Checkpoint]bool)
	instance.viewChangeStore = make(map[vcidx]*ViewChange)
	instance.chkpts = make(map[uint64]string)
	instance.pset = make(map[uint64]*ViewChange_PQ)
	instance.qset = make(map[qidx]*ViewChange_PQ)

	// initialize genesis checkpoint
	// TODO load state from disk
	instance.chkpts[0] = "TODO GENESIS STATE FROM STACK"

	// create non-running timer XXX ugly
	instance.newViewTimer = time.NewTimer(100 * time.Hour)
	instance.newViewTimer.Stop()
	instance.lastNewViewTimeout = instance.newViewTimeout
	instance.outstandingReqs = make(map[string]*Request)

	instance.c = make(chan *Message, 100)
	go instance.msgPump()

	return instance
}

// Close tears down resources opened by `New`.
func (instance *Plugin) Close() {
	close(instance.c)
}

// RecvMsg handles messages from both the stack, as well as internal
// consensus messages.  Consensus messages are serialized via
// instance.c and msgPump().
func (instance *Plugin) RecvMsg(msgWrapped *pb.OpenchainMessage) error {
	if msgWrapped.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		req := &Request{Payload: msgWrapped.Payload}
		msg := &Message{&Message_Request{req}}
		// We pass through the channel, instead of using
		// .broadcast(..., true), so that this msg will be
		// processed synchronously
		instance.c <- msg
		return instance.broadcast(msg, false)
	}
	if msgWrapped.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", msgWrapped.Type)
	}

	msg := &Message{}
	err := proto.Unmarshal(msgWrapped.Payload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}
	instance.c <- msg

	return nil
}

// msgPump takes messages queued by `RecvMsg`, and provides a
// sequential context to process those messages.
func (instance *Plugin) msgPump() {
	for {
		select {
		case msg, ok := <-instance.c:
			if !ok {
				return
			}
			_ = instance.recvMsgSync(msg)
		case <-instance.newViewTimer.C:
			logger.Info("Replica %d view change timeout expired", instance.id)
			instance.sendViewChange()
		}
	}
}

// =============================================================================
// Helper functions for PBFT go here
// =============================================================================

// Given a certain view n, what is the expected primary?
func (instance *Plugin) getPrimary(n uint64) uint64 {
	return n % uint64(instance.replicaCount)
}

// Is the sequence number between watermarks?
func (instance *Plugin) inW(n uint64) bool {
	return n-instance.h > 0 && n-instance.h <= instance.L
}

// Is the view right? And is the sequence number between watermarks?
func (instance *Plugin) inWV(v uint64, n uint64) bool {
	return instance.view == v && instance.inW(n)
}

// Given a digest/view/seq, is there an entry in the certLog?
// If so, return it. If not, create it.
func (instance *Plugin) getCert(v uint64, n uint64) (cert *msgCert) {
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
// Preprepare/prepare/commit quorum checks
// =============================================================================

func (instance *Plugin) prePrepared(digest string, v uint64, n uint64) bool {
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

func (instance *Plugin) prepared(digest string, v uint64, n uint64) bool {
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

func (instance *Plugin) committed(digest string, v uint64, n uint64) bool {
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
// Receive methods go here
// =============================================================================

// recvMsgSync processes messages in the synchronous context of msgPump().
func (instance *Plugin) recvMsgSync(msg *Message) (err error) {
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

func (instance *Plugin) recvRequest(req *Request) error {
	digest := hashReq(req)
	logger.Debug("Replica %d received request: %s", instance.id, digest)

	// TODO verify transaction
	// if err := instance.cpi.VerifyTransaction(...); err != nil {
	//   logger.Warning("Invalid request");
	//   return err
	// }
	instance.reqStore[digest] = req
	instance.outstandingReqs[digest] = req
	if !instance.timerActive {
		instance.startTimer(instance.requestTimeout)
	}

	n := instance.seqNo + 1

	if instance.getPrimary(instance.view) == instance.id && instance.activeView { // if we're primary of current view
		haveOther := false
		for _, cert := range instance.certStore { // check for other PRE-PREPARE for same digest, but different seqNo
			if p := cert.prePrepare; p != nil {
				if p.View == instance.view && p.SequenceNumber != n && p.RequestDigest == digest {
					logger.Debug("Other pre-prepared found with same digest but different seqNo: %d instead of %d", p.SequenceNumber, n)
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

			return instance.broadcast(&Message{&Message_PrePrepare{preprep}}, false)
		}
	}

	return nil
}

func (instance *Plugin) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("Replica %d received pre-prepare from replica %d for view=%d/seqNo=%d",
		instance.id, preprep.ReplicaId, preprep.View,
		preprep.SequenceNumber)

	if !instance.activeView {
		return nil
	}

	if instance.getPrimary(instance.view) != preprep.ReplicaId {
		logger.Warning("Pre-prepare from other than primary: got %d, should be %d", preprep.ReplicaId, instance.getPrimary(instance.view))
		return nil
	}

	if !instance.inWV(preprep.View, preprep.SequenceNumber) {
		logger.Warning("Pre-prepare view different, or sequence number outside watermarks: preprep.View %d, expected.View %d, seqNo %d, low-mark %d", preprep.View, instance.getPrimary(instance.view), preprep.SequenceNumber, instance.h)
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
		// TODO verify transaction
		// if err := instance.cpi.VerifyTransaction(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }
		instance.reqStore[digest] = preprep.Request
		instance.outstandingReqs[digest] = preprep.Request
	}

	if instance.getPrimary(instance.view) != instance.id && instance.prePrepared(preprep.RequestDigest, preprep.View, preprep.SequenceNumber) && !cert.sentPrepare {
		logger.Debug("Backup %d broadcasting prepare for view=%d/seqNo=%d",
			instance.id, preprep.View, preprep.SequenceNumber)

		prep := &Prepare{
			View:           preprep.View,
			SequenceNumber: preprep.SequenceNumber,
			RequestDigest:  preprep.RequestDigest,
			ReplicaId:      instance.id,
		}
		cert.prepare = append(cert.prepare, prep)
		cert.sentPrepare = true

		return instance.broadcast(&Message{&Message_Prepare{prep}}, false)
	}

	return nil
}

func (instance *Plugin) recvPrepare(prep *Prepare) error {
	logger.Debug("Replica %d received prepare from replica %d for view=%d/seqNo=%d",
		instance.id, prep.ReplicaId, prep.View,
		prep.SequenceNumber)

	if !(instance.getPrimary(instance.view) != prep.ReplicaId && instance.inWV(prep.View, prep.SequenceNumber)) {
		logger.Warning("Ignoring invalid prepare")
		return nil
	}

	cert := instance.getCert(prep.View, prep.SequenceNumber)
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

		return instance.broadcast(&Message{&Message_Commit{commit}}, true)
	}

	return nil
}

func (instance *Plugin) recvCommit(commit *Commit) error {
	logger.Debug("Replica %d received commit from replica %d for view=%d/seqNo=%d",
		instance.id, commit.ReplicaId, commit.View,
		commit.SequenceNumber)

	if instance.inWV(commit.View, commit.SequenceNumber) {
		cert := instance.getCert(commit.View, commit.SequenceNumber)
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

func (instance *Plugin) executeOutstanding() error {
	for retry := true; retry; {
		retry = false
		for idx := range instance.certStore {
			if instance.executeOne(idx) {
				retry = true
				break
			}
		}
	}

	return nil
}

func (instance *Plugin) executeOne(idx msgID) bool {
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

	// null request
	if digest == "" {
		logger.Info("Replica %d executing/committing null request for view=%d/seqNo=%d",
			instance.id, idx.v, idx.n)
	} else {
		logger.Info("Replica %d executing/committing request for view=%d/seqNo=%d and digest %s",
			instance.id, idx.v, idx.n, digest)

		tx := &pb.Transaction{}
		err := proto.Unmarshal(req.Payload, tx)
		if err == nil {
			instance.cpi.ExecTXs([]*pb.Transaction{tx})
		}

		delete(instance.outstandingReqs, digest)
	}

	instance.stopTimer()
	instance.lastNewViewTimeout = instance.newViewTimeout
	if len(instance.outstandingReqs) > 0 {
		instance.newViewTimer.Reset(instance.requestTimeout)
	}

	if instance.lastExec%instance.K == 0 {
		// XXX replace with instance.cpi.GetStateHash()
		stateHashBytes := []byte("XXX get current state hash")
		stateHash := base64.StdEncoding.EncodeToString(stateHashBytes)

		logger.Debug("Replica %d preparing checkpoint for view=%d/seqNo=%d and state digest %s",
			instance.id, instance.view, instance.lastExec, stateHash)

		chkpt := &Checkpoint{
			SequenceNumber: instance.lastExec,
			StateDigest:    stateHash,
			ReplicaId:      instance.id,
		}
		instance.chkpts[instance.lastExec] = stateHash
		instance.broadcast(&Message{&Message_Checkpoint{chkpt}}, true)
	}

	return true
}

func (instance *Plugin) recvCheckpoint(chkpt *Checkpoint) error {
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

	// If we do not have this checkpoint locally, we should not
	// clear our state.
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
// the message is also dispatched to the local instance's RecvMsg.
func (instance *Plugin) broadcast(msg *Message, toSelf bool) error {
	msgPacked, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("[broadcast] Cannot marshal message: %s", err)
	}

	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPacked,
	}

	if toSelf {
		err = instance.recvMsgSync(msg)
	}
	err = instance.cpi.Broadcast(msgWrapped)
	return err
}

func (instance *Plugin) startTimer(timeout time.Duration) {
	instance.newViewTimer.Reset(timeout)
	instance.timerActive = true
}

func (instance *Plugin) stopTimer() {
	// remove timeouts that may have raced, to prevent additional view change
	instance.newViewTimer.Stop()
	instance.timerActive = false
loop:
	for {
		select {
		case <-instance.newViewTimer.C:
		default:
			break loop
		}
	}
}

// A getter for the values listed in `config.yaml`.
func (instance *Plugin) getParam(param string) (val string, err error) {
	if ok := instance.config.IsSet(param); !ok {
		err := fmt.Errorf("Key %s does not exist in algo config", param)
		return "nil", err
	}
	val = instance.config.GetString(param)
	return val, nil
}

func hashReq(req *Request) (digest string) {
	packedReq, _ := proto.Marshal(req)
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(packedReq))
}
