package test

import (
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/config"
	pb "github.com/hyperledger/fabric/protos"
)

func TestMain(m *testing.M) {
	config.SetupTestConfig("../../../../../peer")
	os.Exit(m.Run())
}

func TestCar_BuildImage(t *testing.T) {
	if os.Getenv("VAGRANT") == "" {
		t.Skip("skipping test; only supported within vagrant")
	}

	vm, err := container.NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
		return
	}
	// Build the spec
	cwd, err := os.Getwd()
	if err != nil {
		t.Fail()
		t.Logf("Error getting CWD: %s", err)
		return
	}

	chaincodePath := cwd + "/org.hyperledger.chaincode.example02-0.1-SNAPSHOT.car"
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_CAR, ChaincodeID: &pb.ChaincodeID{Path: chaincodePath}, CtorMsg: &pb.ChaincodeInput{Function: "f"}}
	if _, err := vm.BuildChaincodeContainer(spec); err != nil {
		t.Fail()
		t.Log(err)
	}
}

