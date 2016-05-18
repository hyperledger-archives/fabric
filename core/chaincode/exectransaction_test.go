/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chaincode

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/system_chaincode"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

func getNowMillis() int64 {
	nanos := time.Now().UnixNano()
	return nanos / 1000000
}

// Build a chaincode.
func getDeploymentSpec(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	fmt.Printf("getting deployment spec for chaincode spec: %v\n", spec)
	codePackageBytes, err := container.GetChaincodePackageBytes(spec)
	if err != nil {
		return nil, err
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// Deploy a chaincode - i.e., build and initialize.
func deploy(ctx context.Context, spec *pb.ChaincodeSpec) ([]byte, error) {
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := getDeploymentSpec(ctx, spec)
	if err != nil {
		return nil, err
	}

	tid := chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, tid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get handle to ledger: %s ", err)
	}
	ledger.BeginTxBatch("1")
	b, err := Execute(ctx, GetChain(DefaultChain), transaction)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return b, err
}

func deploy2(ctx context.Context, chaincodeDeploymentSpec *pb.ChaincodeDeploymentSpec) ([]byte, error) {
	tid := chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, tid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	ledger.BeginTxBatch("1")
	b, err := Execute(ctx, GetChain(DefaultChain), transaction)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return b, err
}

// Invoke or query a chaincode.
func invoke(ctx context.Context, spec *pb.ChaincodeSpec, typ pb.Transaction_Type) (string, []byte, error) {
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	// Now create the Transactions message and send to Peer.
	uuid := util.GenerateUUID()

	transaction, err := pb.NewChaincodeExecute(chaincodeInvocationSpec, uuid, typ)
	if err != nil {
		return uuid, nil, fmt.Errorf("Error invoking chaincode: %s ", err)
	}

	var retval []byte
	var execErr error
	if typ == pb.Transaction_CHAINCODE_QUERY {
		retval, execErr = Execute(ctx, GetChain(DefaultChain), transaction)
	} else {
		ledger, _ := ledger.GetLedger()
		ledger.BeginTxBatch("1")
		retval, execErr = Execute(ctx, GetChain(DefaultChain), transaction)
		if err != nil {
			return uuid, nil, fmt.Errorf("Error invoking chaincode: %s ", err)
		}
		ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)
	}

	return uuid, retval, execErr
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example01"
	f := "init"
	args := []string{"a", "100", "b", "200"}
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Path: url}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	_, err = deploy(ctxt, spec)
	chaincodeID := spec.ChaincodeID.Name
	if err != nil {
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
		closeListenerAndSleep(lis)
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
	closeListenerAndSleep(lis)
}

// Check the correctness of the final state after transaction execution.
func checkFinalState(uuid string, chaincodeID string) error {
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

	// Success
	fmt.Printf("Aval = %d, Bval = %d\n", Aval, Bval)
	return nil
}

// Invoke chaincode_example02
func invokeExample02Transaction(ctxt context.Context, cID *pb.ChaincodeID, args []string) error {

	f := "init"
	argsDeploy := []string{"a", "100", "b", "200"}
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: argsDeploy}}
	_, err := deploy(ctxt, spec)
	chaincodeID := spec.ChaincodeID.Name
	if err != nil {
		return fmt.Errorf("Error deploying <%s>: %s", chaincodeID, err)
	}

	time.Sleep(time.Second)

	f = "invoke"
	spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	uuid, _, err := invoke(ctxt, spec, pb.Transaction_CHAINCODE_INVOKE)
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
	uuid, _, err = invoke(ctxt, spec, pb.Transaction_CHAINCODE_INVOKE)
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"
	chaincodeID := &pb.ChaincodeID{Path: url}

	args := []string{"a", "b", "10"}
	err = invokeExample02Transaction(ctxt, chaincodeID, args)
	if err != nil {
		t.Fail()
		t.Logf("Error invoking transaction: %s", err)
	} else {
		fmt.Printf("Invoke test passed\n")
		t.Logf("Invoke test passed")
	}

	GetChain(DefaultChain).Stop(ctxt,  &pb.ChaincodeDeploymentSpec{ChaincodeSpec:&pb.ChaincodeSpec{ChaincodeID: chaincodeID}})

	closeListenerAndSleep(lis)
}

