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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

// Build a chaincode.
func build(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	fmt.Printf("Received build request for chaincode spec: %v\n", spec)
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
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// Deploy a chaincode - i.e., build and initialize.
func deploy(ctx context.Context, spec *pb.ChaincodeSpec) ([]byte, error) {
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := build(ctx, spec)

	if err != nil {
		return nil, err
	}
	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return nil, uuidErr
	}
	transaction, err := pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	return Execute(ctx, GetChain(DefaultChain), transaction)
}

// Invoke or query a chaincode.
func invoke(ctx context.Context, spec *pb.ChaincodeSpec, typ pb.Transaction_Type) (string, []byte, error) {
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		return "", nil, uuidErr
	}
	transaction, err := pb.NewChaincodeExecute(chaincodeInvocationSpec, uuid, typ)
	if err != nil {
		return uuid, nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	retval, err := Execute(ctx, GetChain(DefaultChain), transaction)
	return uuid, retval, err
}

func closeListenerAndSleep(l net.Listener) {
	l.Close()
	time.Sleep(2 * time.Second)
}

// Test deploy of a transaction.
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
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//lis, err := net.Listen("tcp", viper.GetString("peer.address"))

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"
	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01"
	version := "0.0.0"
	f := "init"
	args := []string{"a", "100", "b", "200"}
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Url: url, Version: version}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	chaincodeID, _ := getChaincodeID(&pb.ChaincodeID{Url: url, Version: version})
	_, err = deploy(ctxt, spec)
	if err != nil {
		GetChain(DefaultChain).stopChaincode(ctxt, &pb.ChaincodeID{Url: url, Version: version})
		closeListenerAndSleep(lis)
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}

	GetChain(DefaultChain).stopChaincode(ctxt, &pb.ChaincodeID{Url: url, Version: version})
	closeListenerAndSleep(lis)
}

const (
	kEXPECTED_DELTA_STRING_PREFIX = "expected delta for transaction"
)

// Check the correctness of the final state after transaction execution.
func checkFinalState(uuid string, chaincodeID string) error {
	// Check the state in the ledger
	ledgerObj, ledgerErr := ledger.GetLedger()
	if ledgerErr != nil {
		return fmt.Errorf("Error checking ledger for <%s>: %s", chaincodeID, ledgerErr)
	}

	_, delta, err := ledgerObj.GetTempStateHashWithTxDeltaStateHashes()

	if err != nil {
		return fmt.Errorf("Error getting delta for invoke transaction <%s>: %s", chaincodeID, err)
	}

	if delta[uuid] == nil {
		return fmt.Errorf("%s <%s> but found nil", kEXPECTED_DELTA_STRING_PREFIX, uuid)
	}

	fmt.Printf("found delta for transaction <%s>\n", uuid)

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

	// Success
	fmt.Printf("Aval = %d, Bval = %d\n", Aval, Bval)
	return nil
}

// Invoke chaincode_example02
func invokeExample02Transaction(ctxt context.Context, cID *pb.ChaincodeID, args []string) error {

	chaincodeID, _ := getChaincodeID(cID)

	f := "init"
	argsDeploy := []string{"a", "100", "b", "200"}
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: argsDeploy}}
	_, err := deploy(ctxt, spec)
	if err != nil {
		return fmt.Errorf("Error deploying <%s>: %s", chaincodeID, err)
	}

	time.Sleep(time.Second)

	fmt.Printf("Going to invoke\n")
	f = "invoke"
	spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	uuid, _, err := invoke(ctxt, spec, pb.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return fmt.Errorf("Error invoking <%s>: %s", chaincodeID, err)
	}

	err = checkFinalState(uuid, chaincodeID)
	if err != nil {
		return fmt.Errorf("Incorrect final state after transaction for <%s>: %s", chaincodeID, err)
	}

	// Test for delete state
	f = "delete"
	delArgs := []string{"a"}
	spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: delArgs}}
	uuid, _, err = invoke(ctxt, spec, pb.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return fmt.Errorf("Error deleting state in <%s>: %s", chaincodeID, err)
	}

	return nil
}

