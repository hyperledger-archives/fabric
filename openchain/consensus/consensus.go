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
	"golang.org/x/net/context"

	pb "github.com/openblockchain/obc-peer/protos"
)

// Consenter is an interface for consensus implementation
type Consenter interface {
	Recv(msg []byte) error
}

// CPI (Consensus Programming Interface) is to break the import cycle between
// consensus and consenter implementation
type CPI interface {
	SetConsenter(c Consenter)
	HandleMsg(msg *pb.OpenchainMessage) error
	Broadcast(msg []byte) error
	ExecTXs(ctxt context.Context, xacts []*pb.Transaction) ([]byte, []error)
}
