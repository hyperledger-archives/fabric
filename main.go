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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
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
	chaincodeVersion  string
)

var chaincodeCmd = &cobra.Command{
	Use:   chainFuncName,
	Short: fmt.Sprintf("%s specific commands.", chainFuncName),
	Long:  fmt.Sprintf("%s specific commands.", chainFuncName),
}

var chaincodePathArgumentSpecifier = fmt.Sprintf("%s_PATH", strings.ToUpper(chainFuncName))

var chaincodeBuildCmd = &cobra.Command{
	Use:       "build",
	Short:     fmt.Sprintf("Builds the specified %s.", chainFuncName),
	Long:      fmt.Sprintf("Builds the specified %s.", chainFuncName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeBuild(cmd, args)
	},
}

var chaincodeTestCmd = &cobra.Command{
	Use:       fmt.Sprintf("test %s", chaincodePathArgumentSpecifier),
	Short:     fmt.Sprintf("Test the specified %s.", chainFuncName),
	Long:      fmt.Sprintf(`Test the specified %s without persisting state or modifying chain.`, chainFuncName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeTest(cmd, args)
	},
}

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

	viper.BindPFlag("peer_logging_level", flags.Lookup("peer-logging-level"))
	viper.BindPFlag("peer_tls_enabled", flags.Lookup("peer-tls-enabled"))
	viper.BindPFlag("peer_tls_cert_file", flags.Lookup("peer-tls-cert-file"))
	viper.BindPFlag("peer_tls_key_file", flags.Lookup("peer-tls-key-file"))
	viper.BindPFlag("peer_port", flags.Lookup("peer-ports"))
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
		logging.SetLevel(level, "main")
		logging.SetLevel(level, "peer")
		logging.SetLevel(level, "server")
	} else {
		logger.Warning("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("peer.logging.level"), logging.ERROR, err)
		logging.SetLevel(logging.ERROR, chainFuncName)
		logging.SetLevel(logging.ERROR, "consensus")
		logging.SetLevel(logging.ERROR, "main")
		logging.SetLevel(logging.ERROR, "peer")
		logging.SetLevel(logging.ERROR, "server")
	}

	mainCmd.AddCommand(peerCmd)
	mainCmd.AddCommand(statusCmd)
	mainCmd.AddCommand(stopCmd)

	vmCmd.AddCommand(vmPrimeCmd)
	mainCmd.AddCommand(vmCmd)

	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeLang, "lang", "l", "golang", fmt.Sprintf("Language the %s is written in", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeCtorJSON, "ctor", "c", "{}", fmt.Sprintf("Constructor message for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodePath, "path", "p", undefinedParamValue, fmt.Sprintf("Path to %s", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeVersion, "version", "v", undefinedParamValue, fmt.Sprintf("Version for the %s as described at http://semver.org/", chainFuncName))

	chaincodeCmd.AddCommand(chaincodeBuildCmd)
	chaincodeCmd.AddCommand(chaincodeTestCmd)
	chaincodeCmd.AddCommand(chaincodeDeployCmd)
	chaincodeCmd.AddCommand(chaincodeInvokeCmd)
	chaincodeCmd.AddCommand(chaincodeQueryCmd)

	mainCmd.AddCommand(chaincodeCmd)
	mainCmd.Execute()

}

// func serverValidator() error {
// 	lis, err := net.Listen("tcp", viper.GetString("validator.address"))
// 	if err != nil {
// 		grpclog.Fatalf("failed to listen: %v", err)
// 	}
// 	var opts []grpc.ServerOption
// 	if viper.GetBool("validator.tls.enabled") {
// 		creds, err := credentials.NewServerTLSFromFile(viper.GetString("validator.tls.cert.file"), viper.GetString("validator.tls.key.file"))
// 		if err != nil {
// 			grpclog.Fatalf("Failed to generate credentials %v", err)
// 		}
// 		opts = []grpc.ServerOption{grpc.Creds(creds)}
// 	}
// 	grpcServer := grpc.NewServer(opts...)

// 	// Register the Peer server
// 	//pb.RegisterPeerServer(grpcServer, openchain.NewPeer())
// 	var peer *openchain.Peer
// 	if viper.GetBool("peer.consensus.validator.enabled") {
// 		log.Debug("Running as validator")
// 		newValidator := openchain.NewSimpleValidator()
// 		peer, _ = openchain.NewPeerWithHandler(newValidator.GetHandler)
// 	} else {
// 		log.Debug("Running as peer")
// 		peer, _ = openchain.NewPeerWithHandler(func(stream openchain.PeerChatStream) openchain.MessageHandler {
// 			return openchain.NewPeerFSM("", stream)
// 		})
// 		//pb.RegisterPeerServer(grpcServer, peer)
// 	}
// 	pb.RegisterPeerServer(grpcServer, peer)

// 	rootNode, err := openchain.GetRootNode()
// 	if err != nil {
// 		grpclog.Fatalf("Failed to get peer.discovery.rootnode valey: %s", err)
// 	}
// 	log.Info("Starting validator with id=%s, network id=%s, address=%s, discovery.rootnode=%s, validator=%v", viper.GetString("peer.id"), viper.GetString("peer.networkId"), viper.GetString("peer.address"), rootNode, viper.GetBool("peer.consensus.validator.enabled"))
// 	go grpcServer.Serve(lis)
// 	return nil
// }

func serve(args []string) error {

	// if viper.GetBool("validator.enabled") {
	// 	serverValidator()
	// }
	lis, err := net.Listen("tcp", viper.GetString("peer.address"))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
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
	var peerServer *peer.Peer
	if viper.GetBool("peer.consensus.noops") {
		logger.Debug("Running in NOOPS mode!!!!!!")
		peerServer, _ = peer.NewPeerWithHandler(peer.NewNoopsHandler)
	} else if viper.GetBool("peer.consensus.validator.enabled") {
		logger.Debug("Running as validator")
		newValidator, err := peer.NewSimpleValidator(viper.GetBool("peer.consensus.leader.enabled"))
		if err != nil {
			return fmt.Errorf("Error creating simple Validator: %s", err)
		}
		peerServer, _ = peer.NewPeerWithHandler(newValidator.GetHandler)
	} else {
		logger.Debug("Running as peer")
		// peerServer, _ = peer.NewPeerWithHandler(func(p *peer.Peer, stream peer.PeerChatStream) peer.MessageHandler {
		// 	return peer.NewPeerHandlerTo("", stream)
		// })
		peerServer, _ = peer.NewPeerWithHandler(peer.NewPeerHandler)
		//pb.RegisterPeerServer(grpcServer, peer)
	}
	pb.RegisterPeerServer(grpcServer, peerServer)

	// Register the Admin server
	pb.RegisterAdminServer(grpcServer, openchain.NewAdminServer())

	// Register ChainletSupport server
	pb.RegisterChainletSupportServer(grpcServer, chaincode.NewChainletSupport())

	// Register Devops server
	serverDevops := openchain.NewDevopsServer()
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
	peerEndpoint, err := peer.GetPeerEndpoint()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get Peer Endpoint: %s", err))
		return err
	}
	logger.Info("Starting peer with id=%s, network id=%s, address=%s, discovery.rootnode=%s, validator=%v", peerEndpoint.ID, viper.GetString("peer.networkId"), peerEndpoint.Address, rootNode, viper.GetBool("peer.consensus.validator.enabled"))
	if len(args) > 0 && args[0] == "cli" {
		go doCLI()
	}
	grpcServer.Serve(lis)
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

func checkChaincodeCmdParams(cmd *cobra.Command) error {

	if chaincodeVersion == undefinedParamValue {
		err := fmt.Sprintf("Error: must supply value for %s version parameter.\n", chainFuncName)
		cmd.Out().Write([]byte(err))
		cmd.Usage()
		return errors.New(err)
	}

	if chaincodePath == undefinedParamValue {
		err := fmt.Sprintf("Error: must supply value for %s path parameter.\n", chainFuncName)
		cmd.Out().Write([]byte(err))
		cmd.Usage()
		return errors.New(err)
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

func chaincodeBuild(cmd *cobra.Command, args []string) {
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
	spec := &pb.ChainletSpec{Type: pb.ChainletSpec_GOLANG,
		ChainletID: &pb.ChainletID{Url: chaincodePath, Version: chaincodeVersion}}

	chainletDeploymentSpec, err := devopsClient.Build(context.Background(), spec)
	if err != nil {
		errMsg := fmt.Sprintf("Error building %s: %s\n", chainFuncName, err)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return
	}
	logger.Info("Build result: %s", chainletDeploymentSpec.ChainletSpec)
}

func chaincodeTest(cmd *cobra.Command, args []string) error {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		return err
	}
	cmd.Out().Write([]byte(fmt.Sprintf("going to test %s: %s\n", chainFuncName, chaincodePath)))
	return nil
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
	spec := &pb.ChainletSpec{Type: pb.ChainletSpec_GOLANG,
		ChainletID: &pb.ChainletID{Url: chaincodePath, Version: chaincodeVersion}, CtorMsg: input}

	chainletDeploymentSpec, err := devopsClient.Deploy(context.Background(), spec)
	if err != nil {
		errMsg := fmt.Sprintf("Error building %s: %s\n", chainFuncName, err)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return
	}
	logger.Info("Build result: %s", chainletDeploymentSpec.ChainletSpec)
}

func chaincodeInvoke(cmd *cobra.Command, args []string) {
	chaincodeInvokeOrQuery(cmd, args, true)
}

func chaincodeQuery(cmd *cobra.Command, args []string) {
	chaincodeInvokeOrQuery(cmd, args, false)
}

func chaincodeInvokeOrQuery(cmd *cobra.Command, args []string, invoke bool) {
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
	spec := &pb.ChainletSpec{Type: pb.ChainletSpec_GOLANG,
		ChainletID: &pb.ChainletID{Url: chaincodePath, Version: chaincodeVersion}, CtorMsg: input}

	//msg := &pb.ChaincodeInput{Function: "func1", Args: []string{"arg1", "arg2"}}

	// Build the ChaincodeInvocationSpec message
	invocation := &pb.ChaincodeInvocationSpec{ChainletSpec: spec}

	var resp *pb.DevopsResponse
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
		logger.Info("Succesfuly invoked transaction: %s(%s)", invocation, string(resp.Msg))
	} else {
		logger.Info("Succesfuly queried transaction: %s", invocation)
		if err != nil {
			fmt.Printf("Error running query : %s\n", err)
		} else if resp != nil {
			logger.Info("Trying to print as string: %s", string(resp.Msg))
			logger.Info("Raw bytes %x", resp.Msg)
		}
	}
}

func doCLI() {
	wio := bufio.NewWriter(os.Stdout)
	rio := bufio.NewReader(os.Stdin)
	help := "deploy <chaincodeid> <version> <funcname> <arg> <arg>...\n"
	for {
		wio.WriteString(">")
		wio.Flush()
		line, _ := rio.ReadString('\n')
		if line == "q\n" {
			break
		}
		toks := strings.Split(line, "\n") //get rid of /n
		toks = strings.Split(toks[0], " ")
		if toks == nil || len(toks) < 1 {
			wio.WriteString("invalid input " + line + "\n")
			continue
		}
		if toks[0] == "help" {
			wio.WriteString(help)
			continue
		}
		switch toks[0] {
		case "deploy":
			fallthrough
		case "d":
			if len(toks) < 5 {
				wio.WriteString(help)
				continue
			}
			chainletID := toks[1]
			version := toks[2]
			spec := &pb.ChainletSpec{Type: 1, ChainletID: &pb.ChainletID{Url: chainletID, Version: version}, CtorMsg: &pb.ChaincodeInput{Function: toks[3], Args: toks[4:]}}
			_, err := openchain.DeployLocal(context.Background(), spec)
			if err != nil {
				wio.WriteString("Error transacting : " + fmt.Sprintf("%s", err) + "\n")
			} else {
				wio.WriteString("Success\n")
			}
		default:
			wio.WriteString(help)
		}
	}
}
