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
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
	
	//"encoding/json"
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
	go startServices(t) 
	
	// 1. query the validity period
	// 2. wait at least the validity period update time
	// 3. query the validity period again and compare with the previous value, it must be greater
	
	time.Sleep(time.Second * 60) // TODO remove when the test is complete
	
	stopServices()
	
	// 4. cleanup database and test folder
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
	
	
	//exampleQueryTransaction(context.Background(),"github.com/openblockchain/obc-peer/openchain/system_chaincode/validity_period_update", "0.0.1", []string{})

	wg.Wait()
}

func stopTCA(){
	tca.Stop()
	eca.Stop()
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



func exampleQueryTransaction(ctxt context.Context, url string, version string, args []string) error {
	
	chaincodeID, _ := getChaincodeID(&pb.ChaincodeID{Url: url, Version: version})
	

	fmt.Printf("Going to query\n")
	f := "query"
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Url: chaincodePath, 
			Version: chaincodeVersion,
		},
		CtorMsg: &pb.ChaincodeInput{Function: f, Args: args},
		}
	
	
	uuid, _, err := invoke(ctxt, spec, pb.Transaction_CHAINCODE_QUERY)
	
	fmt.Println(uuid)
	if err != nil {
		return fmt.Errorf("Error querying <%s>: %s", chaincodeID, err)
	}
	
	return nil
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
		return uuid, nil, fmt.Errorf("Error invoking chaincode: %s ", err)
	}
	retval, err := chaincode.Execute(ctx, chaincode.GetChain("default"), transaction, nil)
	return uuid, retval, err
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