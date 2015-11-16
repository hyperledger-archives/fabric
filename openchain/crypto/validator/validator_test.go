package validator

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"testing"
	"os"
	"fmt"
)

var validator *Validator


func TestMain(m *testing.M) {
	validator = new(Validator)
	err := validator.Init()
	if (err != nil) {
		fmt.Errorf("Peer Security Module:TestMain: failed initializing security layer: err $s", err)
		os.Exit(-1);
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
	res, err := validator.TransactionPreValidation(mockDeployTransaction());
	if (res == nil) {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreValidation(t *testing.T) {
	res, err := validator.TransactionPreValidation(mockInvokeTransaction());
	if (res == nil) {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestDeployTransactionPreExecution(t *testing.T) {
	res, err := validator.TransactionPreExecution(mockDeployTransaction());
	if (res == nil) {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreExecution(t *testing.T) {
	res, err := validator.TransactionPreExecution(mockInvokeTransaction());
	if (res == nil) {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}
}


func mockDeployTransaction() (*pb.Transaction) {
	tx, _ := pb.NewChainletDeployTransaction(
		&pb.ChainletDeploymentSpec{
			ChainletSpec: &pb.ChainletSpec{
				Type: pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg: nil,
			},
			EffectiveDate: nil,
			CodePackage: nil,
		},
		"uuid",
	)
	return tx
}

func mockInvokeTransaction() (*pb.Transaction) {
	tx, _ := pb.NewChainletInvokeTransaction(
		&pb.ChaincodeInvocation{
			ChainletSpec: &pb.ChainletSpec{
				Type: pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg: nil,
			},
			Message:  &pb.ChainletMessage{
				Function: "hello",
				Args: []string{"World!!!"},
			},
		},
		"uuid",
	)
	return tx
}