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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	"github.com/howeyc/gopass"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/openblockchain/obc-peer/events/producer"
	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus/helper"
	"github.com/openblockchain/obc-peer/openchain/ledger/genesis"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/openblockchain/obc-peer/openchain/rest"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("main")

// Constants go here.
const chainFuncName = "chaincode"
const cmdRoot = "openchain"
const undefinedParamValue = ""

// The main command describes the service and
// defaults to printing the help message.
var mainCmd = &cobra.Command{
	Use: cmdRoot,
}

var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Run openchain peer.",
	Long:  `Runs the openchain peer that interacts with the openchain network.`,
	Run: func(cmd *cobra.Command, args []string) {
		serve(args)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Status of the openchain peer.",
	Long:  `Outputs the status of the currently running openchain peer.`,
	Run: func(cmd *cobra.Command, args []string) {
		status()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop openchain peer.",
	Long:  `Stops the currently running openchain Peer, disconnecting from the openchain network.`,
	Run: func(cmd *cobra.Command, args []string) {
		stop()
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login user on CLI.",
	Long:  `Login the local user on CLI. Must supply username parameter.`,
	Run: func(cmd *cobra.Command, args []string) {
		login(args)
	},
}

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "VM functionality of openchain.",
	Long:  `Interact with the VM functionality of openchain.`,
}

var vmPrimeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Prime the VM functionality of openchain.",
	Long:  `Primes the VM functionality of openchain by preparing the necessary VM construction artifacts.`,
	Run: func(cmd *cobra.Command, args []string) {
		stop()
	},
}

// Chaincode-related variables.
var (
	chaincodeLang     string
	chaincodeCtorJSON string
	chaincodePath     string
	chaincodeName     string
	chaincodeDevMode  bool
	chaincodeUsr      string
)

var chaincodeCmd = &cobra.Command{
	Use:   chainFuncName,
	Short: fmt.Sprintf("%s specific commands.", chainFuncName),
	Long:  fmt.Sprintf("%s specific commands.", chainFuncName),
}

var chaincodePathArgumentSpecifier = fmt.Sprintf("%s_PATH", strings.ToUpper(chainFuncName))

var chaincodeDeployCmd = &cobra.Command{
	Use:       "deploy",
	Short:     fmt.Sprintf("Deploy the specified %s to the network.", chainFuncName),
	Long:      fmt.Sprintf(`Deploy the specified %s to the network.`, chainFuncName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeDeploy(cmd, args)
	},
}

var chaincodeInvokeCmd = &cobra.Command{
	Use:       "invoke",
	Short:     fmt.Sprintf("Invoke the specified %s.", chainFuncName),
	Long:      fmt.Sprintf(`Invoke the specified %s.`, chainFuncName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeInvoke(cmd, args)
	},
}

var chaincodeQueryCmd = &cobra.Command{
	Use:       "query",
	Short:     fmt.Sprintf("Query using the specified %s.", chainFuncName),
	Long:      fmt.Sprintf(`Query using the specified %s.`, chainFuncName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeQuery(cmd, args)
	},
}