// Test the invocation of a transaction.
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
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	//url := "https://github.com/prjayach/chaincode_examples/chaincode_example02"
	version := "0.0.0"
	chaincodeID := &pb.ChaincodeID{Url: url, Version: version}

	args := []string{"a", "b", "10"}
	err = invokeExample02Transaction(ctxt, chaincodeID, args)
	if err != nil {
		t.Fail()
		t.Logf("Error invoking transaction: %s", err)
	} else {
		fmt.Printf("Invoke test passed\n")
		t.Logf("Invoke test passed")
	}

	GetChain(DefaultChain).stopChaincode(ctxt, chaincodeID)

	closeListenerAndSleep(lis)
}

// Execute multiple transactions and queries.
func exec(ctxt context.Context, numTrans int, numQueries int) []error {
	var wg sync.WaitGroup
	errs := make([]error, numTrans+numQueries)

	e := func(qnum int, typ pb.Transaction_Type) {
		defer wg.Done()
		url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
		version := "0.0.0"

		var spec *pb.ChaincodeSpec
		if typ == pb.Transaction_CHAINCODE_EXECUTE {
			f := "invoke"
			args := []string{"a", "b", "10"}

			spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Url: url, Version: version}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

			fmt.Printf("Going to invoke TRANSACTION num %d\n", qnum)
		} else {
			f := "query"
			args := []string{"a"}

			spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Url: url, Version: version}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

			fmt.Printf("Going to invoke QUERY num %d\n", qnum)
		}

		uuid, _, err := invoke(ctxt, spec, typ)

		if typ == pb.Transaction_CHAINCODE_EXECUTE {
			ledgerObj, ledgerErr := ledger.GetLedger()
			if ledgerErr != nil {
				errs[qnum] = fmt.Errorf("Error getting ledger %s", ledgerErr)
				return
			}
			_, delta, err := ledgerObj.GetTempStateHashWithTxDeltaStateHashes()
			if err != nil {
				errs[qnum] = fmt.Errorf("Error getting delta for invoke transaction <%s> :%s", uuid, err)
				return
			}

			if delta[uuid] == nil {
				errs[qnum] = fmt.Errorf("%s <%s> but found nil", kEXPECTED_DELTA_STRING_PREFIX, uuid)
				return
			}
			fmt.Printf("found delta for transaction <%s>\n", uuid)
		}

		if err != nil {
			chaincodeID, _ := getChaincodeID(&pb.ChaincodeID{Url: url, Version: version})
			errs[qnum] = fmt.Errorf("Error executing <%s>: %s", chaincodeID, err)
			return
		}
	}
	wg.Add(numTrans + numQueries)

	//execute transactions sequentially..
	go func() {
		for i := 0; i < numTrans; i++ {
			e(i, pb.Transaction_CHAINCODE_EXECUTE)
		}
	}()

	//...but queries in parallel
	for i := numTrans; i < numTrans+numQueries; i++ {
		go e(i, pb.Transaction_CHAINCODE_QUERY)
	}

	wg.Wait()
	fmt.Printf("EXEC DONE\n")
	return errs
}

