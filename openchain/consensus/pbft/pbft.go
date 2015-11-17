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
	cpi            consensus.CPI       // The consensus programming interface
	config         *viper.Viper        // The link to the config file
	leader         bool                // Is this validating peer the current leader?
	sequenceNumber uint64              // PBFT "n", strict monotonic increasing sequence number
	certStore      map[msgId]*msgCert  // track quorum certificates for requests
	reqStore       map[string]*Request // track requests
}

type msgId struct {
	view           uint64
	sequenceNumber uint64
}

type msgCert struct {
	request    *Request
	prePrepare *PrePrepare
	prepare    []*Prepare
	commits    []*Commit
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

	// XXX missing: PBFT consistency checks

	if req := msg.GetRequest(); req != nil {
		err = instance.recvRequest(msgRaw, req)
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

func (instance *Plugin) recvRequest(msgRaw []byte, req *Request) error {
	logger.Debug("request received")
	// XXX test timestamp
	digest := hashMsg(msgRaw)
	instance.reqStore[digest] = req

	if !instance.leader {
		return nil
	}

	logger.Debug("leader sending pre-prepare for %s", digest)
	preprep := &Message{&Message_PrePrepare{&PrePrepare{
		// XXX view
		SequenceNumber: instance.sequenceNumber,
		RequestDigest:  digest,
	}}}

	return instance.broadcast(preprep, false)
}

func (instance *Plugin) recvPrePrepare(preprep *PrePrepare) error {
	logger.Debug("pre-prepare received")

	if instance.leader {
		return nil
	}

	// XXX speculative execution: ExecTXs

	logger.Debug("backup sending prepare for %s", preprep.RequestDigest)

	prep := &Message{&Message_Prepare{&Prepare{
		// XXX view
		SequenceNumber: preprep.SequenceNumber,
		RequestDigest:  preprep.RequestDigest,
		// XXX ReplicaId
	}}}

	return instance.broadcast(prep, false)
}

func (instance *Plugin) recvPrepare(prep *Prepare) error {
	logger.Debug("prepare received")

	commit := &Message{&Message_Commit{&Commit{
		// XXX view
		SequenceNumber: prep.SequenceNumber,
		RequestDigest:  prep.RequestDigest,
		// XXX ReplicaId
	}}}

	return instance.broadcast(commit, false)
}

func (instance *Plugin) recvCommit(commit *Commit) error {
	logger.Debug("commit received")

	// XXX wait for quorum certificate
	// XXX commit transaction
	return nil
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

// Calculate the digest of a marshalled message.
func hashMsg(packedMsg []byte) (digest string) {
	return base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(packedMsg))
}
