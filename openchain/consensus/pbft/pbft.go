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
// Init
// =============================================================================

// Package-level logger.
var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("consensus/pbft")
}

// =============================================================================
// Custom structure definitions go here
// =============================================================================

// Plugin carries fields related to the consensus algorithm
type Plugin struct {
	activeView   bool          // View change happening
	cpi          consensus.CPI // Link to the CPI object
	config       *viper.Viper  // Link to the config file
	id           uint64        // Replica ID; PBFT `i`
	f            int           // Number of faults we can tolerate
	h            uint64        // Low watermark
	K            uint64        // Checkpoint period
	L            uint64        // Log size
	lastExec     uint64        // Last request we executed
	leader       bool          // Is this validating peer the current leader?
	replicaCount uint64        // Number of replicas; PBFT `|R|`
	seqno        uint64        // PBFT "n"; a strictly monotonic increasing sequence number
	view         uint64        // Current view

	// Implementation of PBFT `in`
	certStore map[msgID]*msgCert  // Track quorum certificates for requests
	reqStore  map[string]*Request // Track requests
}

type msgCert struct {
	request    *Request
	prePrepare *PrePrepare
	prepare    []*Prepare
	commit     []*Commit
}

type msgID struct {
	view  uint64
	seqno uint64
}

// =============================================================================
// Constructors go here.
// =============================================================================

// New creates an plugin-specific structure that will be held in the
// consensus `helper` object. (See `controller` and `helper` packages for more.)
func New(c consensus.CPI) *Plugin {
	instance := &Plugin{}
	instance.cpi = c

	// Setup the link to the config file.
	instance.config = viper.New()

	// For environment variables.
	instance.config.SetEnvPrefix(configPrefix)
	instance.config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	instance.config.SetEnvKeyReplacer(replacer)

	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Cannot read plugin config: %s", err))
	}

	// TODO: Initialize the algorithm here.
	// You may want to set the fields of `instance` using `instance.GetParam()`.
	// e.g. instance.blockTimeOut = strconv.Atoi(instance.getParam("timeout.block"))

	instance.K = 128
	instance.L = 1 * instance.K

	// Init the logs
	instance.certStore = make(map[msgID]*msgCert)
	instance.reqStore = make(map[string]*Request)

	return instance
}

// =============================================================================
// Helper functions for PBFT go here
// =============================================================================

// Is the sequence number between watermarks?
func (instance *Plugin) inW(n uint64) bool {
	return n-instance.h > 0 && n-instance.h <= instance.L
}

// Is the view right and is the sequence number between watermarks?
func (instance *Plugin) inWv(v uint64, n uint64) bool {
	return instance.view == v && instance.inW(n)
}

// Given a certain view, what is the expected primary?
func (instance *Plugin) primary(n uint64) uint64 {
	return n % instance.replicaCount
}

// =============================================================================
// Receive methods go here
// =============================================================================

// Request is the main entry into the consensus plugin.  `txs` will be
// passed to CPI.ExecTXs once consensus is reached.
func (instance *Plugin) Request(txsPacked []byte) error {
	logger.Info("New consensus request received")

	reqMsg, err := convertToRequest(txsPacked)
	if err != nil {
		return err
	}

	return instance.broadcast(reqMsg, true) // Route to ourselves as well
}

// RecvMsg receives messages transmitted by `CPI.Broadcast()` or `CPI.Unicast()`
func (instance *Plugin) RecvMsg(msg *pb.OpenchainMessage) error {
	if msg.Type == pb.OpenchainMessage_REQUEST {
		return instance.Request(msg.Payload)
	}
	if msg.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", msg.Type)
	}

	newMsg := &Message{}
	err := proto.Unmarshal(msg.Payload, newMsg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	// TODO Missing: PBFT consistency checks

	if req := newMsg.GetRequest(); req != nil {
		err = instance.recvRequest(msg.Payload, req)
	} else if preprep := newMsg.GetPrePrepare(); preprep != nil {
		err = instance.recvPrePrepare(preprep)
	} else if prep := newMsg.GetPrepare(); prep != nil {
		err = instance.recvPrepare(prep)
	} else if commit := newMsg.GetCommit(); commit != nil {
		err = instance.recvCommit(commit)
	} else {
		err := fmt.Errorf("Invalid message received: %v", msg.Payload)
		logger.Error("%s", err)
	}

	return err
}

