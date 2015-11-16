package peer

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"testing"
	"os"
	"fmt"
)

var peer *Peer


func TestMain(m *testing.M) {
	peer = new(Peer)

	err := peer.Init()
	if (err != nil) {
		fmt.Errorf("Peer Security Module:TestMain: failed initializing security layer: err $s", err)
		os.Exit(-1);
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

	if (tx == nil) {
		t.Fatalf("TransactionPreValidation: transaction must be different from nil.")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransactionPreValidation(t *testing.T) {
	tx, err := peer.TransactionPreValidation(mockInvokeTransaction())

	if (tx == nil) {
		t.Fatalf("TransactionPreValidation: transaction must be different from nil.")
	}
	if (err != nil) {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
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