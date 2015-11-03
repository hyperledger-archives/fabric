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

package chaincode

import (
	"testing"

	openchain "github.com/openblockchain/obc-peer/openchain"

	pb "github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

// func Benchmark_GetExecutionContext(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		performChat(b, peerClientConn)
// 	}
// }

// func Benchmark_GetExecutionContext_Parallel(b *testing.B) {
// 	b.SetParallelism(10)
// 	b.RunParallel(func(pb *testing.PB) {
// 		for pb.Next() {
// 			performChat(b, peerClientConn)
// 		}
// 	})
// }

func TestChainletSupport_GetExecutionContext(t *testing.T) {
	t.Skip("TODO: Have to rework chaincode testing.")
	clientConn, err := openchain.NewPeerClientConnection()
	if err != nil {
		t.Logf("Error trying to connect to local peer:", err)
		t.Fail()
		return
	}

	t.Log("Getting execution context from peer")
	serverClient := pb.NewChainletSupportClient(clientConn)

	status, err := serverClient.GetExecutionContext(context.Background(), &pb.ChainletRequestContext{})
	if err != nil {
		t.Errorf("Error getting execution context: %s", err)
		return
	}
	t.Logf("Current status: %v  err: %v", status, err)

}
