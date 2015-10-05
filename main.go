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
	"errors"
	"fmt"
	"net"
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
	pb "github.com/openblockchain/obc-peer/protos"
)

var log = logging.MustGetLogger("main")

const ChainFuncFormalName = "chaincode"

const RootOfCommand = "openchain"

// The main command describes the service and defaults to printing the
// help message.
var mainCmd = &cobra.Command{
	Use: RootOfCommand,
}

var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Run Openchain peer.",
	Long:  `Runs the Openchain peer that interacts with the openchain network.`,
	Run: func(cmd *cobra.Command, args []string) {
		serve()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Status of the Openchain peer.",
	Long:  `Outputs the status of the currently running openchain Peer.`,
	Run: func(cmd *cobra.Command, args []string) {
		status()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops the Openchain peer.",
	Long:  `Stops the currently running openchain Peer, disconnecting from the openchain network.`,
	Run: func(cmd *cobra.Command, args []string) {
		stop()
	},
}

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "VM functionality of openchain.",
	Long:  `interact with the VM functionality of openchain.`,
}

var vmPrimeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Prime the VM functionality of openchain.",
	Long:  `Primes the VM functionality of openchain by preparing the necessary VM construction artifacts.`,
	Run: func(cmd *cobra.Command, args []string) {
		stop()
	},
}

var (
	chaincodeLang     string
	chaincodeCtorJson string
	chaincodePath     string
	chaincodeVersion  string
)

const undefinedParamValue = ""

var chaincodeCmd = &cobra.Command{
	Use:   ChainFuncFormalName,
	Short: fmt.Sprintf("%s specific commands.", ChainFuncFormalName),
	Long:  fmt.Sprintf("%s specific commands.", ChainFuncFormalName),
}

var chaincodePathArgumentSpecifier = fmt.Sprintf("%s_PATH", strings.ToUpper(ChainFuncFormalName))

var chaincodeBuildCmd = &cobra.Command{
	Use:       "build",
	Short:     fmt.Sprintf("Builds the specified %s.", ChainFuncFormalName),
	Long:      fmt.Sprintf("Builds the specified %s.", ChainFuncFormalName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeBuild(cmd, args)
	},
}

var chaincodeTestCmd = &cobra.Command{
	Use:       fmt.Sprintf("test %s", chaincodePathArgumentSpecifier),
	Short:     fmt.Sprintf("Test the specified %s.", ChainFuncFormalName),
	Long:      fmt.Sprintf(`Will test the specified %s without persisting state or modifying chain.`, ChainFuncFormalName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeTest(cmd, args)
	},
}

var chaincodeDeployCmd = &cobra.Command{
	Use:       "deploy",
	Short:     fmt.Sprintf("Deploy the specified %s to the network.", ChainFuncFormalName),
	Long:      fmt.Sprintf(`Will deploy the specified %s to the network.`, ChainFuncFormalName),
	ValidArgs: []string{"1"},
	Run: func(cmd *cobra.Command, args []string) {
		chaincodeDeploy(cmd, args)
	},
}

