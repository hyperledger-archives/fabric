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

package ca

import (
	"encoding/json"
	"fmt"
	google_protobuf "google/protobuf"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/hyperledger/fabric/consensus/helper"
	"github.com/hyperledger/fabric/core"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/genesis"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/rest"
	pb "github.com/hyperledger/fabric/protos"
)

var (
	tca    *TCA
	eca    *ECA
	server *grpc.Server
)

type ValidityPeriod struct {
	Name  string
	Value string
}

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func setupTestConfig() {
	viper.AutomaticEnv()
	viper.SetConfigName("ca_test") // name of config file (without extension)
	viper.AddConfigPath("./")      // path to look for the config file in
	viper.AddConfigPath("./..")    // path to look for the config file in
	err := viper.ReadInConfig()    // Find and read the config file
	if err != nil {                // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}

func TestValidityPeriod(t *testing.T) {
	t.Skip()
	var updateInterval int64
	updateInterval = 37

	// 1. Start TCA and Openchain...
	go startServices(t)

	// ... and wait just let the services finish the startup
	time.Sleep(time.Second * 240)

	// 2. Obtain the validity period by querying and directly from the ledger
	validityPeriod_A := queryValidityPeriod(t)
	validityPeriodFromLedger_A := getValidityPeriodFromLedger(t)

	// 3. Wait for the validity period to be updated...
	time.Sleep(time.Second * 40)

	// ... and read the values again
	validityPeriod_B := queryValidityPeriod(t)
	validityPeriodFromLedger_B := getValidityPeriodFromLedger(t)

	// 5. Stop TCA and Openchain
	stopServices(t)

	// 6. Compare the values
	if validityPeriod_A != validityPeriodFromLedger_A {
		t.Logf("Validity period read from ledger must be equals tothe one obtained by querying the Openchain. Expected: %s, Actual: %s", validityPeriod_A, validityPeriodFromLedger_A)
		t.Fail()
	}

	if validityPeriod_B != validityPeriodFromLedger_B {
		t.Logf("Validity period read from ledger must be equals tothe one obtained by querying the Openchain. Expected: %s, Actual: %s", validityPeriod_B, validityPeriodFromLedger_B)
		t.Fail()
	}

	if validityPeriod_B-validityPeriod_A != updateInterval {
		t.Logf("Validity period difference must be equal to the update interval. Expected: %s, Actual: %s", updateInterval, validityPeriod_B-validityPeriod_A)
		t.Fail()
	}

	// 7. since the validity period is used as time in the validators convert both validity periods to Unix time and compare them
	vpA := time.Unix(validityPeriodFromLedger_A, 0)
	vpB := time.Unix(validityPeriodFromLedger_B, 0)

	nextVP := vpA.Add(time.Second * 37)
	if !vpB.Equal(nextVP) {
		t.Logf("Validity period difference must be equal to the update interval. Error converting validity period to Unix time.")
		t.Fail()
	}

	// 8. cleanup tca and openchain folders
	if err := os.RemoveAll(viper.GetString("peer.fileSystemPath")); err != nil {
		t.Logf("Failed removing [%s] [%s]\n", viper.GetString("peer.fileSystemPath"), err)
	}
	if err := os.RemoveAll(".ca"); err != nil {
		t.Logf("Failed removing [%s] [%s]\n", ".ca", err)
	}
}

func startServices(t *testing.T) {
	go startTCA()
	err := startOpenchain(t)
	if err != nil {
		t.Logf("Error starting Openchain: %s", err)
		t.Fail()
	}
}

func stopServices(t *testing.T) {
	stopOpenchain(t)
	stopTCA()
}

func startTCA() {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = NewECA()
	defer eca.Close()

	tca = NewTCA(eca)
	defer tca.Close()

	sockp, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		panic("Cannot open port: " + err.Error())
	}

	server = grpc.NewServer()

	eca.Start(server)
	tca.Start(server)

	server.Serve(sockp)
}

func stopTCA() {
	eca.Close()
	tca.Close()
	server.Stop()
}

func queryValidityPeriod(t *testing.T) int64 {
	hash := viper.GetString("pki.validity-period.chaincodeHash")
	args := []string{"system.validity.period"}

	validityPeriod, err := queryTransaction(hash, args, t)
	if err != nil {
		t.Logf("Failed querying validity period: %s", err)
		t.Fail()
	}

	var vp ValidityPeriod
	json.Unmarshal(validityPeriod, &vp)

	value, err := strconv.ParseInt(vp.Value, 10, 64)
	if err != nil {
		t.Logf("Failed parsing validity period: %s", err)
		t.Fail()
	}

	return value
}

func getValidityPeriodFromLedger(t *testing.T) int64 {
	cid := viper.GetString("pki.validity-period.chaincodeHash")

	ledger, err := ledger.GetLedger()
	if err != nil {
		t.Logf("Failed getting access to the ledger: %s", err)
		t.Fail()
	}

	vp_bytes, err := ledger.GetState(cid, "system.validity.period", true)
	if err != nil {
		t.Logf("Failed reading validity period from the ledger: %s", err)
		t.Fail()
	}

	i, err := strconv.ParseInt(string(vp_bytes[:]), 10, 64)
	if err != nil {
		t.Logf("Failed to parse validity period: %s", err)
		t.Fail()
	}

	return i
}

func queryTransaction(hash string, args []string, t *testing.T) ([]byte, error) {

	chaincodeInvocationSpec := createChaincodeInvocationForQuery(args, hash, "system_chaincode_invoker")

	fmt.Printf("Going to query\n")

	response, err := queryChaincode(chaincodeInvocationSpec, t)

	if err != nil {
		return nil, fmt.Errorf("Error querying <%s>: %s", "validity period", err)
	}

	t.Logf("Successfully invoked validity period update: %s", string(response.Msg))

	return response.Msg, nil
}

func queryChaincode(chaincodeInvSpec *pb.ChaincodeInvocationSpec, t *testing.T) (*pb.Response, error) {

	devopsClient, err := getDevopsClient(viper.GetString("pki.validity-period.devops-address"))

	if err != nil {
		return nil, fmt.Errorf("Error retrieving devops client: %s", err)
	}

	resp, err := devopsClient.Query(context.Background(), chaincodeInvSpec)

	if err != nil {
		return nil, fmt.Errorf("Error invoking validity period update system chaincode: %s", err)
	}

	t.Logf("Successfully invoked validity period update: %s(%s)", chaincodeInvSpec, string(resp.Msg))

	return resp, nil
}

func createChaincodeInvocationForQuery(arguments []string, chaincodeHash string, token string) *pb.ChaincodeInvocationSpec {
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Name: chaincodeHash},
		CtorMsg: &pb.ChaincodeInput{Function: "query",
			Args: arguments,
		},
	}

	spec.SecureContext = string(token)

	invocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	return invocationSpec
}