// Test the execution of a query.
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
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version := "0.0.0"

	cID := &pb.ChaincodeID{Url: url, Version: version}
	chaincodeID, _ := getChaincodeID(cID)
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	numTrans := 2
	numQueries := 10
	errs := exec(ctxt, numTrans, numQueries)

	var numerrs int
	for i := 0; i < numTrans+numQueries; i++ {
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

// Test the execution of an invalid transaction.
func TestExecuteInvokeInvalidTransaction(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version := "0.0.0"
	chaincodeID := &pb.ChaincodeID{Url: url, Version: version}

	//FAIL, FAIL!
	args := []string{"x", "-1"}
	err = invokeExample02Transaction(ctxt, chaincodeID, args)

	//this HAS to fail with kEXPECTED_DELTA_STRING_PREFIX
	if err != nil {
		errStr := err.Error()
		t.Logf("Got error %s\n", errStr)
		t.Logf("InvalidInvoke test passed")
		GetChain(DefaultChain).stopChaincode(ctxt, chaincodeID)

		closeListenerAndSleep(lis)
		return
	}

	t.Fail()
	t.Logf("Error invoking transaction %s", err)

	GetChain(DefaultChain).stopChaincode(ctxt, chaincodeID)

	closeListenerAndSleep(lis)
}

// Test the execution of an invalid query.
func TestExecuteInvalidQuery(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example03"
	version := "0.0.0"

	cID := &pb.ChaincodeID{Url: url, Version: version}
	chaincodeID, _ := getChaincodeID(cID)
	f := "init"
	args := []string{"a", "100"}

	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	f = "query"
	args = []string{"b","200"}

	spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// This query should fail as it attempts to put state
	_, _, err = invoke(ctxt, spec, pb.Transaction_CHAINCODE_QUERY)

	if err == nil {
		t.Fail()
		t.Logf("This query should not have succeeded as it attempts to put state")
	}	

	GetChain(DefaultChain).stopChaincode(ctxt, cID)
	closeListenerAndSleep(lis)
}

// Test the execution of a chaincode that invokes another chaincode.
func TestChaincodeInvokeChaincode(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version1 := "0.0.1"

	cID1 := &pb.ChaincodeID{Url: url1, Version: version1}
	chaincodeID1, _ := getChaincodeID(cID1)
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example04"
	version2 := "0.0.1"

	cID2 := &pb.ChaincodeID{Url: url2, Version: version2}
	chaincodeID2, _ := getChaincodeID(cID2)
	f = "init"
	args = []string{"e", "0"}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn invoke the first chaincode
	f = "invoke"
	args = []string{"e","1"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	var uuid string
	uuid, _, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_EXECUTE)

	if err != nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	// Check the state in the ledger
	err = checkFinalState(uuid, chaincodeID1)
	if err != nil {
		t.Fail()
		t.Logf("Incorrect final state after transaction for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).stopChaincode(ctxt, cID1)
	GetChain(DefaultChain).stopChaincode(ctxt, cID2)
	closeListenerAndSleep(lis)
}

// Test the execution of a chaincode query that queries another chaincode.
func TestChaincodeQueryChaincode(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.db.path", "/var/openchain/tmpdb")

	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:40303"

	lis, err := net.Listen("tcp", peerAddress)
	if err != nil {
		t.Fail()
		t.Logf("Error starting peer listener %s", err)
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02"
	version1 := "0.0.1"

	cID1 := &pb.ChaincodeID{Url: url1, Version: version1}
	chaincodeID1, _ := getChaincodeID(cID1)
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example05"
	version2 := "0.0.1"

	cID2 := &pb.ChaincodeID{Url: url2, Version: version2}
	chaincodeID2, _ := getChaincodeID(cID2)
	f = "init"
	args = []string{"sum", "0"}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn query the first chaincode
	t.Logf("Starting chaincode query chaincode in transaction mode")
	f = "invoke"
	args = []string{"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02", "0.0.1", "sum"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	var retVal []byte
	_, retVal, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_EXECUTE)

	if err != nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	// Check the return value
	result, err := strconv.Atoi(string(retVal))
	if err != nil || result != 300 {
		t.Fail()
		t.Logf("Incorrect final state after transaction for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	// Query second chaincode, which will inturn query the first chaincode
	t.Logf("Starting chaincode query chaincode in query mode")
	f = "query"
	args = []string{"github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02", "0.0.1", "sum"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	_, retVal, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_QUERY)

	if err != nil {
		t.Fail()
		t.Logf("Error querying <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	// Check the return value
	result, err = strconv.Atoi(string(retVal))
	if err != nil || result != 300 {
		t.Fail()
		t.Logf("Incorrect final value after query for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).stopChaincode(ctxt, cID1)
		GetChain(DefaultChain).stopChaincode(ctxt, cID2)
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).stopChaincode(ctxt, cID1)
	GetChain(DefaultChain).stopChaincode(ctxt, cID2)
	closeListenerAndSleep(lis)
}

func TestMain(m *testing.M) {
	SetupTestConfig()
	os.Exit(m.Run())
}