// Execute multiple transactions and queries.
func exec(ctxt context.Context, chaincodeID string, numTrans int, numQueries int) []error {
	var wg sync.WaitGroup
	errs := make([]error, numTrans+numQueries)

	e := func(qnum int, typ pb.Transaction_Type) {
		defer wg.Done()
		var spec *pb.ChaincodeSpec
		if typ == pb.Transaction_CHAINCODE_INVOKE {
			f := "invoke"
			args := []string{"a", "b", "10"}

			spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Name: chaincodeID}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
		} else {
			f := "query"
			args := []string{"a"}

			spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Name: chaincodeID}, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
		}

		_, _, err := invoke(ctxt, spec, typ)

		if err != nil {
			errs[qnum] = fmt.Errorf("Error executing <%s>: %s", chaincodeID, err)
			return
		}
	}
	wg.Add(numTrans + numQueries)

	//execute transactions sequentially..
	go func() {
		for i := 0; i < numTrans; i++ {
			e(i, pb.Transaction_CHAINCODE_INVOKE)
		}
	}()

	//...but queries in parallel
	for i := numTrans; i < numTrans+numQueries; i++ {
		go e(i, pb.Transaction_CHAINCODE_QUERY)
	}

	wg.Wait()
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"

	cID := &pb.ChaincodeID{Path: url}
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec)
	chaincodeID := spec.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(2 * time.Second)

	//start := getNowMillis()
	//fmt.Fprintf(os.Stderr, "Starting: %d\n", start)
	numTrans := 2
	numQueries := 10
	errs := exec(ctxt, chaincodeID, numTrans, numQueries)

	var numerrs int
	for i := 0; i < numTrans+numQueries; i++ {
		if errs[i] != nil {
			t.Logf("Error doing query on %d %s", i, errs[i])
			numerrs++
		}
	}

	if numerrs == 0 {
		t.Logf("Query test passed")
	} else {
		t.Logf("Query test failed(total errors %d)", numerrs)
		t.Fail()
	}

	//end := getNowMillis()
	//fmt.Fprintf(os.Stderr, "Ending: %d\n", end)
	//fmt.Fprintf(os.Stderr, "Elapsed : %d millis\n", end-start)
	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"
	chaincodeID := &pb.ChaincodeID{Path: url}

	//FAIL, FAIL!
	args := []string{"x", "-1"}
	err = invokeExample02Transaction(ctxt, chaincodeID, args)

	//this HAS to fail with expectedDeltaStringPrefix
	if err != nil {
		errStr := err.Error()
		t.Logf("Got error %s\n", errStr)
		t.Logf("InvalidInvoke test passed")
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:&pb.ChaincodeSpec{ChaincodeID:chaincodeID}})

		closeListenerAndSleep(lis)
		return
	}

	t.Fail()
	t.Logf("Error invoking transaction %s", err)

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:&pb.ChaincodeSpec{ChaincodeID:chaincodeID}})

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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	url := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example03"

	cID := &pb.ChaincodeID{Path: url}
	f := "init"
	args := []string{"a", "100"}

	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec)
	chaincodeID := spec.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	f = "query"
	args = []string{"b", "200"}

	spec = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// This query should fail as it attempts to put state
	_, _, err = invoke(ctxt, spec, pb.Transaction_CHAINCODE_QUERY)

	if err == nil {
		t.Fail()
		t.Logf("This query should not have succeeded as it attempts to put state")
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec})
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"

	cID1 := &pb.ChaincodeID{Path: url1}
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	chaincodeID1 := spec1.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example04"

	cID2 := &pb.ChaincodeID{Path: url2}
	f = "init"
	args = []string{"e", "0"}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	chaincodeID2 := spec2.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn invoke the first chaincode
	f = "invoke"
	args = []string{"e", "1"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	var uuid string
	uuid, _, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_INVOKE)

	if err != nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	// Check the state in the ledger
	err = checkFinalState(uuid, chaincodeID1)
	if err != nil {
		t.Fail()
		t.Logf("Incorrect final state after transaction for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
	closeListenerAndSleep(lis)
}

// Test the execution of a chaincode that invokes another chaincode with wrong parameters. Should receive error from
// from the called chaincode
func TestChaincodeInvokeChaincodeErrorCase(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"

	cID1 := &pb.ChaincodeID{Path: url1}
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	chaincodeID1 := spec1.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/hyperledger/fabric/examples/chaincode/go/passthru"

	cID2 := &pb.ChaincodeID{Path: url2}
	f = "init"
	args = []string{""}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	chaincodeID2 := spec2.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn invoke the first chaincode but pass bad params
	f = chaincodeID1
	args = []string{"invoke", "a"} //expect {"invoke", "a","b","10"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	_, _, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_INVOKE)

	if err == nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	if strings.Index(err.Error(), "Incorrect number of arguments. Expecting 3") < 0 {
		t.Fail()
		t.Logf("Unexpected error %s", err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
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
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"

	cID1 := &pb.ChaincodeID{Path: url1}
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	chaincodeID1 := spec1.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example05"

	cID2 := &pb.ChaincodeID{Path: url2}
	f = "init"
	args = []string{"sum", "0"}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	chaincodeID2 := spec2.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn query the first chaincode
	t.Logf("Starting chaincode query chaincode in transaction mode")
	f = "invoke"
	args = []string{chaincodeID1, "sum"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	var retVal []byte
	_, retVal, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_INVOKE)

	if err != nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	// Check the return value
	result, err := strconv.Atoi(string(retVal))
	if err != nil || result != 300 {
		t.Fail()
		t.Logf("Incorrect final state after transaction for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	// Query second chaincode, which will inturn query the first chaincode
	t.Logf("Starting chaincode query chaincode in query mode")
	f = "query"
	args = []string{chaincodeID1, "sum"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	_, retVal, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_QUERY)

	if err != nil {
		t.Fail()
		t.Logf("Error querying <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	// Check the return value
	result, err = strconv.Atoi(string(retVal))
	if err != nil || result != 300 {
		t.Fail()
		t.Logf("Incorrect final value after query for <%s>: %s", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
	closeListenerAndSleep(lis)
}

// Test the execution of a chaincode that queries another chaincode with invalid parameter. Should receive error from
// from the called chaincode
func TestChaincodeQueryChaincodeErrorCase(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	// Deploy first chaincode
	url1 := "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02"

	cID1 := &pb.ChaincodeID{Path: url1}
	f := "init"
	args := []string{"a", "100", "b", "200"}

	spec1 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID1, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec1)
	chaincodeID1 := spec1.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID1, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Deploy second chaincode
	url2 := "github.com/hyperledger/fabric/examples/chaincode/go/passthru"

	cID2 := &pb.ChaincodeID{Path: url2}
	f = "init"
	args = []string{""}

	spec2 := &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}

	_, err = deploy(ctxt, spec2)
	chaincodeID2 := spec2.ChaincodeID.Name
	if err != nil {
		t.Fail()
		t.Logf("Error initializing chaincode %s(%s)", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	time.Sleep(time.Second)

	// Invoke second chaincode, which will inturn invoke the first chaincode but pass bad params
	f = chaincodeID1
	args = []string{"query", "c"} //expect {"query", "a"}

	spec2 = &pb.ChaincodeSpec{Type: 1, ChaincodeID: cID2, CtorMsg: &pb.ChaincodeInput{Function: f, Args: args}}
	// Invoke chaincode
	_, _, err = invoke(ctxt, spec2, pb.Transaction_CHAINCODE_QUERY)

	if err == nil {
		t.Fail()
		t.Logf("Error invoking <%s>: %s", chaincodeID2, err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	if strings.Index(err.Error(), "Nil amount for c") < 0 {
		t.Fail()
		t.Logf("Unexpected error %s", err)
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
		GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
		closeListenerAndSleep(lis)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec1})
	GetChain(DefaultChain).Stop(ctxt, &pb.ChaincodeDeploymentSpec{ChaincodeSpec:spec2})
	closeListenerAndSleep(lis)
}

// Test deploy of a transaction.
func TestExecuteDeploySysChaincode(t *testing.T) {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	viper.Set("peer.fileSystemPath", "/var/hyperledger/test/tmpdb")

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
	pb.RegisterChaincodeSupportServer(grpcServer, NewChaincodeSupport(DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	var ctxt = context.Background()

	system_chaincode.RegisterSysCCs()

	url := "github.com/hyperledger/fabric/core/system_chaincode/sample_syscc"

	args := []string{"greeting", "hello world"}
	cds := &pb.ChaincodeDeploymentSpec{ExecEnv:1, ChaincodeSpec: &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Name: "sample_syscc", Path: url}, CtorMsg: &pb.ChaincodeInput{Args: args}}}
	_, err = deploy2(ctxt, cds)
	chaincodeID := cds.ChaincodeSpec.ChaincodeID.Name
	if err != nil {
		GetChain(DefaultChain).Stop(ctxt, cds)
		closeListenerAndSleep(lis)
		t.Fail()
		t.Logf("Error deploying <%s>: %s", chaincodeID, err)
		return
	}

	GetChain(DefaultChain).Stop(ctxt, cds)
	closeListenerAndSleep(lis)
}
func TestMain(m *testing.M) {
	SetupTestConfig()
	viper.Set("ledger.blockchain.deploy-system-chaincode", "false")
	viper.Set("validator.validity-period.verification", "false")
	os.Exit(m.Run())
}