func main() {

	runtime.GOMAXPROCS(2)
	viper.SetEnvPrefix(RootOfCommand)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	// Set the flags on the server command
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

	// Now set the configuration file
	viper.SetConfigName(RootOfCommand) // name of config file (without extension)
	viper.AddConfigPath("./")          // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	level, err := logging.LogLevel(viper.GetString("peer.logging.level"))
	if err == nil {
		// No error, use the setting
		logging.SetLevel(level, ChainFuncFormalName)
		logging.SetLevel(level, "main")
		logging.SetLevel(level, "server")
		logging.SetLevel(level, "peer")
	} else {
		log.Warning("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("peer.logging.level"), logging.ERROR, err)
		logging.SetLevel(logging.ERROR, ChainFuncFormalName)
		logging.SetLevel(logging.ERROR, "main")
		logging.SetLevel(logging.ERROR, "server")
		logging.SetLevel(logging.ERROR, "peer")
	}

	mainCmd.AddCommand(peerCmd)
	mainCmd.AddCommand(statusCmd)
	mainCmd.AddCommand(stopCmd)

	vmCmd.AddCommand(vmPrimeCmd)
	mainCmd.AddCommand(vmCmd)

	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeLang, "lang", "l", "golang", fmt.Sprintf("Language the %s is written in", ChainFuncFormalName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeCtorJson, "ctor", "c", "{}", fmt.Sprintf("Constructor message for the %s in JSON format", ChainFuncFormalName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodePath, "path", "p", undefinedParamValue, fmt.Sprintf("Path to %s", ChainFuncFormalName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeVersion, "version", "v", undefinedParamValue, fmt.Sprintf("Version for the %s as described at http://semver.org/", ChainFuncFormalName))

	chaincodeCmd.AddCommand(chaincodeBuildCmd)
	chaincodeCmd.AddCommand(chaincodeTestCmd)
	chaincodeCmd.AddCommand(chaincodeDeployCmd)
	mainCmd.AddCommand(chaincodeCmd)
	mainCmd.Execute()

	// Leaving to demonstrate how to get a sub-object in YAML
	// testList := viper.Get("peer.discovery.testNodes")
	// log.Info("Peer discovery test Nodes: %v", testList)

}

func serve() error {
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
	pb.RegisterPeerServer(grpcServer, openchain.NewPeer())

	// Register the Admin server
	pb.RegisterAdminServer(grpcServer, openchain.NewAdminServer())

	// Regist ChainletSupport server
	pb.RegisterChainletSupportServer(grpcServer, openchain.NewChainletSupport())

	// Regist Devops server
	pb.RegisterDevopsServer(grpcServer, openchain.NewDevopsServer())

	log.Info("Starting peer with id=%s, network id=%s, address=%s", viper.GetString("peer.id"), viper.GetString("peer.networkId"), viper.GetString("peer.address"))
	grpcServer.Serve(lis)
	return nil
}

func status() {
	clientConn, err := openchain.NewPeerClientConnection()
	if err != nil {
		log.Error("Error trying to connect to local peer:", err)
	}

	serverClient := pb.NewAdminClient(clientConn)

	status, err := serverClient.GetStatus(context.Background(), &google_protobuf.Empty{})
	log.Info("Current status: %s", status)
}

func stop() {
	clientConn, err := openchain.NewPeerClientConnection()
	if err != nil {
		log.Error("Error trying to connect to local peer:", err)
		return
	}

	log.Info("Stopping peer...")
	serverClient := pb.NewAdminClient(clientConn)

	status, err := serverClient.StopServer(context.Background(), &google_protobuf.Empty{})
	log.Info("Current status: %s", status)

}

func checkChaincodeCmdParams(cmd *cobra.Command) error {
	if chaincodeVersion == undefinedParamValue {
		errMsg := fmt.Sprintf("Error:  must supply value for %s version parameter\n", ChainFuncFormalName)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return errors.New(errMsg)
	}
	if chaincodePath == undefinedParamValue {
		errMsg := fmt.Sprintf("Error:  must supply value for %s path parameter\n", ChainFuncFormalName)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return errors.New(errMsg)
	}
	return nil
}

func getDevopsClient(cmd *cobra.Command) (pb.DevopsClient, error) {
	clientConn, err := openchain.NewPeerClientConnection()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error trying to connect to local peer: %s", err))
	}
	devopsClient := pb.NewDevopsClient(clientConn)
	return devopsClient, nil
}

func chaincodeBuild(cmd *cobra.Command, args []string) {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		log.Error(fmt.Sprintf("Error building %s: %s", ChainFuncFormalName, err))
		return
	}
	devopsClient, err := getDevopsClient(cmd)
	if err != nil {
		log.Error(fmt.Sprintf("Error building %s: %s", ChainFuncFormalName, err))
		return
	}
	// Build the spec
	spec := &pb.ChainletSpec{Type: pb.ChainletSpec_GOLANG,
		ChainletID: &pb.ChainletID{Url: chaincodePath, Version: chaincodeVersion}}

	chainletDeploymentSpec, err := devopsClient.Build(context.Background(), spec)
	if err != nil {
		errMsg := fmt.Sprintf("Error building %s: %s\n", ChainFuncFormalName, err)
		cmd.Out().Write([]byte(errMsg))
		cmd.Usage()
		return
	}
	log.Info("Build result: %s", chainletDeploymentSpec.ChainletSpec)
}

func chaincodeTest(cmd *cobra.Command, args []string) error {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		return err
	}
	cmd.Out().Write([]byte(fmt.Sprintf("going to test %s: %s\n", ChainFuncFormalName, chaincodePath)))
	return nil
}

func chaincodeDeploy(cmd *cobra.Command, args []string) error {
	if err := checkChaincodeCmdParams(cmd); err != nil {
		return err
	}
	cmd.Out().Write([]byte(fmt.Sprintf("going to deploy %s: %s\n", ChainFuncFormalName, chaincodePath)))
	return nil
}
