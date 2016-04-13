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

package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/membersrvc/ca"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"io/ioutil"
	"os"
	"path/filepath"

	"reflect"
)

const (
	chaincodeStartupTimeoutDefault int = 5000
)

var (
	testLogger = logging.MustGetLogger("test")

	lis net.Listener

	administrator crypto.Client
	alice         crypto.Client
	bob           crypto.Client

	server *grpc.Server
	eca    *ca.ECA
	tca    *ca.TCA
	tlsca  *ca.TLSCA
)

func TestMain(m *testing.M) {
	removeFolders()
	setup()
	go initOBCCA()

	fmt.Println("Wait for some secs for OBCCA")
	time.Sleep(2 * time.Second)

	go initVP()

	fmt.Println("Wait for some secs for VP")
	time.Sleep(2 * time.Second)

	go initAssetManagementChaincode()

	fmt.Println("Wait for some secs for Chaincode")
	time.Sleep(2 * time.Second)

	if err := initClients(); err != nil {
		panic(err)
	}

	fmt.Println("Wait for 10 secs for chaincode to be started")
	time.Sleep(10 * time.Second)

	ret := m.Run()

	closeListenerAndSleep(lis)

	defer removeFolders()
	os.Exit(ret)
}

func TestAssetManagement(t *testing.T) {
	// Administrator deploy the chaicode
	adminCert, err := administrator.GetTCertificateHandlerNext()
	if err != nil {
		t.Fatal(err)
	}

	if err := deploy(adminCert); err != nil {
		t.Fatal(err)
	}

	// Administrator assigns ownership of Picasso to Alice
	aliceCert, err := alice.GetTCertificateHandlerNext()
	if err != nil {
		t.Fatal(err)
	}

	// This must fail
	if err := assignOwnership(aliceCert, "Picasso", aliceCert); err == nil {
		t.Fatal(err)
	}

	// This must succeed
	if err := assignOwnership(adminCert, "Picasso", aliceCert); err != nil {
		t.Fatal(err)
	}

	theOnwerIs, err := whoIsTheOwner("Picasso")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(theOnwerIs, aliceCert.GetCertificate()) {
		t.Fatal("Alice is not the owner of Picasso")
	}

	// Alice transfers ownership of Picasso to Bob
	bobCert, err := bob.GetTCertificateHandlerNext()
	if err != nil {
		t.Fatal(err)
	}

	// This must fail
	if err := transferOwnership(bob, bobCert, "Picasso", adminCert); err == nil {
		t.Fatal(err)
	}

	// This must succeed
	if err := transferOwnership(alice, aliceCert, "Picasso", bobCert); err != nil {
		t.Fatal(err)
	}

	theOnwerIs, err = whoIsTheOwner("Picasso")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(theOnwerIs, bobCert.GetCertificate()) {
		t.Fatal("Bob is not the owner of Picasso")
	}

}

func deploy(admCert crypto.CertificateHandler) error {
	// Prepare the spec. The metadata includes the role of the users allowed to assign assets
	spec := &pb.ChaincodeSpec{
		Type:                 1,
		ChaincodeID:          &pb.ChaincodeID{Name: "mycc"},
		CtorMsg:              &pb.ChaincodeInput{Function: "init", Args: []string{}},
		Metadata:             []byte("assigner"),
		ConfidentialityLevel: pb.ConfidentialityLevel_PUBLIC,
	}

	// First build and get the deployment spec
	var ctx = context.Background()
	chaincodeDeploymentSpec, err := getDeploymentSpec(ctx, spec)
	if err != nil {
		return err
	}

	tid := chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := administrator.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, tid)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	ledger.BeginTxBatch("1")
	_, err = chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return err
}

