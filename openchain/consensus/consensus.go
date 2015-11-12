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
	"github.com/op/go-logging"
)

// =============================================================================
// Init.
// =============================================================================

// Package-level logger.
var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("consensus")
}

// =============================================================================
// Interface definitions go here.
// =============================================================================

// Consenter should be implemented by every consensus algorithm implementation (plugin).
type Consenter interface {
	// inject an opaque transaction into the consensus layer
	Request(transaction []byte) error
	// receive message as sent via CPI.Broadcast and CPI.Unicast
	RecvMsg(msgPayload []byte) error
}

// CPI (Consensus Programming Interface)
type CPI interface {
	// deliver msgPayload to all other Consenter.RecvMsg
	Broadcast(msgPayload []byte) error
	// deliver msgPayload to one specific Consenter.RecvMsg
	Unicast(msgPayload []byte, receiver string) error
	// execute opaque transaction passed in via Consenter.Request
	ExecTx(transaction []byte) ([]byte, error)
}
