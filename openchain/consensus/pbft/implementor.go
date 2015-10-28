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
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/consensus"
)

var logger = logging.MustGetLogger("consensus")

// PluginByzantine implements Plugin interface
type PluginByzantine struct {
	cpi consensus.CPI
}

// New constructor
func New(c consensus.CPI) consensus.Consenter {
	return &PluginByzantine{cpi: c}
}

// Called by the package loader
func init() {
	// TODO: initialize the algorithm here
}

// Recv is called by consensus
func (p *PluginByzantine) Recv(msg []byte) error {
	if logger.IsEnabledFor(logging.DEBUG) {
		logger.Debug("Receiving a message")
	}
	return nil
}