func (instance *Plugin) recvRequest(msgPacked []byte, req *Request) error {
	logger.Debug("Request received")

	// TODO Test timestamp

	digest := hashMsg(msgPacked)
	instance.reqStore[digest] = req

	n := instance.seqno + 1

	if instance.primary(instance.view) == instance.id {
		// Check for other PRE-PREPARE for same digest, but different `seqno`
		haveOther := false
		for _, cert := range instance.certStore {
			if p := cert.prePrepare; p != nil {
				if p.View == instance.view && p.SequenceNumber != n && p.RequestDigest == digest {
					haveOther = true
					break
				}
			}
		}

		// if instance.inWv(instance.view, n) && !haveOther {
		if instance.inW(n) && !haveOther {
			logger.Debug("Primary sending pre-prepare %d for %s", n, digest)

			instance.seqno = n
			preprep := &PrePrepare{
				View:           instance.view,
				SequenceNumber: n,
				RequestDigest:  digest,
			}

			cert := instance.getCert(digest, instance.view, n)
			cert.prePrepare = preprep

			return instance.broadcast(&Message{&Message_PrePrepare{preprep}}, false)
		}
	}

	return nil
}

func (instance *Plugin) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("Pre-prepare received")

	if instance.leader {
		return nil
	}

	// TODO Speculative execution: ExecTXs

	logger.Debug("Backup sending prepare for %s", preprep.RequestDigest)

	prep := &Message{&Message_Prepare{&Prepare{
		// TODO Set view
		SequenceNumber: preprep.SequenceNumber,
		RequestDigest:  preprep.RequestDigest,
		// TODO Set GlobalHash
		// TODO Set ReplicaId
		// TODO Set TxErrors
	}}}

	return instance.broadcast(prep, false)
}

func (instance *Plugin) recvPrepare(prep *Prepare) error {
	logger.Debug("Prepare received")

	// TODO Wait for prepared certificate

	commit := &Message{&Message_Commit{&Commit{
		// TODO Set view
		SequenceNumber: prep.SequenceNumber,
		RequestDigest:  prep.RequestDigest,
		// TODO Set GlobalHash
		// TODO Set ReplicaId
		// TODO Set TxErrors
	}}}

	return instance.broadcast(commit, false)
}

func (instance *Plugin) recvCommit(commit *Commit) error {
	logger.Debug("Commit received")

	// TODO Wait for quorum certificate
	// TODO Commit transaction

	return nil
}

// =============================================================================
// Certificate checking goes here
// =============================================================================

func (instance *Plugin) prePrepared(digest string, v uint64, n uint64) bool {
	_, mInLog := instance.reqStore[digest]

	if !mInLog {
		return false
	}

	cert := instance.certStore[msgID{v, n}]
	if cert != nil {
		p := cert.prePrepare
		if p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
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
	cert := instance.certStore[msgID{v, n}]
	if cert == nil {
		return false
	}

	for _, p := range cert.prepare {
		if p.View == v && p.SequenceNumber == n && p.RequestDigest == digest {
			quorum++
		}
	}

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

	return quorum >= 2*instance.f+1
}

// =============================================================================
// Misc. methods go here
// =============================================================================

// Marshals a `Message` and hands it to the CPI. If `toSelf` is true, the
//  message is also dispatched to the local instance's `RecvMsg()`.
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

// Receives the payload of `OpenchainMessage_REQUEST`, turns it into a Request
func convertToRequest(txsPacked []byte) (reqMsg *Message, err error) {
	txs := &pb.TransactionBlock{}
	err = proto.Unmarshal(txsPacked, txs)
	if err != nil {
		err = fmt.Errorf("Cannot unmarshal payload of OpenchainMessage Request: %s", err)
		return
	}

	numTx := len(txs.Transactions)

	// TODO For now only handle single transactions
	if numTx != 1 {
		err = fmt.Errorf("request should carry 1 transaction instead of: %d", numTx)
		return
	}

	tx := txs.Transactions[0]

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

func (instance *Plugin) getCert(digest string, v uint64, n uint64) (cert *msgCert) {
	idx := msgID{v, n}

	cert, ok := instance.certStore[idx]
	if ok {
		return
	}

	cert = &msgCert{
		prepare: make([]*Prepare, 0),
		commit:  make([]*Commit, 0),
	}
	instance.certStore[idx] = cert
	return
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

// Returns the digest of a marshalled message (ATTN: returned value is a string)
func hashMsg(packedMsg []byte) (digest string) {
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(packedMsg))
}

// Allows us to check whether a validating peer is the current leader
func (instance *Plugin) isLeader() bool {
	return instance.leader
}

// Flags a validating peer as the leader. This is a temporary state.
func (instance *Plugin) setLeader(flag bool) bool {
	logger.Debug("Setting leader=%v", flag)
	instance.leader = flag
	return instance.leader
}