func main() {

	runtime.GOMAXPROCS(2)

	// For environment variables.
	viper.SetEnvPrefix(cmdRoot)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	// Set the flags on the server command.
	flags := peerCmd.Flags()
	flags.String("peer-logging-level", "error", "Logging level, can be one of [CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG]")
	flags.Bool("peer-tls-enabled", false, "Connection uses TLS if true, else plain TCP")
	flags.String("peer-tls-cert-file", "testdata/server1.pem", "TLS cert file")
	flags.String("peer-tls-key-file", "testdata/server1.key", "TLS key file")
	flags.Int("peer-port", 30303, "Port this peer listens to for incoming connections")
	flags.Int("peer-gomaxprocs", 2, "The maximum number threads excuting peer code")
	flags.Bool("peer-discovery-enabled", true, "Whether peer discovery is enabled")

	flags.BoolVarP(&chaincodeDevMode, "peer-chaincodedev", "", false, "Whether peer in chaincode development mode")

	viper.BindPFlag("peer_logging_level", flags.Lookup("peer-logging-level"))
	viper.BindPFlag("peer_tls_enabled", flags.Lookup("peer-tls-enabled"))
	viper.BindPFlag("peer_tls_cert_file", flags.Lookup("peer-tls-cert-file"))
	viper.BindPFlag("peer_tls_key_file", flags.Lookup("peer-tls-key-file"))
	viper.BindPFlag("peer_port", flags.Lookup("peer-port"))
	viper.BindPFlag("peer_gomaxprocs", flags.Lookup("peer-gomaxprocs"))
	viper.BindPFlag("peer_discovery_enabled", flags.Lookup("peer-discovery-enabled"))

	// Now set the configuration file.
	viper.SetConfigName(cmdRoot) // Name of config file (without extension)
	viper.AddConfigPath("./")    // Path to look for the config file in
	err := viper.ReadInConfig()  // Find and read the config file
	if err != nil {              // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error when reading %s config file: %s \n", cmdRoot, err))
	}

	level, err := logging.LogLevel(viper.GetString("peer.logging.level"))
	if err == nil {
		// No error, use the setting.
		logging.SetLevel(level, chainFuncName)
		logging.SetLevel(level, "consensus")
		logging.SetLevel(level, "consensus/controller")
		logging.SetLevel(level, "consensus/helper")
		logging.SetLevel(level, "consensus/noops")
		logging.SetLevel(level, "consensus/pbft")
		logging.SetLevel(level, "main")
		logging.SetLevel(level, "peer")
		logging.SetLevel(level, "server")
	} else {
		logger.Warning("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("peer.logging.level"), logging.ERROR, err)
		logging.SetLevel(logging.ERROR, chainFuncName)
		logging.SetLevel(logging.ERROR, "consensus")
		logging.SetLevel(logging.ERROR, "consensus/controller")
		logging.SetLevel(logging.ERROR, "consensus/helper")
		logging.SetLevel(logging.ERROR, "consensus/noops")
		logging.SetLevel(logging.ERROR, "consensus/pbft")
		logging.SetLevel(logging.ERROR, "main")
		logging.SetLevel(logging.ERROR, "peer")
		logging.SetLevel(logging.ERROR, "server")
	}

	mainCmd.AddCommand(peerCmd)
	mainCmd.AddCommand(statusCmd)
	mainCmd.AddCommand(stopCmd)
	mainCmd.AddCommand(loginCmd)

	vmCmd.AddCommand(vmPrimeCmd)
	mainCmd.AddCommand(vmCmd)

	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeLang, "lang", "l", "golang", fmt.Sprintf("Language the %s is written in", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeCtorJSON, "ctor", "c", "{}", fmt.Sprintf("Constructor message for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodePath, "path", "p", undefinedParamValue, fmt.Sprintf("Path to %s", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeName, "name", "n", undefinedParamValue, fmt.Sprintf("Name of the chaincode returned by the deploy transaction"))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeUsr, "username", "u", undefinedParamValue, fmt.Sprintf("Username for chaincode operations when security is enabled"))

	chaincodeCmd.AddCommand(chaincodeDeployCmd)
	chaincodeCmd.AddCommand(chaincodeInvokeCmd)
	chaincodeCmd.AddCommand(chaincodeQueryCmd)

	mainCmd.AddCommand(chaincodeCmd)
	mainCmd.Execute()

}

func createEventHubServer() (net.Listener, *grpc.Server, error) {
	var lis net.Listener
	var grpcServer *grpc.Server
	var err error
	if viper.GetBool("peer.validator.enabled") {
		lis, err = net.Listen("tcp", viper.GetString("peer.validator.events.address"))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to listen: %v", err)
		}

		//TODO - do we need different SSL material for events ?
		var opts []grpc.ServerOption
		if viper.GetBool("peer.tls.enabled") {
			creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to generate credentials %v", err)
			}
			opts = []grpc.ServerOption{grpc.Creds(creds)}
		}

		grpcServer = grpc.NewServer(opts...)
		ehServer := producer.NewOpenchainEventsServer(uint(viper.GetInt("peer.validator.events.buffersize")), viper.GetInt("peer.validator.events.timeout"))
		pb.RegisterOpenchainEventsServer(grpcServer, ehServer)
	}
	return lis, grpcServer, err
}

func serve(args []string) error {

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

	ehubLis, ehubGrpcServer, err := createEventHubServer()
	if err != nil {
		grpclog.Fatalf("failed to create ehub server: %v", err)
	}

	if chaincodeDevMode {
		logger.Info("Running in chaincode development mode. Set consensus to NOOPS and user starts chaincode")
		viper.Set("peer.validator.enabled", "true")
		viper.Set("peer.validator.consensus", "noops")
		viper.Set("chaincode.mode", chaincode.DevModeUserRunsChaincode)
	}
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

	// Create and register the REST service if configured
	if viper.GetBool("rest.enabled") {
		go rest.StartOpenchainRESTServer(serverOpenchain, serverDevops)
	}

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
	if ehubGrpcServer != nil && ehubLis != nil {
		go ehubGrpcServer.Serve(ehubLis)
	}

	// Block until grpc server exits
	<-serve

	return nil
}

func status() {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		logger.Error("Error trying to connect to local peer:", err)
	}

	serverClient := pb.NewAdminClient(clientConn)

	status, err := serverClient.GetStatus(context.Background(), &google_protobuf.Empty{})
	logger.Info("Current status: %s", status)
}

