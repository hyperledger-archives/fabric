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

package consensus

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/op/go-logging"
)

// =============================================================================
// Init
// =============================================================================

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus")
}

// =============================================================================
// Interface definitions go here
// =============================================================================

// Consenter is implemented by every consensus algorithm implementation (plugin).
type Consenter interface {
	RecvMsg(msg *pb.OpenchainMessage) error
}

// CPI stands for Consensus Programming Interface.
// It is the set of stack-facing methods available to the plugin.
type CPI interface {
	GetReplicas() (replicas []string, err error)
	GetReplicaID() (id uint64, err error)
	Broadcast(msg *pb.OpenchainMessage) error
	Unicast(msgPayload []byte, receiver string) error
	ExecTXs(txs []*pb.Transaction) ([]byte, []error)
}
