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
// Init.
// =============================================================================

// Package-level logger.
var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("plugin")
}

// =============================================================================
// Custom structure definitions go here.
// =============================================================================

// Plugin carries fields related to the consensus algorithm.
type Plugin struct {
	cpi          consensus.CPI // The consensus programming interface
	config       *viper.Viper  // The link to the config file
	leader       bool          // Is this validating peer the current leader?
	id           uint64        // replica id, PBFT `i`
	replicaCount uint64        // number of replicas, PBFT `|R|`
	L            uint64        // log size
	f            int           // number of faults we can tolerate

	view       uint64 // current view
	activeView bool   // view change happening
	lastExec   uint64 // last request we executed
	seqno      uint64 // PBFT "n", strict monotonic increasing sequence number
	h          uint64 // low watermark

	// implementation of PBFT `in`
	certStore map[msgId]*msgCert  // track quorum certificates for requests
	reqStore  map[string]*Request // track requests
}

type msgId struct {
	v uint64
	n uint64
}

type msgCert struct {
	request     *Request
	prePrepare  *PrePrepare
	sentPrepare bool
	prepare     []*Prepare
	sentCommit  bool
	commit      []*Commit
}

// =============================================================================
// Constructors go here.
// =============================================================================

// New creates an implementation-specific structure that will be held in the
// consensus `helper` object. (See `controller` and `helper` packages for more.)
func New(c consensus.CPI) *Plugin {
	instance := &Plugin{}
	instance.cpi = c

	// TODO: Initialize the algorithm here.
	// You may want to set the fields of `instance` using `instance.GetParam()`.
	// e.g. instance.blockTimeOut = strconv.Atoi(instance.getParam("timeout.block"))

	instance.config = viper.New()

	// For environment variables.
	instance.config.SetEnvPrefix(configPrefix)
	instance.config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	instance.config.SetEnvKeyReplacer(replacer)

	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	instance.config.AddConfigPath("../pbft/") // For when you run a test from `controller`.
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// Create the data store for incoming messages.
	instance.certStore = make(map[msgId]*msgCert)
	instance.reqStore = make(map[string]*Request)

	instance.L = 128

	return instance
}

// =============================================================================
// Consenter interface implementation goes here.
// =============================================================================

// Request is the main entry into the consensus plugin.  `txs` will be
// passed to CPI.ExecTXs once consensus is reached.
func (instance *Plugin) Request(txs []byte) error {
	logger.Info("new consensus request received")

	reqMsg, err := convertToRequest(txs)
	if err != nil {
		return err
	}

	return instance.broadcast(reqMsg, true) // route to ourselves as well
}

// RecvMsg receives messages transmitted by CPI.Broadcast or CPI.Unicast.
func (instance *Plugin) RecvMsg(msgWrapped *pb.OpenchainMessage) error {
	if msgWrapped.Type == pb.OpenchainMessage_REQUEST {
		return instance.Request(msgWrapped.Payload)
	}
	if msgWrapped.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("unexpected message type %s", msgWrapped.Type)
	}

	msgRaw := msgWrapped.Payload

	msg := &Message{}
	err := proto.Unmarshal(msgRaw, msg)
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
		err := fmt.Errorf("invalid message: ", msgRaw)
		logger.Error("%s", err)
	}

	return err
}

func (instance *Plugin) primary(n uint64) uint64 {
	return n % instance.replicaCount
}

func (instance *Plugin) inW(n uint64) bool {
	return n-instance.h > 0 && n-instance.h <= instance.L
}

func (instance *Plugin) inWv(v uint64, n uint64) bool {
	return instance.view == v && instance.inW(n)
}

func (instance *Plugin) prePrepared(digest string, v uint64, n uint64) bool {
	_, mInLog := instance.reqStore[digest]

	if !mInLog {
		return false
	}

	cert := instance.certStore[msgId{v, n}]
	if cert != nil {
		p := cert.prePrepare
		if p != nil && p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			return true
		}
	}
	return false
}

func (instance *Plugin) prepared(digest string, v uint64, n uint64) bool {
	if !instance.prePrepared(digest, v, n) {
		return false
	}

	quorum := 0
	cert := instance.certStore[msgId{v, n}]
	if cert == nil {
		return false
	}

	for _, p := range cert.prepare {
		if p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			quorum += 1
		}
	}

	return quorum >= 2*instance.f
}

func (instance *Plugin) committed(digest string, v uint64, n uint64) bool {
	if !instance.prepared(digest, v, n) {
		return false
	}

	quorum := 0
	cert := instance.certStore[msgId{v, n}]
	if cert == nil {
		return false
	}

	for _, p := range cert.commit {
		if p.View == v && p.SequenceNumber == n {
			quorum += 1
		}
	}

	return quorum >= 2*instance.f+1
}