func stop() {
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

// login confirms the enrollmentID and secret password of the client with the
// CA and stores the enrollment certificate and key in the Devops server.
func login(args []string) {
	logger.Info("CLI client login...")

	// Check for username argument
	if len(args) == 0 {
		logger.Error("Error: must supply username.\n")
		return
	}

	// Check for other extraneous arguments
	if len(args) != 1 {
		logger.Error("Error: must supply username as the 1st and only parameter.\n")
		return
	}

	// Retrieve the CLI data storage path
	// Returns /var/openchain/production/client/
	localStore := getCliFilePath()
	logger.Info("Local data store for client loginToken: %s", localStore)

	// If the user is already logged in, return
	if _, err := os.Stat(localStore + "loginToken_" + args[0]); err == nil {
		logger.Info("User '%s' is already logged in.\n", args[0])
		return
	}

	// User is not logged in, prompt for password
	fmt.Printf("Enter password for user '%s': ", args[0])
	pw := gopass.GetPasswdMasked()

	// Log in the user
	logger.Info("Logging in user '%s' on CLI interface...\n", args[0])

	// Get a devopsClient to perform the login
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		logger.Error(fmt.Sprintf("Error trying to connect to local peer: %s", err))
		return
	}
	devopsClient := pb.NewDevopsClient(clientConn)

	// Build the login spec and login
	loginSpec := &pb.Secret{EnrollId: args[0], EnrollSecret: string(pw)}
	loginResult, err := devopsClient.Login(context.Background(), loginSpec)

	// Check if login is successful
	if loginResult.Status == pb.Response_SUCCESS {
		// If /var/openchain/production/client/ directory does not exist, create it
		if _, err := os.Stat(localStore); err != nil {
			if os.IsNotExist(err) {
				// Directory does not exist, create it
				if err := os.Mkdir(localStore, 0755); err != nil {
					panic(fmt.Errorf("Fatal error when creating %s directory: %s\n", localStore, err))
				}
			} else {
				// Unexpected error
				panic(fmt.Errorf("Fatal error on os.Stat of %s directory: %s\n", localStore, err))
			}
		}

		// Store client security context into a file
		logger.Info("Storing login token for user '%s'.\n", args[0])
		err = ioutil.WriteFile(localStore+"loginToken_"+args[0], []byte(args[0]), 0755)
		if err != nil {
			panic(fmt.Errorf("Fatal error when storing client login token: %s\n", err))
		}

		logger.Info("Login successful for user '%s'.\n", args[0])
	} else {
		logger.Error(fmt.Sprintf("Error on client login: %s", string(loginResult.Msg)))
	}

	return
}

// getCliFilePath is a helper function to retrieve the local storage directory
// of client login tokens.
func getCliFilePath() string {
	localStore := viper.GetString("peer.fileSystemPath")
	if !strings.HasSuffix(localStore, "/") {
		localStore = localStore + "/"
	}
	localStore = localStore + "client/"
	return localStore
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

func checkChaincodeCmdParams(cmd *cobra.Command) error {

	if chaincodeName == undefinedParamValue {
		if chaincodePath == undefinedParamValue {
			err := fmt.Sprintf("Error: must supply value for %s path parameter.\n", chainFuncName)
			cmd.Out().Write([]byte(err))
			cmd.Usage()
			return errors.New(err)
		}
	}

	if chaincodeCtorJSON != "{}" {
		// Check to ensure the JSON has "function" and "args" keys
		input := &pb.ChaincodeMessage{}
		jsonerr := json.Unmarshal([]byte(chaincodeCtorJSON), &input)
		if jsonerr != nil {
			err := fmt.Sprintf("Error: must supply 'function' and 'args' keys in %s constructor parameter.\n", chainFuncName)
			cmd.Out().Write([]byte(err))
			cmd.Usage()
			return errors.New(err)
		}
	}

	return nil
}

func getDevopsClient(cmd *cobra.Command) (pb.DevopsClient, error) {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		return nil, fmt.Errorf("Error trying to connect to local peer: %s", err)
	}
	devopsClient := pb.NewDevopsClient(clientConn)
	return devopsClient, nil
}

