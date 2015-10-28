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

package helper

import (
	"golang.org/x/net/context"

	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("consensus")

// Helper data structure
type Helper struct {
	consenter consensus.Consenter
}

// New is a constructor returning a consensus.CPI
func New() consensus.CPI {
	return &Helper{}
}

// SetConsenter is called from the implementor. It is a singleton
// @c - the consenter for this consensus
func (h *Helper) SetConsenter(c consensus.Consenter) {
	logger.Info("Setting the consenter")
	h.consenter = c
}

// HandleMsg is called by the VP FSM when OpenchainMessage.Type = CONSENSUS
func (h *Helper) HandleMsg(msg *pb.OpenchainMessage) error {
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Handle message: %s", msg.Type)
	}
	return h.consenter.Recv(msg.Payload)
}

// Broadcast sends the message to all validators. This is called by the
// consenter to broadcast messages during consensus. We wrap the msg as
// payload of the OpenchainMessage_CONSENSUS
func (h *Helper) Broadcast(msg []byte) error {
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Broadcast a message")
	}

	// TODO: Call someone to send newMsg
	// newMsg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS, Payload: msg}

	return nil
}

//ExecTXs will execute transactions on the array one by one
//will return an array of errors one for each transaction. If the execution
//succeeded, array element will be nil. returns state hash
func (h *Helper) ExecTXs(ctxt context.Context, xacts []*pb.Transaction) ([]byte, []error) {
	return nil, nil
}
