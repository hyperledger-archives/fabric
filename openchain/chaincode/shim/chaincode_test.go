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

package shim

import (
	"fmt"
	"testing"

	"golang.org/x/net/context"

	pb "github.com/openblockchain/obc-peer/protos"
)

type TestChainlet struct {
}

func (t *TestChainlet) Run(chainletSupportClient pb.ChainletSupportClient) error {
	status, err := chainletSupportClient.GetExecutionContext(context.Background(), &pb.ChainletRequestContext{})
	if err != nil {
		return fmt.Errorf("Error getting execution context: %s\n", err)
	}
	fmt.Printf("Current status: %v  err: %v\n", status, err)
	return nil
}

func TestChainlet_Start(t *testing.T) {
	t.Skip("TODO: Have to rework client with new chaincode shim setup")
	err := Start(new(TestChainlet))
	if err != nil {
		t.Logf("Error Start(ing) chaincode: %s", err)
		t.Fail()
	}
}
