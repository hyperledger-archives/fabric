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

package peer

import (
	obc "github.com/openblockchain/obc-peer/protos"

	"fmt"
	"os"
	"testing"
)

var peer *Peer

func TestMain(m *testing.M) {
	peer = new(Peer)

	err := peer.Init()
	if err != nil {
		panic(fmt.Errorf("Peer Security Module:TestMain: failed initializing security layer: err %s", err))
	} else {
		os.Exit(m.Run())
	}
}

func TestID(t *testing.T) {
	// Verify that any id modification doesn't change
	id := peer.GetID()
	id[0] = id[0] + 1
	id2 := peer.GetID()
	if id2[0] == id[0] {
		t.Fatalf("Invariant not respected.")
	}
}

func TestDeployTransactionPreValidation(t *testing.T) {
	tx, err := peer.TransactionPreValidation(mockDeployTransaction())

	if tx == nil {
		t.Fatalf("TransactionPreValidation: transaction must be different from nil.")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreValidation(t *testing.T) {
	tx, err := peer.TransactionPreValidation(mockInvokeTransaction())

	if tx == nil {
		t.Fatalf("TransactionPreValidation: transaction must be different from nil.")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func mockDeployTransaction() *obc.Transaction {
	tx, _ := obc.NewChaincodeDeployTransaction(
		&obc.ChaincodeDeploymentSpec{
			ChaincodeSpec: &obc.ChaincodeSpec{
				Type:        obc.ChaincodeSpec_GOLANG,
				ChaincodeID: &obc.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		"uuid",
	)
	return tx
}

func mockInvokeTransaction() *obc.Transaction {
	tx, _ := obc.NewChaincodeExecute(
		&obc.ChaincodeInvocationSpec{
			ChaincodeSpec: &obc.ChaincodeSpec{
				Type:        obc.ChaincodeSpec_GOLANG,
				ChaincodeID: &obc.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
		},
		"uuid",
		obc.Transaction_CHAINCODE_EXECUTE,
	)
	return tx
}
