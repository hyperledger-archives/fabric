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

package validator

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"fmt"
	"os"
	"testing"
)

var validator *Validator

func TestMain(m *testing.M) {
	validator = new(Validator)
	err := validator.Init()
	if err != nil {
		panic(fmt.Errorf("Peer Security Module:TestMain: failed initializing security layer: err %s", err))
	} else {
		os.Exit(m.Run())
	}
}

func TestID(t *testing.T) {
	// Verify that any id modification doesn't change
	id := validator.GetID()
	id[0] = id[0] + 1
	id2 := validator.GetID()
	if id2[0] == id[0] {
		t.Fatalf("Invariant not respected.")
	}
}

func TestDeployTransactionPreValidation(t *testing.T) {
	res, err := validator.TransactionPreValidation(mockDeployTransaction())
	if res == nil {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreValidation(t *testing.T) {
	res, err := validator.TransactionPreValidation(mockInvokeTransaction())
	if res == nil {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestDeployTransactionPreExecution(t *testing.T) {
	res, err := validator.TransactionPreExecution(mockDeployTransaction())
	if res == nil {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreExecution(t *testing.T) {
	res, err := validator.TransactionPreExecution(mockInvokeTransaction())
	if res == nil {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}
}

func TestSignVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := validator.Sign(msg)
	if err != nil {
		t.Fatalf("TestSign: failed generating signature: %s", err)
	}

	err = validator.Verify(validator.GetID(), signature, msg)
	if err != nil {
		t.Fatalf("TestSign: failed validating signature: %s", err)
	}
}

func mockDeployTransaction() *pb.Transaction {
	tx, _ := pb.NewChainletDeployTransaction(
		&pb.ChainletDeploymentSpec{
			ChainletSpec: &pb.ChainletSpec{
				Type:       pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		"uuid",
	)
	return tx
}

func mockInvokeTransaction() *pb.Transaction {
	tx, _ := pb.NewChainletExecute(
		&pb.ChaincodeInvocationSpec{
			ChainletSpec: &pb.ChainletSpec{
				Type:       pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:    nil,
			},
		},
		"uuid",
		pb.Transaction_CHAINLET_EXECUTE,
	)
	return tx
}