func assignOwnership(admCert crypto.CertificateHandler, asset string, newOwnerCert crypto.CertificateHandler) error {
	// Get a transaction handler to be used to submit the execute transaction
	// and bind the chaincode access control logic using the binding
	submittingCertHandler, err := administrator.GetTCertificateHandlerNext()
	if err != nil {
		return err
	}
	txHandler, err := submittingCertHandler.GetTransactionHandler()
	if err != nil {
		return err
	}
	binding, err := txHandler.GetBinding()
	if err != nil {
		return err
	}

	chaincodeInput := &pb.ChaincodeInput{Function: "assign", Args: []string{asset, string(newOwnerCert.GetCertificate())}}
	chaincodeInputRaw, err := proto.Marshal(chaincodeInput)
	if err != nil {
		return err
	}

	// Access control. Administrator signs chaincodeInputRaw || binding to confirm his identity
	sigma, err := admCert.Sign(append(chaincodeInputRaw, binding...))
	if err != nil {
		return err
	}

	// Prepare spec and submit
	spec := &pb.ChaincodeSpec{
		Type:                 1,
		ChaincodeID:          &pb.ChaincodeID{Name: "mycc"},
		CtorMsg:              chaincodeInput,
		Metadata:             sigma, // Proof of identity
		ConfidentialityLevel: pb.ConfidentialityLevel_PUBLIC,
	}

	var ctx = context.Background()
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	tid := chaincodeInvocationSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := txHandler.NewChaincodeExecute(chaincodeInvocationSpec, tid)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	ledger.BeginTxBatch("1")
	_, err = chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return err
}

