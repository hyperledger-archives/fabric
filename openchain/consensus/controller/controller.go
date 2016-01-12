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

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/noops"
	"github.com/openblockchain/obc-peer/openchain/consensus/obcpbft"
)

// =============================================================================
// Init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/controller")
}

// =============================================================================
// Constructors go here
// =============================================================================

// NewConsenter constructs a consenter object.
// Called by handler.NewConsensusHandler().
func NewConsenter(cpi consensus.CPI) consensus.Consenter {
	plugin := viper.GetString("peer.validator.consensus")
	var algo consensus.Consenter
	if plugin == "obcpbft" {
		logger.Debug("Running with OBC-PBFT consensus")
		algo = obcpbft.GetPlugin(cpi)
	} else {
		logger.Debug("Running with NOOPS consensus")
		algo = noops.GetNoops(cpi)
	}
	return algo
}
