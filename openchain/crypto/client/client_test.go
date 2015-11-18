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

func Test_NewChaincodeDeployTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChaincodeDeployTransaction(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:       pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		uuid,
	)

	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed creating NewChaincodeDeployTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed creating NewChaincodeDeployTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed checking transaction: err %s", err)
	}
}

func Test_NewChaincodeInvokeTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChaincodeInvokeTransaction(
		&pb.ChaincodeInvocationSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:       pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
		},
		uuid,
	)

	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed checking transaction: err %s", err)
	}
}
