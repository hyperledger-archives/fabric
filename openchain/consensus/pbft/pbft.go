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

func init() {
	logger = logging.MustGetLogger("consensus/plugin")
}

// =============================================================================
// Custom structure definitions go here
// =============================================================================

// Plugin carries fields related to the consensus algorithm implementation.
type Plugin struct {
	// internal data
	config *viper.Viper  // link to the plugin config file
	cpi    consensus.CPI // link to the CPI

	// PBFT data
	activeView   bool   // view change happening
	f            int    // number of faults we can tolerate
	h            uint64 // low watermark
	id           uint64 // replica ID; PBFT `i`
	K            uint64 // checkpoint period
	L            uint64 // log size
	lastExec     uint64 // last request we executed
	replicaCount uint64 // number of replicas; PBFT `|R|`
	seqNo        uint64 // PBFT "n", strictly monotonic increasing sequence number
	view         uint64 // current view

	// Implementation of PBFT `in`
	certStore map[msgID]*msgCert  // track quorum certificates for requests
	reqStore  map[string]*Request // track requests
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

// =============================================================================
// Constructors go here
// =============================================================================

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
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// TODO Initialize the algorithm here
	// You may want to set the fields of `instance` using `instance.GetParam()`.

	// In dev/debugging mode you are expected to override the config value
	// with the environment variable OPENCHAIN_PBFT_REPLICA_ID
	replicaID, err := instance.getParam("replica.id")
	if err != nil {
		panic(fmt.Errorf("No ID assigned to the replica: %s", err))
	}
	id, err := strconv.ParseUint(replicaID, 10, 64)
	if err != nil {
		panic(fmt.Errorf("Cannot convert ID to int: %s", err))
	}
	if id < 0 {
		panic(fmt.Errorf("An invalid ID has been assigned to the replica: %s", err))
	}
	instance.id = id

	instance.K = 128
	instance.L = 1 * instance.K

	// init the logs
	instance.certStore = make(map[msgID]*msgCert)
	instance.reqStore = make(map[string]*Request)

	return instance
}

// =============================================================================
// Helper functions for PBFT go here
// =============================================================================

// Given a certain view n, what is the expected primary?
func (instance *Plugin) getPrimary(n uint64) uint64 {
	return n % instance.replicaCount
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
func (instance *Plugin) getCert(digest string, v uint64, n uint64) (cert *msgCert) {
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

	if !mInLog {
		return false
	}

	cert := instance.certStore[msgID{v, n}]
	if cert != nil {
		p := cert.prePrepare
		if p != nil && p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			return true
		}
	}
	logger.Debug("Replica %d does not have v:%d,s:%d pre-prepared",
		instance.id, v, n)
	return false
}

func (instance *Plugin) prepared(digest string, v uint64, n uint64) bool {
	if !instance.prePrepared(digest, v, n) {
		return false
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

	logger.Debug("Replica %d prepared v:%d,s:%d quorum %d",
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

	logger.Debug("Replica %d committed v:%d,s:%d quorum %d",
		instance.id, v, n, quorum)

	return quorum >= 2*instance.f+1
}

// =============================================================================
// Receive methods go here
// =============================================================================

// RecvMsg receives messages transmitted by CPI.Broadcast or CPI.Unicast.
func (instance *Plugin) RecvMsg(msgWrapped *pb.OpenchainMessage) error {
	if msgWrapped.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		return instance.Request(msgWrapped.Payload)
	}
	if msgWrapped.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", msgWrapped.Type)
	}

	msg := &Message{}
	err := proto.Unmarshal(msgWrapped.Payload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	if req := msg.GetRequest(); req != nil {
		err = instance.recvRequest(req)
	} else if preprep := msg.GetPrePrepare(); preprep != nil {
		err = instance.recvPrePrepare(preprep)
	} else if prep := msg.GetPrepare(); prep != nil {
		err = instance.recvPrepare(prep)
	} else if commit := msg.GetCommit(); commit != nil {
		err = instance.recvCommit(commit)
	} else {
		err := fmt.Errorf("Invalid message: %v", msgWrapped.Payload)
		logger.Error("%s", err)
	}

	return err
}

// Request is the main entry into the consensus plugin.
// txs will be passed to CPI.ExecTXs once consensus is reached.
func (instance *Plugin) Request(txs []byte) error {
	logger.Info("New consensus request received")
	req := &Request{Payload: txs}                                    // TODO assign "client" timestamp
	return instance.broadcast(&Message{&Message_Request{req}}, true) // route to ourselves as well
}

func (instance *Plugin) recvRequest(req *Request) error {
	digest := hashReq(req)
	logger.Debug("Replica %d received request: %s", instance.id, digest)

	// TODO test timestamp
	instance.reqStore[digest] = req

	n := instance.seqNo + 1

	if instance.getPrimary(instance.view) == instance.id { // if we're primary of current view
		// check for other PRE-PREPARE for same digest, but different seqNo
		haveOther := false
		for _, cert := range instance.certStore {
			if p := cert.prePrepare; p != nil {
				if p.View == instance.view && p.SequenceNumber != n && p.RequestDigest == digest {
					haveOther = true
					break
				}
			}
		}

		if instance.inWV(instance.view, n) && !haveOther {
			logger.Debug("Primary %d sending pre-prepare (v:%d,s:%d) for digest: %s",
				instance.id, instance.view, n, digest)
			instance.seqNo = n
			preprep := &PrePrepare{
				View:           instance.view,
				SequenceNumber: instance.seqNo,
				RequestDigest:  digest,
				Request:        req,
				ReplicaId:      instance.id,
			}
			cert := instance.getCert(digest, instance.view, n)
			cert.prePrepare = preprep

			return instance.broadcast(&Message{&Message_PrePrepare{preprep}}, false)
		}
	}

	return nil
}

func (instance *Plugin) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("Replica %d received pre-prepare from replica %d (v:%d,s:%d)",
		instance.id, preprep.ReplicaId, preprep.View,
		preprep.SequenceNumber)

	if instance.getPrimary(instance.view) != preprep.ReplicaId {
		logger.Warning("Pre-prepare from other than primary: got %d, should be %d", preprep.ReplicaId, instance.getPrimary(instance.view))
		return nil
	}

	if !instance.inWV(preprep.View, preprep.SequenceNumber) {
		logger.Warning("Pre-prepare sequence number outside watermarks: seqNo %d, low-mark %d", preprep.SequenceNumber, instance.h)
		return nil
	}

	var cert msgCert
	if cert, ok := instance.certStore[msgID{preprep.View, preprep.SequenceNumber}]; ok {
		if cert.prePrepare != nil && cert.prePrepare.RequestDigest != preprep.RequestDigest {
			logger.Warning("Pre-prepare found for same view/seqNo but different digest: recevied %s, stored %s", preprep.RequestDigest, cert.prePrepare.RequestDigest)
		}
	} else {
		cert := instance.getCert(preprep.RequestDigest, preprep.View, preprep.SequenceNumber)
		cert.prePrepare = preprep
	}

	// Store the request if, for whatever reason, we haven't received it
	// from an earlier broadcast.
	if _, ok := instance.reqStore[preprep.RequestDigest]; !ok {
		digest := hashReq(preprep.Request)
		if digest != preprep.RequestDigest {
			logger.Warning("Pre-prepare request and request digest do not match: request %s, digest %s",
				digest, preprep.RequestDigest)
			return nil
		}
		instance.reqStore[digest] = preprep.Request
	}

	if instance.getPrimary(instance.view) != instance.id && instance.prePrepared(preprep.RequestDigest, preprep.View, preprep.SequenceNumber) && !cert.sentPrepare {
		// TODO speculative execution: ExecTXs

		logger.Debug("Backup %d sending prepare (v:%d,s:%d)",
			instance.id, preprep.View, preprep.SequenceNumber)

		prep := &Prepare{
			View:           preprep.View,
			SequenceNumber: preprep.SequenceNumber,
			RequestDigest:  preprep.RequestDigest,
			ReplicaId:      instance.id,
			// GlobalHash:
			// TxErrors:
		}
		cert.prepare = append(cert.prepare, prep)
		cert.sentPrepare = true

		return instance.broadcast(&Message{&Message_Prepare{prep}}, false)
	}

	return nil
}

