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

package client

import (
	"fmt"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
	"os"
	"testing"
)

var client *Client

func TestMain(m *testing.M) {
	client = new(Client)
	err := client.Init()
	if err != nil {
		panic(fmt.Errorf("Client Security Module:TestMain: failed initializing security layer: err %s", err))
	} else {
		os.Exit(m.Run())
	}
}

func Test_NewChainletDeployTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChainletDeployTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChainletDeployTransaction(
		&pb.ChainletDeploymentSpec{
			ChainletSpec: &pb.ChainletSpec{
				Type:       pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		uuid,
	)

	if err != nil {
		t.Fatalf("Test_NewChainletDeployTransaction: failed creating NewChainletDeployTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChainletDeployTransaction: failed creating NewChainletDeployTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChainletDeployTransaction: failed checking transaction: err %s", err)
	}
}

func Test_NewChainletInvokeTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChainletInvokeTransaction(
		&pb.ChaincodeInvocation{
			ChainletSpec: &pb.ChainletSpec{
				Type:       pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
			Message: &pb.ChainletMessage{
				Function: "hello",
				Args:     []string{"World!!!"},
			},
		},
		uuid,
	)

	if err != nil {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed creating NewChainletInvokeTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed creating NewChainletInvokeTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed checking transaction: err %s", err)
	}
}
