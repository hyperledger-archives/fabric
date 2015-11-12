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
	"fmt"
	"time"

	pb "google/protobuf"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

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
	msgStore map[string]*Request // Where we store incoming `REQUEST` messages.
}

// =============================================================================
// Custom interface definitions go here.
// =============================================================================

type validator interface {
	getParam(param string) (val string, err error)
	isLeader() bool
	setLeader(flag bool) bool
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
	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	instance.config.AddConfigPath("../pbft/") // For when you run a test from `controller`.
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// Create the data store for incoming messages.
	instance.msgStore = make(map[string]*Request)

	return instance
}

// =============================================================================
// Consenter interface implementation goes here.
// =============================================================================

// Request is used to post a new transaction request to PBFT.  This
// peer takes over the role of a client and broadcasts the request to
// all replicas.  Eventually every peer will pass the transaction to
// CPI.ExecTx.
func (instance *Plugin) Request(transaction []byte) (err error) {
	logger.Info("new request received")

	req := &Request{
		Timestamp:   &pb.Timestamp{Seconds: time.Now().Unix()},
		Transaction: transaction,
	}
	reqMsg, err := proto.Marshal(&Message{Payload: &Message_Request{req}})
	if err != nil {
		err = fmt.Errorf("Error marshalling REQUEST message: ", err)
		logger.Error("", err)
		return err
	}
	return instance.cpi.Broadcast(reqMsg)
}

// RecvMsg is called on the receiving peers for all messages PBFT
// sends via Broadcast or Unicast
func (instance *Plugin) RecvMsg(payload []byte) (err error) {
	logger.Info("PBFT message received")

	extractedMsg := &Message{}
	err = proto.Unmarshal(payload, extractedMsg)
	if err != nil {
		err = fmt.Errorf("Error unpacking payload from message: %s", err)
		logger.Error("", err)
		return err
	}

	if msg := extractedMsg.GetRequest(); msg != nil {
		logger.Debug("received request, timestamp=%d", msg.Timestamp)
	} else if msg := extractedMsg.GetPrePrepare(); msg != nil {
		logger.Debug("received pre-prepare, view=%d sequenceNumber=%d digest=%x",
			msg.View, msg.SequenceNumber, msg.RequestDigest)
	} else {
		err = fmt.Errorf("Received unknown message type")
		logger.Error("", err)
	}

	return
}

// =============================================================================
// Custom interface implementation goes here.
// =============================================================================

// getParam is a getter for the values listed in `config.yaml`.
func (instance *Plugin) getParam(param string) (val string, err error) {
	if ok := instance.config.IsSet(param); !ok {
		err := fmt.Errorf("Key %s does not exist in algo config", param)
		return "nil", err
	}
	val = instance.config.GetString(param)
	return val, nil
}

// isLeader allows us to check whether a validating peer is the current leader.
func (instance *Plugin) isLeader() bool {

	return instance.leader
}

// setLeader flags a validating peer as the leader. This is a temporary state.
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