func (instance *Plugin) recvPrepare(prep *Prepare) error {
	logger.Debug("Replica %d received prepare from replica %d (v:%d,s:%d)",
		instance.id, prep.ReplicaId, prep.View,
		prep.SequenceNumber)

	if instance.getPrimary(instance.view) != prep.ReplicaId && instance.inWV(prep.View, prep.SequenceNumber) {
		cert := instance.getCert(prep.RequestDigest, prep.View, prep.SequenceNumber)
		cert.prepare = append(cert.prepare, prep)
	}
	cert := instance.certStore[msgID{prep.View, prep.SequenceNumber}]

	if instance.prepared(prep.RequestDigest, prep.View, prep.SequenceNumber) && !cert.sentCommit {
		logger.Debug("Replica %d sending commit (v:%d,s:%d)",
			instance.id, prep.View, prep.SequenceNumber)

		commit := &Commit{
			View:           prep.View,
			SequenceNumber: prep.SequenceNumber,
			RequestDigest:  prep.RequestDigest,
			ReplicaId:      instance.id,
		}
		cert.commit = append(cert.commit, commit)
		cert.sentCommit = true

		return instance.broadcast(&Message{&Message_Commit{commit}}, false)
	}

	return nil
}

func (instance *Plugin) recvCommit(commit *Commit) error {
	logger.Debug("Replica %d received commit from replica %d (v:%d,s:%d)",
		instance.id, commit.ReplicaId, commit.View,
		commit.SequenceNumber)

	if instance.inWV(commit.View, commit.SequenceNumber) {
		cert := instance.getCert(commit.RequestDigest, commit.View, commit.SequenceNumber)
		cert.commit = append(cert.commit, commit)

		instance.executeOutstanding()
	} else {
		logger.Warning("Replica %d ignoring commit (v:%d,s:%d): not in-wv",
			instance.id, commit.View, commit.SequenceNumber)
	}

	return nil
}

func (instance *Plugin) executeOutstanding() error {
	for retry := true; retry; {
		retry = false
		for idx, cert := range instance.certStore {
			if idx.n != instance.lastExec+1 || cert == nil || cert.prePrepare == nil {
				continue
			}

			// we now have the right sequence number that doesn't create holes

			digest := cert.prePrepare.RequestDigest
			req := instance.reqStore[digest]

			if !instance.committed(digest, idx.v, idx.n) {
				continue
			}

			// we have a commit certificate for this batch

			logger.Info("Replica %d executing/committing request (v:%d,s:%d): %s",
				instance.id, idx.v, idx.n, digest)
			instance.lastExec = idx.n

			tx := &pb.Transaction{}
			err := proto.Unmarshal(req.Payload, tx)
			if err == nil {
				instance.cpi.ExecTXs([]*pb.Transaction{tx})
			}

			retry = true
		}
	}

	return nil
}

// =============================================================================
// Misc. methods go here
// =============================================================================

// Marshals a Message and hands it to the CPI.  If toSelf is true,
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

	err = instance.cpi.Broadcast(msgWrapped)
	if err == nil && toSelf {
		err = instance.RecvMsg(msgWrapped)
	}
	return err
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