func getSecHelper() (crypto.Peer, error) {
	var secHelper crypto.Peer
	var err error
	if viper.GetBool("security.enabled") {
		enrollID := viper.GetString("security.enrollID")
		enrollSecret := viper.GetString("security.enrollSecret")
		if viper.GetBool("peer.validator.enabled") {
			if err = crypto.RegisterValidator(enrollID, nil, enrollID, enrollSecret); nil != err {
				return nil, err
			}
			secHelper, err = crypto.InitValidator(enrollID, nil)
			if nil != err {
				return nil, err
			}
		} else {
			if err = crypto.RegisterPeer(enrollID, nil, enrollID, enrollSecret); nil != err {
				return nil, err
			}
			secHelper, err = crypto.InitPeer(enrollID, nil)
			if nil != err {
				return nil, err
			}
		}
	}
	return secHelper, err
}

func startOpenchain(t *testing.T) error {

	peerEndpoint, err := peer.GetPeerEndpoint()
	if err != nil {
		return fmt.Errorf("Failed to get Peer Endpoint: %s", err)
	}

	listenAddr := viper.GetString("peer.listenaddress")

	if "" == listenAddr {
		t.Log("Listen address not specified, using peer endpoint address")
		listenAddr = peerEndpoint.Address
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}

	t.Logf("Security enabled status: %t", viper.GetBool("security.enabled"))

	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	grpcServer := grpc.NewServer(opts...)

	// Register the Peer server
	var peerServer *peer.PeerImpl

	secHelper, err := getSecHelper()
	if err != nil {
		return err
	}

	secHelperFunc := func() crypto.Peer {
		return secHelper
	}

	if viper.GetBool("peer.validator.enabled") {
		t.Logf("Running as validating peer - installing consensus %s", viper.GetString("peer.validator.consensus"))
		peerServer, _ = peer.NewPeerWithHandler(secHelperFunc, helper.NewConsensusHandler)
	} else {
		t.Log("Running as non-validating peer")
		peerServer, _ = peer.NewPeerWithHandler(secHelperFunc, peer.NewPeerHandler)
	}
	pb.RegisterPeerServer(grpcServer, peerServer)

	// Register the Admin server
	pb.RegisterAdminServer(grpcServer, core.NewAdminServer())

	// Register ChaincodeSupport server...
	// TODO : not the "DefaultChain" ... we have to revisit when we do multichain

	registerChaincodeSupport(chaincode.DefaultChain, grpcServer, secHelper)

	// Register Devops server
	serverDevops := core.NewDevopsServer(peerServer)
	pb.RegisterDevopsServer(grpcServer, serverDevops)

	// Register the ServerOpenchain server
	serverOpenchain, err := rest.NewOpenchainServer()
	if err != nil {
		return fmt.Errorf("Error creating OpenchainServer: %s", err)
	}

	pb.RegisterOpenchainServer(grpcServer, serverOpenchain)

	// Create and register the REST service
	go rest.StartOpenchainRESTServer(serverOpenchain, serverDevops)

	rootNode, err := core.GetRootNode()
	if err != nil {
		grpclog.Fatalf("Failed to get peer.discovery.rootnode valey: %s", err)
	}

	t.Logf("Starting peer with id=%s, network id=%s, address=%s, discovery.rootnode=%s, validator=%v",
		peerEndpoint.ID, viper.GetString("peer.networkId"),
		peerEndpoint.Address, rootNode, viper.GetBool("peer.validator.enabled"))

	// Start the grpc server. Done in a goroutine so we can deploy the
	// genesis block if needed.
	serve := make(chan bool)
	go func() {
		grpcServer.Serve(lis)
		serve <- true
	}()

	// Deploy the geneis block if needed.
	if viper.GetBool("peer.validator.enabled") {
		makeGeneisError := genesis.MakeGenesis()
		if makeGeneisError != nil {
			return makeGeneisError
		}
	}

	// Block until grpc server exits
	<-serve

	return nil
}

