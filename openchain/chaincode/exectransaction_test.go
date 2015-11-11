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

	"golang.org/x/net/context"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/util"
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
	chainletDeploymentSepc := &pb.ChainletDeploymentSpec{ChainletSpec: spec, CodePackage: codePackageBytes}
	return chainletDeploymentSepc, nil
}

func deploy(ctx context.Context, spec *pb.ChainletSpec) ([]byte, error) {
	// First build and get the deployment spec
	chainletDeploymentSepc, err := build(ctx, spec)

	if err != nil {
		return nil, err
	}
	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return nil, uuidErr
	}
	transaction, err := pb.NewChainletDeployTransaction(chainletDeploymentSepc, uuid)
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

	chainletID := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01"
	version := "0.0.0"
	f := "f"
	args := []string{ "a", "100", "b", "200" }
        spec := &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: chainletID, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	_,err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chainletID, err)
		return
	}
}

func TestMain(m *testing.M) {
        SetupTestConfig()
        os.Exit(m.Run())
}

