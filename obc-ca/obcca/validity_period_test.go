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
	//"encoding/json"
	//"errors"
	"fmt"
	//"io/ioutil"
	"net"
	"os"
	//"runtime"
	"strconv"
	//"strings"
	"time"
	
	//"golang.org/x/net/context"

	//google_protobuf "google/protobuf"

	//"github.com/howeyc/gopass"
	//"github.com/op/go-logging"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	//"github.com/spf13/cobra"
	//"github.com/openblockchain/obc-peer/events/producer"
	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus/helper"
	"github.com/openblockchain/obc-peer/openchain/ledger/genesis"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/openblockchain/obc-peer/openchain/rest"
	pb "github.com/openblockchain/obc-peer/protos"
	
	"sync"
	"io/ioutil"
)

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func setupTestConfig() {
	viper.AutomaticEnv()
	viper.SetConfigName("obcca_test") // name of config file (without extension)
	viper.AddConfigPath("./")        // path to look for the config file in
	viper.AddConfigPath("./..")      // path to look for the config file in
	viper.AddConfigPath("./../..")      // path to look for the config file in
	err := viper.ReadInConfig()      // Find and read the config file
	if err != nil {                  // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	
	chaincodeDevMode := false
	if chaincodeDevMode {
		logger.Info("Running in chaincode development mode. Set consensus to NOOPS and user starts chaincode")
		viper.Set("peer.validator.enabled", true)
		viper.Set("peer.validator.consensus", "noops")
		viper.Set("chaincode.mode", chaincode.DevModeUserRunsChaincode)
	}
	
	/*viper.Set("peer.fileSystemPath", "/var/openchain/test")
	viper.Set("security.enabled", true)
	viper.Set("ports.ecaP", ":50051")
	viper.Set("ports.ecaA", ":50052")
	viper.Set("ports.tcaP", ":50551")
	viper.Set("ports.tcaA", ":50552")
	viper.Set("ports.tlscaP", ":50951")
	viper.Set("ports.tlscaA", ":50952")
	viper.Set("hosts.eca", "localhost")
	viper.Set("hosts.tca", "localhost")
	viper.Set("hosts.tlsca", "localhost")
	viper.Set("eca.users.nepumuk", "9gvZQRwhUq9q")
	viper.Set("eca.users.jim", "AwbeJH2kw9qK")
	viper.Set("eca.users.lukas", "NPKYL39uKbkj")
	viper.Set("eca.users.system_chaincode_invoker", "DRJ20pEql15a")
	viper.Set("pki.validity-period.update", true)
	viper.Set("pki.validity-period.tls.enabled", false)
	viper.Set("pki.validity-period.tls.cert.file", "testdata/server1.pem")
	viper.Set("pki.validity-period.tls.key.file", "testdata/server1.key")
	viper.Set("pki.validity-period.devops-address", "0.0.0.0:30303")*/
}

func TestValidityPeriod(t *testing.T) {
	
	startTCA()

	err := startOpenchain()
	if(err != nil){
		t.Logf("Error starting Openchain: %s", err)
		t.Fail()
	}
}

func startTCA() {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)
	
	eca := NewECA()
	defer eca.Close()

	tca := NewTCA(eca)
	defer tca.Close()

	var wg sync.WaitGroup
	eca.Start(&wg)
	tca.Start(&wg)

	//wg.Wait()
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