func chaincodeDeploy(cmd *cobra.Command, args []string) {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		logger.Error(fmt.Sprintf("Error building %s: %s", chainFuncName, err))
		return
	}
	devopsClient, err := getDevopsClient(cmd)
	if err != nil {
		logger.Error(fmt.Sprintf("Error building %s: %s", chainFuncName, err))
		return
	}
	// Build the spec
	input := &pb.ChaincodeInput{}
	if jsonerr := json.Unmarshal([]byte(chaincodeCtorJSON), &input); jsonerr != nil {
		logger.Error(fmt.Sprintf("Error building %s: %s", chainFuncName, err))
		return
	}
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Path: chaincodePath, Name: chaincodeName}, CtorMsg: input}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		if chaincodeUsr == undefinedParamValue {
			err := fmt.Sprintf("Error: must supply username for chaincode when security is enabled.\n")
			cmd.Out().Write([]byte(err))
			cmd.Usage()
			return
		}

		// Retrieve the CLI data storage path
		// Returns /var/openchain/production/client/
		localStore := getCliFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				logger.Error("Error: User not logged in. Use the 'login' command to obtain a security token.\n")
				return
			}
			// Unexpected error
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// If privacy is enabled, mark chaincode as confidential
	if viper.GetBool("security.privacy") {
		spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
	}

	chaincodeDeploymentSpec, err := devopsClient.Deploy(context.Background(), spec)
	if err != nil {
		errMsg := fmt.Sprintf("Error building %s: %s\n", chainFuncName, err)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return
	}
	logger.Info("Deploy result: %s", chaincodeDeploymentSpec.ChaincodeSpec)
}

func chaincodeInvoke(cmd *cobra.Command, args []string) {
	chaincodeInvokeOrQuery(cmd, args, true)
}

func chaincodeQuery(cmd *cobra.Command, args []string) {
	chaincodeInvokeOrQuery(cmd, args, false)
}

func chaincodeInvokeOrQuery(cmd *cobra.Command, args []string, invoke bool) {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		logger.Error(fmt.Sprintf("Error invoking %s: %s", chainFuncName, err))
		return
	}

	if chaincodeName == "" {
		logger.Error(fmt.Sprintf("Name not given for invoke/query"))
		return
	}

	devopsClient, err := getDevopsClient(cmd)
	if err != nil {
		logger.Error(fmt.Sprintf("Error building %s: %s", chainFuncName, err))
		return
	}
	// Build the spec
	input := &pb.ChaincodeInput{}
	if jsonerr := json.Unmarshal([]byte(chaincodeCtorJSON), &input); jsonerr != nil {
		logger.Error(fmt.Sprintf("Error building %s: %s", chainFuncName, err))
		return
	}
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Name: chaincodeName}, CtorMsg: input}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		if chaincodeUsr == undefinedParamValue {
			err := fmt.Sprintf("Error: must supply username for chaincode when security is enabled.\n")
			cmd.Out().Write([]byte(err))
			cmd.Usage()
			return
		}

		// Retrieve the CLI data storage path
		// Returns /var/openchain/production/client/
		localStore := getCliFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				logger.Error("Error: User not logged in. Use the 'login' command to obtain a security token.\n")
				return
			}
			// Unexpected error
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// If privacy is enabled, mark chaincode as confidential
	if viper.GetBool("security.privacy") {
		spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
	}

	// Build the ChaincodeInvocationSpec message
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	var resp *pb.Response
	if invoke {
		resp, err = devopsClient.Invoke(context.Background(), invocation)
	} else {
		resp, err = devopsClient.Query(context.Background(), invocation)
	}

	if err != nil {
		var errMsg string
		if invoke {
			errMsg = fmt.Sprintf("Error invoking %s: %s\n", chainFuncName, err)
		} else {
			errMsg = fmt.Sprintf("Error querying %s: %s\n", chainFuncName, err)
		}
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return
	}
	if invoke {
		logger.Info("Successfully invoked transaction: %s(%s)", invocation, string(resp.Msg))
	} else {
		logger.Info("Successfully queried transaction: %s", invocation)
		if err != nil {
			fmt.Printf("Error running query : %s\n", err)
		} else if resp != nil {
			logger.Info("Trying to print as string: %s", string(resp.Msg))
			logger.Info("Raw bytes %x", resp.Msg)
		}
	}
}
