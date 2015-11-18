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
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/consensus/noops"
	"github.com/openblockchain/obc-peer/openchain/consensus/pbft"
	"github.com/spf13/viper"
)

var controllerLogger = logging.MustGetLogger("controller")

// NewConsenter constructs a consenter object
func NewConsenter(cpi consensus.CPI) consensus.Consenter {
	if viper.GetString("peer.mode") == "dev" {
		return noops.New(cpi)
	}
	plugin := viper.GetString("peer.consensus.plugin")
	var algo consensus.Consenter
	if plugin == "pbft" {
		controllerLogger.Debug("Running with PBFT consensus")
		algo = pbft.New(cpi)
	} else {
		controllerLogger.Debug("Running with NOOPS consensus")
		algo = noops.New(cpi)
	}
	return algo
}