func transferOwnership(owner crypto.Client, ownerCert crypto.CertificateHandler, asset string, newOwnerCert crypto.CertificateHandler) error {
	// Get a transaction handler to be used to submit the execute transaction
	// and bind the chaincode access control logic using the binding

	submittingCertHandler, err := owner.GetTCertificateHandlerNext()
	if err != nil {
		return err
	}
	txHandler, err := submittingCertHandler.GetTransactionHandler()
	if err != nil {
		return err
	}
	binding, err := txHandler.GetBinding()
	if err != nil {
		return err
	}

	chaincodeInput := &pb.ChaincodeInput{Function: "transfer", Args: []string{asset, string(newOwnerCert.GetCertificate())}}
	chaincodeInputRaw, err := proto.Marshal(chaincodeInput)
	if err != nil {
		return err
	}

	// Access control. Owner signs chaincodeInputRaw || binding to confirm his identity
	sigma, err := ownerCert.Sign(append(chaincodeInputRaw, binding...))
	if err != nil {
		return err
	}

	// Prepare spec and submit
	spec := &pb.ChaincodeSpec{
		Type:                 1,
		ChaincodeID:          &pb.ChaincodeID{Name: "mycc"},
		CtorMsg:              chaincodeInput,
		Metadata:             sigma, // Proof of identity
		ConfidentialityLevel: pb.ConfidentialityLevel_PUBLIC,
	}

	var ctx = context.Background()
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	tid := chaincodeInvocationSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := txHandler.NewChaincodeExecute(chaincodeInvocationSpec, tid)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	ledger.BeginTxBatch("1")
	_, err = chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
	if err != nil {
		return fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return err

}

func whoIsTheOwner(asset string) ([]byte, error) {
	chaincodeInput := &pb.ChaincodeInput{Function: "query", Args: []string{asset}}

	// Prepare spec and submit
	spec := &pb.ChaincodeSpec{
		Type:                 1,
		ChaincodeID:          &pb.ChaincodeID{Name: "mycc"},
		CtorMsg:              chaincodeInput,
		ConfidentialityLevel: pb.ConfidentialityLevel_PUBLIC,
	}

	var ctx = context.Background()
	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	tid := chaincodeInvocationSpec.ChaincodeSpec.ChaincodeID.Name

	// Now create the Transactions message and send to Peer.
	transaction, err := administrator.NewChaincodeQuery(chaincodeInvocationSpec, tid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	ledger, err := ledger.GetLedger()
	ledger.BeginTxBatch("1")
	result, err := chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s", err)
	}
	ledger.CommitTxBatch("1", []*pb.Transaction{transaction}, nil, nil)

	return result, err
}

func setup() {
	// Conf
	viper.SetConfigName("asset") // name of config file (without extension)
	viper.AddConfigPath(".")     // path to look for the config file in
	err := viper.ReadInConfig()  // Find and read the config file
	if err != nil {              // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file [%s] \n", err))
	}

	// Logging
	var formatter = logging.MustStringFormatter(
		`%{color}[%{module}] %{shortfunc} [%{shortfile}] -> %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(formatter)

	logging.SetLevel(logging.DEBUG, "peer")
	logging.SetLevel(logging.DEBUG, "chaincode")
	logging.SetLevel(logging.DEBUG, "cryptoain")

	// Init the crypto layer
	if err := crypto.Init(); err != nil {
		panic(fmt.Errorf("Failed initializing the crypto layer [%s]%", err))
	}

	viper.Set("peer.fileSystemPath", filepath.Join(os.TempDir(), "hyperledger", "production"))
	viper.Set("server.rootpath", filepath.Join(os.TempDir(), "ca"))

	removeFolders()
}

func initOBCCA() {
	ca.LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = ca.NewECA()
	tca = ca.NewTCA(eca)
	tlsca = ca.NewTLSCA(eca)

	var opts []grpc.ServerOption
	if viper.GetBool("peer.pki.tls.enabled") {
		// TLS configuration
		creds, err := credentials.NewServerTLSFromFile(
			filepath.Join(viper.GetString("server.rootpath"), "tlsca.cert"),
			filepath.Join(viper.GetString("server.rootpath"), "tlsca.priv"),
		)
		if err != nil {
			panic("Failed creating credentials for OBC-CA: " + err.Error())
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	fmt.Printf("open socket...\n")
	sockp, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		panic("Cannot open port: " + err.Error())
	}
	fmt.Printf("open socket...done\n")

	server = grpc.NewServer(opts...)

	eca.Start(server)
	tca.Start(server)
	tlsca.Start(server)

	fmt.Printf("start serving...\n")
	server.Serve(sockp)
}

func initVP() {
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
	peerAddress := "0.0.0.0:40404"
	var err error
	lis, err = net.Listen("tcp", peerAddress)
	if err != nil {
		return
	}

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(chaincodeStartupTimeoutDefault) * time.Millisecond
	userRunsCC := true

	// Install security object for peer
	var secHelper crypto.Peer
	if viper.GetBool("security.enabled") {
		enrollID := viper.GetString("security.enrollID")
		enrollSecret := viper.GetString("security.enrollSecret")
		var err error

		if viper.GetBool("peer.validator.enabled") {
			testLogger.Debug("Registering validator with enroll ID: %s", enrollID)
			if err = crypto.RegisterValidator(enrollID, nil, enrollID, enrollSecret); nil != err {
				panic(err)
			}
			testLogger.Debug("Initializing validator with enroll ID: %s", enrollID)
			secHelper, err = crypto.InitValidator(enrollID, nil)
			if nil != err {
				panic(err)
			}
		} else {
			testLogger.Debug("Registering non-validator with enroll ID: %s", enrollID)
			if err = crypto.RegisterPeer(enrollID, nil, enrollID, enrollSecret); nil != err {
				panic(err)
			}
			testLogger.Debug("Initializing non-validator with enroll ID: %s", enrollID)
			secHelper, err = crypto.InitPeer(enrollID, nil)
			if nil != err {
				panic(err)
			}
		}
	}

	pb.RegisterChaincodeSupportServer(grpcServer,
		chaincode.NewChaincodeSupport(chaincode.DefaultChain, getPeerEndpoint, userRunsCC,
			ccStartupTimeout, secHelper))

	grpcServer.Serve(lis)
}

func initAssetManagementChaincode() {
	err := shim.Start(new(AssetManagementChaincode))
	if err != nil {
		panic(err)
	}
}

func initClients() error {
	// Administrator
	if err := crypto.RegisterClient("jim", nil, "jim", "6avZQLwcUe9b"); err != nil {
		return err
	}
	var err error
	administrator, err = crypto.InitClient("jim", nil)
	if err != nil {
		return err
	}

	// Alice
	if err := crypto.RegisterClient("lukas", nil, "lukas", "NPKYL39uKbkj"); err != nil {
		return err
	}
	alice, err = crypto.InitClient("lukas", nil)
	if err != nil {
		return err
	}

	// Bob
	if err := crypto.RegisterClient("diego", nil, "diego", "DRJ23pEQl16a"); err != nil {
		return err
	}
	bob, err = crypto.InitClient("diego", nil)
	if err != nil {
		return err
	}

	return nil
}

func closeListenerAndSleep(l net.Listener) {
	l.Close()
	time.Sleep(2 * time.Second)
}

func getDeploymentSpec(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	fmt.Printf("getting deployment spec for chaincode spec: %v\n", spec)
	var codePackageBytes []byte
	//if we have a name, we don't need to deploy (we are in userRunsCC mode)
	if spec.ChaincodeID.Name == "" {
		var err error
		codePackageBytes, err = container.GetChaincodePackageBytes(spec)
		if err != nil {
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

func removeFolders() {
	if err := os.RemoveAll(filepath.Join(os.TempDir(), ".ca")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", ".ca", err)
	}
	if err := os.RemoveAll(filepath.Join(os.TempDir(), ".fabric")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", ".fabric", err)
	}
}
