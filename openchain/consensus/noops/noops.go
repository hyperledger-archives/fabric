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

package noops

import (
	"github.com/openblockchain/obc-peer/openchain/consensus"

	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("noops")

// Noops is a consensus plugin object implementing consensus.Consenter interface
type Noops struct {
	cpi consensus.CPI // The consensus programming interface
}

// New is a constructor returning a consensus.Consenter object
func New(c consensus.CPI) consensus.Consenter {
	i := &Noops{}
	i.cpi = c
	return i
}

// Request is the main entry into the consensus plugin.  `txs` will be
// passed to CPI.ExecTXs once consensus is reached.
func (i *Noops) Request(txs []byte) (err error) {
	logger.Info("Request received")

	err = i.cpi.Broadcast(txs) // broadcast to others so they can exec the tx
	if err == nil {
		err = i.RecvMsg(txs) // process locally as well
	}

	return
}

// RecvMsg receives messages transmitted by CPI.Broadcast or CPI.Unicast.
func (i *Noops) RecvMsg(msg []byte) (err error) {
	logger.Info("Message received")

	return nil // cpi.ExecTXs(ctx context.Context, txs []*pb.Transaction) ([]byte, []error)
}
