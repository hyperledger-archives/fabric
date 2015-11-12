package client

import (
	pb "github.com/openblockchain/obc-peer/protos"
	"testing"
	"os"
	"github.com/openblockchain/obc-peer/openchain/util"
	"fmt"
)

var client *Client

func TestMain(m *testing.M) {
	client = new(Client)
	err := client.Init()
	if (err != nil) {
		fmt.Errorf("Client Security Module:TestMain: failed initializing security layer: err $s", err)
		os.Exit(-1);
	} else {
		os.Exit(m.Run())
	}
}

func Test_NewChainletDeployTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChainletDeployTransaction: failed generating uuid: err $s", err)
	}
	tx, err := client.NewChainletDeployTransaction(
		&pb.ChainletDeploymentSpec{
			ChainletSpec: &pb.ChainletSpec{
				Type: pb.ChainletSpec_GOLANG,
				ChainletID: &pb.ChainletID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg: nil,
			},
			EffectiveDate: nil,
			CodePackage: nil,
		},
		uuid,
	)

	if (err != nil) {
		t.Fatalf("Test_NewChainletDeployTransaction: failed creating NewChainletDeployTransaction: err $s", err)
	}

	if (tx == nil) {
		t.Fatalf("Test_NewChainletDeployTransaction: failed creating NewChainletDeployTransaction: result is nil")
	}

	err = client.checkTransaction(tx);
	if (err != nil) {
		t.Fatalf("Test_NewChainletDeployTransaction: failed checking transaction: err $s", err)
	}
}

func Test_NewChainletInvokeTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed generating uuid: err $s", err)
	}
	tx, err := client.NewChainletInvokeTransaction(
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
		uuid,
	)

	if (err != nil) {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed creating NewChainletInvokeTransaction: err $s", err)
	}

	if (tx == nil) {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed creating NewChainletInvokeTransaction: result is nil")
	}

	err = client.checkTransaction(tx);
	if (err != nil) {
		t.Fatalf("Test_NewChainletInvokeTransaction: failed checking transaction: err $s", err)
	}
}
