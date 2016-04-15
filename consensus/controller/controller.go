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

package controller

import (
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"strings"
	"sync"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/consensus/noops"
	"github.com/hyperledger/fabric/consensus/obcpbft"
	"github.com/hyperledger/fabric/consensus/util"
)

var logger *logging.Logger // package-level logger
var MessageFan *util.MessageFan
var consenter consensus.Consenter
var lock sync.Mutex

func init() {
	logger = logging.MustGetLogger("consensus/controller")
	MessageFan = util.NewMessageFan()
}

// NewConsenter constructs a Consenter object if not already present
func NewConsenter(stack consensus.Stack) consensus.Consenter {
	lock.Lock()
	defer lock.Unlock()
	// TODO, construct this singleton at initialization, not driven by Handler

	if consenter != nil {
		return consenter
	}

	plugin := strings.ToLower(viper.GetString("peer.validator.consensus.plugin"))
	if plugin == "pbft" {
		//logger.Info("Running with consensus plugin %s", plugin)
		consenter = obcpbft.GetPlugin(stack)
	} else {
		//logger.Info("Running with default consensus plugin (noops)")
		consenter = noops.GetNoops(stack)
	}

	go func() {
		logger.Debug("Starting up message thread for consenter")

		// The channel never closes, so this should never break
		for msg := range MessageFan.GetOutChannel() {
			logger.Debug("Received message from %v delivering to consenter", msg.Sender)
			consenter.RecvMsg(msg.Msg, msg.Sender)
		}
	}()

	return consenter
}