func (instance *Plugin) recvRequest(req *Request) error {
	digest := hashReq(req)
	logger.Debug("%d received request %s", instance.id, digest)

	// XXX test timestamp

	instance.reqStore[digest] = req

	n := instance.seqno + 1
	if instance.primary(instance.view) == instance.id {
		// check for other PRE-PREPARE for same digest, but different seqno
		haveOther := false
		for _, cert := range instance.certStore {
			if p := cert.prePrepare; p != nil {
				if p.View == instance.view && p.SequenceNumber != n && p.RequestDigest == digest {
					haveOther = true
					break
				}
			}
		}

		if instance.inWv(instance.view, n) && !haveOther {
			logger.Debug("primary %d sending pre-prepare %d/%d for %s",
				instance.id, instance.view, n, digest)

			instance.seqno = n
			preprep := &PrePrepare{
				View:           instance.view,
				SequenceNumber: n,
				RequestDigest:  digest,
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
	logger.Debug("%d received %d pre-prepare %d/%d %s",
		instance.id, preprep.ReplicaId, preprep.View,
		preprep.SequenceNumber, preprep.RequestDigest)

	if instance.primary(instance.view) != preprep.ReplicaId {
		logger.Warning("pre-prepare from other than primary: %d should be %d", preprep.ReplicaId, instance.primary(instance.view))
		return nil
	}
	if !instance.inWv(preprep.View, preprep.SequenceNumber) {
		logger.Warning("pre-prepare seqno outside watermarks: seqno %d, low-mark %d", preprep.SequenceNumber, instance.h)
		return nil
	}
	var cert msgCert
	if cert, ok := instance.certStore[msgId{preprep.View, preprep.SequenceNumber}]; ok {
		if cert.prePrepare != nil && cert.prePrepare.RequestDigest != preprep.RequestDigest {
			logger.Warning("pre-prepare for same v,n but different digest: recv %s, stored %s", preprep.RequestDigest, cert.prePrepare.RequestDigest)
		}
	} else {
		cert := instance.getCert(preprep.RequestDigest, preprep.View, preprep.SequenceNumber)
		cert.prePrepare = preprep
	}
	if cert.sentPrepare {
		return nil
	}

	// XXX speculative execution: ExecTXs

	if instance.primary(instance.view) != instance.id {
		logger.Debug("backup %d sending prepare %d/%d for %s",
			instance.id, preprep.View, preprep.SequenceNumber, preprep.RequestDigest)

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
	logger.Debug("%d received %d prepare %d/%d %s",
		instance.id, prep.ReplicaId, prep.View,
		prep.SequenceNumber, prep.RequestDigest)

	if instance.primary(instance.view) != prep.ReplicaId && instance.inWv(prep.View, prep.SequenceNumber) {
		cert := instance.getCert(prep.RequestDigest, prep.View, prep.SequenceNumber)
		cert.prepare = append(cert.prepare, prep)
	}
	cert := instance.certStore[msgId{prep.View, prep.SequenceNumber}]

	if instance.prepared(prep.RequestDigest, prep.View, prep.SequenceNumber) && !cert.sentCommit {
		logger.Debug("replica %d sending commit %d/%d for %s",
			instance.id, prep.View, prep.SequenceNumber, prep.RequestDigest)

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
	logger.Debug("%d received %d commit %d/%d %s",
		instance.id, commit.ReplicaId, commit.View,
		commit.SequenceNumber, commit.RequestDigest)

	if instance.inWv(commit.View, commit.SequenceNumber) {
		cert := instance.getCert(commit.RequestDigest, commit.View, commit.SequenceNumber)
		cert.commit = append(cert.commit, commit)

		instance.executeOutstanding()
	}

	return nil
}

func (instance *Plugin) executeOutstanding() error {
	for retry := true; retry; {
		retry = false
		for idx, cert := range instance.certStore {
			if idx.n != instance.lastExec+1 || cert == nil || cert.request == nil {
				continue
			}
			digest := hashReq(cert.request)

			if !instance.committed(digest, idx.v, idx.n) {
				continue
			}

			logger.Info("%d executing/committing transaction %d/%d %s",
				instance.id, idx.v, idx.n, digest)
			instance.lastExec = idx.n

			retry = true
		}
	}

	return nil
}

func (instance *Plugin) getCert(digest string, v uint64, n uint64) (cert *msgCert) {
	idx := msgId{v, n}

	cert, ok := instance.certStore[idx]
	if ok {
		return
	}

	cert = &msgCert{}
	instance.certStore[idx] = cert
	return
}

// =============================================================================
// Custom interface implementation goes here.
// =============================================================================

// broadcast marshals the Message and hands it to the CPI.  If toSelf
// is true, the message is also dispatched to the local instance's
// RecvMsg.
func (instance *Plugin) broadcast(msg *Message, toSelf bool) error {
	msgPacked, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("broadcast: could not marshal message: %s", err)
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

// Allows us to check whether a validating peer is the current leader.
func (instance *Plugin) isLeader() bool {
	return instance.leader
}

// Flags a validating peer as the leader. This is a temporary state.
func (instance *Plugin) setLeader(flag bool) bool {
	logger.Debug("Setting leader=%s.", flag)
	instance.leader = flag
	return instance.leader
}

// =============================================================================
// Misc. helper functions go here.
// =============================================================================

// Receives the payload of `OpenchainMessage_REQUEST`, turns it into a Request
func convertToRequest(txs []byte) (reqMsg *Message, err error) {
	txBatch := &pb.TransactionBlock{}
	err = proto.Unmarshal(txs, txBatch)
	if err != nil {
		err = fmt.Errorf("Error unmarshalling transaction payload: %s", err)
		return
	}

	numTx := len(txBatch.Transactions)
	logger.Debug("Unmarshaled payload, number of transactions it carries: %d", numTx)

	// XXX for now only handle single transactions
	if numTx != 1 {
		err = fmt.Errorf("request should carry 1 transaction instead of: %d", numTx)
		return
	}

	tx := txBatch.Transactions[0]

	txPacked, err := proto.Marshal(tx)
	if err != nil {
		err = fmt.Errorf("Error marshalling single transaction.")
		return
	}

	reqMsg = &Message{&Message_Request{&Request{
		Timestamp: tx.Timestamp,
		Payload:   txPacked,
	}}}

	return
}

func hashReq(req *Request) (digest string) {
	packedReq, _ := proto.Marshal(req)
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(packedReq))
}
