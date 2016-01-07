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

package obcca

import (
	"testing"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
	"io/ioutil"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"golang.org/x/net/context"
	google_protobuf "google/protobuf"
	
	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus/helper"
	"github.com/openblockchain/obc-peer/openchain/ledger/genesis"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/openblockchain/obc-peer/openchain/rest"
	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	
	"encoding/json"
	//"errors"
	//"runtime"
	//"strings"	
	//"github.com/howeyc/gopass"
	//"github.com/op/go-logging"
	//"github.com/spf13/cobra"
	//"github.com/openblockchain/obc-peer/events/producer"
)

var (
	tca *TCA 
	eca *ECA 
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
	viper.SetConfigName("obcca_test") // name of config file (without extension)
	viper.AddConfigPath("./")         // path to look for the config file in
	viper.AddConfigPath("./..")       // path to look for the config file in
	err := viper.ReadInConfig()       // Find and read the config file
	if err != nil {                   // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}

func TestValidityPeriod(t *testing.T) {
	var updateInterval int64
	updateInterval = 37
	
	// 1. Start TCA and Openchain
	go startServices(t) 
			
	// 2. Obtain the validity period by querying and directly from the ledger	
	validityPeriod_A := queryValidityPeriod(t)
	validityPeriodFromLedger_A := getValidityPeriodFromLedger(t)

	// 3. Wait for the validity period to be updated...
	time.Sleep(time.Second * 40)
	
	// ... and read the values again
	validityPeriod_B := queryValidityPeriod(t)
	validityPeriodFromLedger_B := getValidityPeriodFromLedger(t)

	// 5. Stop TCA and Openchain
	stopServices()
	
		
	// 6. Compare the values
	if validityPeriod_A != validityPeriodFromLedger_A {
		t.Logf("Validity period read from ledger must be equals tothe one obtained by querying the Openchain. Expected: %s, Actual: %s", validityPeriod_A, validityPeriodFromLedger_A)
		t.Fail()
	}
	
	if validityPeriod_B != validityPeriodFromLedger_B {
		t.Logf("Validity period read from ledger must be equals tothe one obtained by querying the Openchain. Expected: %s, Actual: %s", validityPeriod_B, validityPeriodFromLedger_B)
		t.Fail()
	}
	
	if validityPeriod_B - validityPeriod_A != updateInterval {
		t.Logf("Validity period difference must be equal to the update interval. Expected: %s, Actual: %s", updateInterval, validityPeriod_B - validityPeriod_A)
		t.Fail()
	}

	// 7. cleanup tca and openchain folders
	if err := os.RemoveAll(viper.GetString("peer.fileSystemPath")); err != nil {
		t.Logf("Failed removing [%s] [%s]\n", viper.GetString("peer.fileSystemPath"), err)
	}
	if err := os.RemoveAll(".obcca"); err != nil {
		t.Logf("Failed removing [%s] [%s]\n", ".obcca", err)
	}
}

func startServices(t *testing.T) {
	go startTCA()
	err := startOpenchain()
	if(err != nil){
		t.Logf("Error starting Openchain: %s", err)
		t.Fail()
	}
}

func stopServices(){
	stopOpenchain()
	stopTCA()
}

func startTCA() {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)
	
	eca = NewECA()
	defer eca.Close()

	tca = NewTCA(eca)
	defer tca.Close()

	var wg sync.WaitGroup
	eca.Start(&wg)
	tca.Start(&wg)

	wg.Wait()
}

func stopTCA(){
	tca.Stop()
	eca.Stop()
}

func queryValidityPeriod(t *testing.T) int64 {
	url := "github.com/openblockchain/obc-peer/openchain/system_chaincode/validity_period_update"
	version := "0.0.1"
	args := []string{"system.validity.period"}
	
	validityPeriod, err := queryTransaction(url, version, args)
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
	chaincodeID := &pb.ChaincodeID{Url: "github.com/openblockchain/obc-peer/openchain/system_chaincode/validity_period_update", 
		Version: "0.0.1",
	}
		
	cid, _ := getChaincodeID(chaincodeID)
		
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


// getChaincodeID constructs the ID from pb.ChaincodeID; used by handlerMap
func getChaincodeID(cID *pb.ChaincodeID) (string, error) {
	if cID == nil {
		return "", fmt.Errorf("Cannot construct chaincodeID, got nil object")
	}
	var urlLocation string
	if strings.HasPrefix(cID.Url, "http://") {
		urlLocation = cID.Url[7:]
	} else if strings.HasPrefix(cID.Url, "https://") {
		urlLocation = cID.Url[8:]
	} else {
		urlLocation = cID.Url
	}
	return urlLocation + ":" + cID.Version, nil
}

func queryTransaction(url string, version string, args []string) ([]byte, error) {
	
	chaincodeInvocationSpec := createChaincodeInvocationForQuery(args, url, version, "system_chaincode_invoker")

	fmt.Printf("Going to query\n")
	
	response, err := queryChaincode(chaincodeInvocationSpec)
	
	if err != nil {
		return nil, fmt.Errorf("Error querying <%s>: %s", url, err)
	}
	
		
	logger.Info("Successfully invoked validity period update: %s(%s)", url, string(response.Msg))
	
	return response.Msg, nil
}

func queryChaincode(chaincodeInvSpec *pb.ChaincodeInvocationSpec) (*pb.Response, error) {

	devopsClient, err := getDevopsClient(viper.GetString("pki.validity-period.devops-address"))
	if err != nil {
		logger.Error(fmt.Sprintf("Error retrieving devops client: %s", err))
		return nil,err
	}

	resp, err := devopsClient.Query(context.Background(), chaincodeInvSpec)

	if err != nil {
		logger.Error(fmt.Sprintf("Error invoking validity period update system chaincode: %s", err))
		return nil,err
	}
	
	logger.Info("Successfully invoked validity period update: %s(%s)", chaincodeInvSpec, string(resp.Msg))
	
	return resp,nil
}

func createChaincodeInvocationForQuery(arguments []string, chaincodePath string, chaincodeVersion string, token string) *pb.ChaincodeInvocationSpec {
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG, 
		ChaincodeID: &pb.ChaincodeID{Url: chaincodePath, 
			Version: chaincodeVersion,
		}, 
		CtorMsg: &pb.ChaincodeInput{Function: "query", 
			Args: arguments,
		},
	}
	
	spec.SecureContext = string(token)
	
	invocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	
	return invocationSpec
}

func startOpenchain() error {

	peerEndpoint, err := peer.GetPeerEndpoint()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get Peer Endpoint: %s", err))
		return err
	}

	listenAddr := viper.GetString("peer.listenaddress")

	if "" == listenAddr {
		logger.Debug("Listen address not specified, using peer endpoint address")
		listenAddr = peerEndpoint.Address
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}

//	ehubLis, ehubGrpcServer, err := createEventHubServer()
//	if err != nil {
//		grpclog.Fatalf("failed to create ehub server: %v", err)
//	}

	logger.Info("Security enabled status: %t", viper.GetBool("security.enabled"))

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
	//pb.RegisterPeerServer(grpcServer, openchain.NewPeer())
	var peerServer *peer.PeerImpl

	if viper.GetBool("peer.validator.enabled") {
		logger.Debug("Running as validating peer - installing consensus %s", viper.GetString("peer.validator.consensus"))
		peerServer, _ = peer.NewPeerWithHandler(helper.NewConsensusHandler)
	} else {
		logger.Debug("Running as non-validating peer")
		peerServer, _ = peer.NewPeerWithHandler(peer.NewPeerHandler)
	}
	pb.RegisterPeerServer(grpcServer, peerServer)

	// Register the Admin server
	pb.RegisterAdminServer(grpcServer, openchain.NewAdminServer())

	// Register ChaincodeSupport server...
	// TODO : not the "DefaultChain" ... we have to revisit when we do multichain
	registerChaincodeSupport(chaincode.DefaultChain, grpcServer)

	// Register Devops server
	serverDevops := openchain.NewDevopsServer(peerServer)
	pb.RegisterDevopsServer(grpcServer, serverDevops)

	// Register the ServerOpenchain server
	serverOpenchain, err := openchain.NewOpenchainServer()
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating OpenchainServer: %s", err))
		return err
	}

	pb.RegisterOpenchainServer(grpcServer, serverOpenchain)

	// Create and register the REST service
	go rest.StartOpenchainRESTServer(serverOpenchain, serverDevops)

	rootNode, err := openchain.GetRootNode()
	if err != nil {
		grpclog.Fatalf("Failed to get peer.discovery.rootnode valey: %s", err)
	}

	logger.Info("Starting peer with id=%s, network id=%s, address=%s, discovery.rootnode=%s, validator=%v",
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
		makeGeneisError := genesis.MakeGenesis(peerServer.GetSecHelper())
		if makeGeneisError != nil {
			return makeGeneisError
		}
	}

	//start the event hub server
//	if ehubGrpcServer != nil && ehubLis != nil {
//		go ehubGrpcServer.Serve(ehubLis)
//	}

	// Block until grpc server exits
	<-serve

	return nil
}

func stopOpenchain() {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		logger.Error("Error trying to connect to local peer:", err)
		return
	}

	logger.Info("Stopping peer...")
	serverClient := pb.NewAdminClient(clientConn)

	status, err := serverClient.StopServer(context.Background(), &google_protobuf.Empty{})
	logger.Info("Current status: %s", status)

}

func registerChaincodeSupport(chainname chaincode.ChainName, grpcServer *grpc.Server) {
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

	pb.RegisterChaincodeSupportServer(grpcServer, chaincode.NewChaincodeSupport(chainname, peer.GetPeerEndpoint, userRunsCC, ccStartupTimeout))
}