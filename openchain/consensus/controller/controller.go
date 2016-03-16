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

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/noops"
	"github.com/openblockchain/obc-peer/openchain/consensus/obcpbft"
)

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/controller")
}

// NewConsenter constructs a Consenter object
func NewConsenter(stack consensus.Stack) (consenter consensus.Consenter) {
	plugin := strings.ToLower(viper.GetString("peer.validator.consensus"))
	if plugin == "obcpbft" {
		//logger.Info("Running with consensus plugin %s", plugin)
		consenter = obcpbft.GetPlugin(stack)
	} else {
		//logger.Info("Running with default consensus plugin (noops)")
		consenter = noops.GetNoops(stack)
	}
	return
}