func stopOpenchain(t *testing.T) {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		t.Log(fmt.Errorf("Error trying to connect to local peer:", err))
		t.Fail()
	}

	t.Log("Stopping peer...")
	serverClient := pb.NewAdminClient(clientConn)

	status, err := serverClient.StopServer(context.Background(), &google_protobuf.Empty{})
	t.Logf("Current status: %s", status)

}

func registerChaincodeSupport(chainname chaincode.ChainName, grpcServer *grpc.Server, secHelper crypto.Peer) {
	//get user mode
	userRunsCC := false
	if viper.GetString("chaincode.mode") == chaincode.DevModeUserRunsChaincode {
		userRunsCC = true
	}

	//get chaincode startup timeout
	tOut, err := strconv.Atoi(viper.GetString("chaincode.startuptimeout"))
	if err != nil { //what went wrong ?
		fmt.Printf("could not retrive timeout var...setting to 5secs\n")
		tOut = 5000
	}
	ccStartupTimeout := time.Duration(tOut) * time.Millisecond

	//(chainname ChainName, getPeerEndpoint func() (*pb.PeerEndpoint, error), userrunsCC bool, ccstartuptimeout time.Duration, secHelper crypto.Peer)
	pb.RegisterChaincodeSupportServer(grpcServer, chaincode.NewChaincodeSupport(chainname, peer.GetPeerEndpoint, userRunsCC, ccStartupTimeout, secHelper))
}
