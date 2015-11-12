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
	"fmt"
	"net"
	"os"
	"testing"
	"strconv"

	"golang.org/x/net/context"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	pb "github.com/openblockchain/obc-peer/protos"
)

func build(context context.Context, spec *pb.ChainletSpec) (*pb.ChainletDeploymentSpec, error) {
	fmt.Printf("Received build request for chainlet spec: %v\n", spec)
	var codePackageBytes []byte
	// Get new VM and as for building of container image
	vm, err := container.NewVM()
	if err != nil {
		return nil, err
	}
	// Build the spec
	codePackageBytes, err = vm.BuildChaincodeContainer(spec)
	if err != nil {
		return nil, err
	}
	chainletDeploymentSpec := &pb.ChainletDeploymentSpec{ChainletSpec: spec, CodePackage: codePackageBytes}
	return chainletDeploymentSpec, nil
}

func deploy(ctx context.Context, spec *pb.ChainletSpec) ([]byte, error) {
	// First build and get the deployment spec
	chainletDeploymentSpec, err := build(ctx, spec)

	if err != nil {
		return nil, err
	}
	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return nil, uuidErr
	}
	transaction, err := pb.NewChainletDeployTransaction(chainletDeploymentSpec, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	return Execute(ctx, GetChain(DEFAULTCHAIN), transaction)
}

func invoke(ctx context.Context, spec *pb.ChainletSpec) ([]byte, error) {
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChainletSpec: spec}

	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return nil, uuidErr
	}
	transaction, err := pb.NewChainletInvokeTransaction(chaincodeInvocationSpec, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	return Execute(ctx, GetChain(DEFAULTCHAIN), transaction)
}

func TestExecuteDeployTransaction(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)

	lis, err := net.Listen("tcp", viper.GetString("peer.address"))
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	pb.RegisterChainletSupportServer(grpcServer, NewChainletSupport())
	
        //Override USER_RUNS_CC if set to true
        USER_RUNS_CC = false

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01"
	version := "0.0.0"
	f := "init"
	args := []string{ "a", "100", "b", "200" }
        spec := &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: url, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	chaincodeID,_ := getChaincodeID(&pb.ChainletID{Url: url, Version: version})
	_,err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}
}

func TestExecuteInvokeTransaction(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)

	lis, err := net.Listen("tcp", viper.GetString("peer.address"))
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	pb.RegisterChainletSupportServer(grpcServer, NewChainletSupport())
	
        //Override USER_RUNS_CC if set to true
        USER_RUNS_CC = false

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version := "0.0.0"
	chaincodeID,_ := getChaincodeID(&pb.ChainletID{Url: url, Version: version})
	f := "init"
	args := []string{ "a", "100", "b", "200" }
        spec := &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: url, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	_,err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}

	fmt.Printf("Going to invoke")
	f = "invoke"
	args = []string{ "a", "b", "10" }
        spec = &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: url, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	_,err = invoke(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID, err)
		return
	}

	// Check the state in the ledger
	ledgerObj, ledgerErr := ledger.GetLedger()
	if ledgerErr != nil {
		t.Fail()
		t.Logf("Error checking ledger for <%s>: %s", chaincodeID, ledgerErr)
		return
	}

	// Invoke ledger to get state
	var Aval, Bval int
	resbytes, resErr := ledgerObj.GetState(chaincodeID, "a")
	if resErr != nil {
		t.Fail()
		t.Logf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
		return
	} 
	Aval, resErr = strconv.Atoi(string(resbytes))
	if resErr != nil {
		t.Fail()
		t.Logf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
		return
	}
	if Aval != 90 {
		t.Fail()
		t.Logf("Incorrect result. Aval is wrong for <%s>", chaincodeID)
		return
	}

	resbytes, resErr = ledgerObj.GetState(chaincodeID, "b")
	if resErr != nil {
		t.Fail()
		t.Logf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
		return
	} 
	Bval, resErr = strconv.Atoi(string(resbytes))
	if resErr != nil {
		t.Fail()
		t.Logf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
		return
	}
	if Bval != 210 {
		t.Fail()
		t.Logf("Incorrect result. Bval is wrong for <%s>", chaincodeID)
		return
	}
	fmt.Printf("Aval = %d, Bval = %d\n",Aval, Bval)
	fmt.Printf("Test passed")
	t.Logf("Test passed")
}

func TestMain(m *testing.M) {
        SetupTestConfig()
        os.Exit(m.Run())
}

