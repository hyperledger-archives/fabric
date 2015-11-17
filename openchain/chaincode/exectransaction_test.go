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
	"sync"
	"time"

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
	return Execute(ctx, GetChain(DefaultChain), transaction)
}

func invoke(ctx context.Context, spec *pb.ChainletSpec, typ pb.Transaction_Type) ([]byte, error) {
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChainletSpec: spec}

	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return nil, uuidErr
	}
	transaction, err := pb.NewChainletExecute(chaincodeInvocationSpec, uuid, typ)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	return Execute(ctx, GetChain(DefaultChain), transaction)
}

func closeListenerAndSleep(l net.Listener) {
	l.Close()
	time.Sleep(2*time.Second)
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

	//lis, err := net.Listen("tcp", viper.GetString("peer.address"))

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
        peerAddress = "0.0.0.0:40303"
	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}


	pb.RegisterChainletSupportServer(grpcServer, NewChainletSupport())
	
        //Override UserRunsCC if set to true
        UserRunsCC = false

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
		closeListenerAndSleep(lis)
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}
	closeListenerAndSleep(lis)
}

func invokeExample02Transaction(ctxt context.Context, cID *pb.ChainletID) error {

	chaincodeID,_ := getChaincodeID(cID)

	f := "init"
	args := []string{ "a", "100", "b", "200" }
        spec := &pb.ChainletSpec { Type: 1, ChainletID: cID, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	_,err := deploy(ctxt, spec)
	if err != nil {
		return fmt.Errorf("Error deploying <%s>: %s", chaincodeID, err)
	}

	time.Sleep(time.Second)

	fmt.Printf("Going to invoke\n")
	f = "invoke"
	args = []string{ "a", "b", "10" }
       	spec = &pb.ChainletSpec { Type: 1, ChainletID: cID, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }
	_,err = invoke(ctxt, spec, pb.Transaction_CHAINLET_EXECUTE)
	if err != nil {
		return fmt.Errorf("Error invoking <%s>: %s", chaincodeID, err)
	}

	// Check the state in the ledger
	ledgerObj, ledgerErr := ledger.GetLedger()
	if ledgerErr != nil {
		return fmt.Errorf("Error checking ledger for <%s>: %s", chaincodeID, ledgerErr)
	}

	// Invoke ledger to get state
	var Aval, Bval int
	resbytes, resErr := ledgerObj.GetState(chaincodeID, "a", false)
	if resErr != nil {
		return fmt.Errorf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
	} 
	fmt.Printf("Got string: %s\n", string(resbytes))
	Aval, resErr = strconv.Atoi(string(resbytes))
	if resErr != nil {
		return fmt.Errorf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
	}
	if Aval != 90 {
		return fmt.Errorf("Incorrect result. Aval is wrong for <%s>", chaincodeID)
	}

	resbytes, resErr = ledgerObj.GetState(chaincodeID, "b", false)
	if resErr != nil {
		return fmt.Errorf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
	} 
	Bval, resErr = strconv.Atoi(string(resbytes))
	if resErr != nil {
		return fmt.Errorf("Error retrieving state from ledger for <%s>: %s", chaincodeID, resErr)
	}
	if Bval != 210 {
		return fmt.Errorf("Incorrect result. Bval is wrong for <%s>", chaincodeID)
	}

	fmt.Printf("Aval = %d, Bval = %d\n",Aval, Bval)

	return nil
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

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
        peerAddress = "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	pb.RegisterChainletSupportServer(grpcServer, NewChainletSupport())
	
        //Override UserRunsCC if set to true
        UserRunsCC = false

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version := "0.0.0"
	chaincodeID := &pb.ChainletID{Url: url, Version: version}

	err = invokeExample02Transaction(ctxt, chaincodeID)
	if err != nil {
		t.Fail()
		t.Logf("Error invoking transaction %s", err)
	} else {
		fmt.Printf("Invoke test passed\n")
		t.Logf("Invoke test passed")
	}
	
	GetChain(DefaultChain).stopChaincode(ctxt, chaincodeID)

	closeListenerAndSleep(lis)
}

func exec(ctxt context.Context, numTrans int, numQueries int) []error {
	var wg sync.WaitGroup
	errs := make([]error, numTrans+numQueries)

	exec := func(qnum int, typ pb.Transaction_Type) {
		url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
		version := "0.0.0"

		var spec *pb.ChainletSpec
		if typ == pb.Transaction_CHAINLET_EXECUTE {
			f := "invoke"
			args := []string{ "a", "b", "10" }

       			spec = &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: url, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }

			fmt.Printf("Going to invoke TRANSACTION num %d\n", qnum)
		} else {
			f := "query"
			args := []string{ "a" }

       			spec = &pb.ChainletSpec { Type: 1, ChainletID: &pb.ChainletID{ Url: url, Version: version }, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }

			fmt.Printf("Going to invoke QUERY num %d\n", qnum)
		}

		_,err := invoke(ctxt, spec, typ)

		if err != nil {
			chaincodeID,_ := getChaincodeID(&pb.ChainletID{Url: url, Version: version})
			errs[qnum] = fmt.Errorf("Error executign <%s>: %s", chaincodeID, err)
			wg.Done()
			return
		}
		wg.Done()
	}
	wg.Add(numTrans+numQueries)

	//execute transactions sequntially..
	go func() {
		for i:=0; i<numTrans; i++ {
			exec(i, pb.Transaction_CHAINLET_EXECUTE)
		}
	}()

	//...but queries in parallel
	for i:=numTrans; i<numTrans+numQueries; i++ {
		go exec(i, pb.Transaction_CHAINLET_QUERY)
	}

	wg.Wait()
	return errs
}

func TestExecuteQuery(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
        peerAddress = "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	pb.RegisterChainletSupportServer(grpcServer, NewChainletSupport())
	
        //Override UserRunsCC if set to true
        UserRunsCC = false

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version := "0.0.0"

	cID := &pb.ChainletID{Url: url, Version: version}
	chaincodeID,_ := getChaincodeID(cID)
	f := "init"
	args := []string{ "a", "100", "b", "200" }

        spec := &pb.ChainletSpec { Type: 1, ChainletID: cID, CtorMsg: &pb.ChaincodeInput{ Function : f, Args: args } }

	_,err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	numTrans := 1
	numQueries := 10
	errs := exec(ctxt, numTrans, numQueries)

	var numerrs int
	for i:=0; i < numTrans+numQueries; i++ {
		if errs[i] != nil {
			t.Logf("Error doing query on %d %s", i, errs[i])
			numerrs++
		}
	}
		
	if numerrs == 0 {
		fmt.Printf("Query test passed\n")
		t.Logf("Query test passed")
	} else {
		t.Logf("Query test failed(total errors %d)", numerrs)
		t.Fail()
	}

	GetChain(DefaultChain).stopChaincode(ctxt, cID)
	closeListenerAndSleep(lis)
}
func TestMain(m *testing.M) {
        SetupTestConfig()
        os.Exit(m.Run())
}

