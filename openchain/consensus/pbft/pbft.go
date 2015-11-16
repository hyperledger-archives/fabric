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
	cpi      consensus.CPI       // The consensus programming interface
	config   *viper.Viper        // The link to the config file
	leader   bool                // Is this validating peer the current leader?
	msgStore map[string]*Message // Where we store incoming `REQUEST` messages.
}

// =============================================================================
// Custom interface definitions go here.
// =============================================================================

type validator interface {
	getParam(param string) (val string, err error)
	isLeader() bool
	retrieveRequest(digest string) (reqMsg *Message, err error)
	setLeader(flag bool) bool
	storeRequest(digest string, reqMsg *Message) (count int)
}

// =============================================================================
// Constructors go here.
// =============================================================================

// New creates an implementation-specific structure that will be held in the
// consensus `helper` object. (See `controller` and `helper` packages for more.)
func New(c consensus.CPI) *Plugin {

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Creating the consenter.")
	}
	instance := &Plugin{}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Setting the consenter's CPI.")
	}
	instance.cpi = c

	// TODO: Initialize the algorithm here.
	// You may want to set the fields of `instance` using `instance.GetParam()`.
	// e.g. instance.blockTimeOut = strconv.Atoi(instance.getParam("timeout.block"))

	// Create a link to the config file.
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Linking to the consenter's config file.")
	}
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
	instance.msgStore = make(map[string]*Message)

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

	reqMsgPacked, err := proto.Marshal(reqMsg)
	if err != nil {
		return fmt.Errorf("Error marshalling request message.")
	}

	err = instance.cpi.Broadcast(reqMsgPacked)
	if err == nil {
		// route to ourselves as well
		return instance.RecvMsg(reqMsgPacked)
	}

	return err
}

// RecvMsg receives messages transmitted by CPI.Broadcast or CPI.Unicast.
func (instance *Plugin) RecvMsg(msgRaw []byte) error {
	msg := &Message{}
	err := proto.Unmarshal(msgRaw, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	if req := msg.GetRequest(); req != nil {
		logger.Debug("request received")
		digest := hashMsg(msgRaw)
		_ = instance.storeRequest(digest, msg)
	} else if preprep := msg.GetPrePrepare(); preprep != nil {
		logger.Debug("pre-prepare received")
	} else if prep := msg.GetPrepare(); prep != nil {
		logger.Debug("prepare received")
	} else if commit := msg.GetCommit(); commit != nil {
		logger.Debug("commit received")
	} else {
		err := fmt.Errorf("invalid message: ", msgRaw)
		logger.Error("%s", err)
		return err
	}

	return nil
}

// =============================================================================
// Custom interface implementation goes here.
// =============================================================================

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

// retrieve
func (instance *Plugin) retrieveRequest(digest string) (reqMsg *Message, err error) {

	if val, ok := instance.msgStore[digest]; ok {
		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Message with digest %s found in map.", digest)
		}
		return val, nil
	}

	err = fmt.Errorf("Message with digest %s does not exist in map.", digest)
	return nil, err

}

// Flags a validating peer as the leader. This is a temporary state.
func (instance *Plugin) setLeader(flag bool) bool {

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Setting the leader flag.")
	}

	instance.leader = flag

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Leader flag set.")
	}

	return instance.leader
}

// Maps a `REQUEST` message to its digest and stores it for future reference.
func (instance *Plugin) storeRequest(digest string, reqMsg *Message) (count int) {

	if _, ok := instance.msgStore[digest]; ok {
		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debug("Message with digest %s already exists in map.", digest)
		}
	}

	instance.msgStore[digest] = reqMsg
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Stored REQUEST message in map.")
	}

	count = len(instance.msgStore)
	return
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

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Unmarshaled payload, number of transactions it carries: %d", numTx)
	}

	// Extract transaction.
	if numTx != 1 {
		err = fmt.Errorf("request should carry 1 transaction instead of: %d", numTx)
		return
	}

	tx := txBatch.Transactions[0]

	// Marshal transaction.
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		err = fmt.Errorf("Error marshalling single transaction.")
		return
	}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Marshaled single transaction.")
	}

	reqMsg = &Message{&Message_Request{&Request{
		Timestamp: tx.Timestamp,
		Payload:   txPacked,
	}}}

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Created REQUEST message.")
	}

	return
}

// Calculate the digest of a marshalled message.
func hashMsg(packedMsg []byte) (digest string) {

	digest = base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(packedMsg))

	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Digest of marshalled message is: %s", digest)
	}

	return
}
