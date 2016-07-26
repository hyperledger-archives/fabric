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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"google/protobuf"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	"github.com/howeyc/gopass"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "net/http/pprof"

	"github.com/hyperledger/fabric/core"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/peer"
	peerchaincode "github.com/hyperledger/fabric/peer/chaincode"
	"github.com/hyperledger/fabric/peer/node"
	"github.com/hyperledger/fabric/peer/util"
	"github.com/hyperledger/fabric/peer/version"
	pb "github.com/hyperledger/fabric/protos"
)

var logger = logging.MustGetLogger("main")

// Constants go here.
const fabric = "hyperledger"
const networkFuncName = "network"
const cmdRoot = "core"
const undefinedParamValue = ""

// The main command describes the service and
// defaults to printing the help message.
var mainCmd = &cobra.Command{
	Use: "peer",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return core.CacheConfiguration()
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		core.LoggingInit("peer")
	},
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			version.Print()
		} else {
			cmd.HelpFunc()(cmd, args)
		}
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   networkFuncName,
	Short: fmt.Sprintf("%s specific commands.", networkFuncName),
	Long:  fmt.Sprintf("%s specific commands.", networkFuncName),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.LoggingInit(networkFuncName)
	},
}

var networkLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Logs in user to CLI.",
	Long:  `Logs in the local user to CLI. Must supply username as a parameter.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return networkLogin(args)
	},
}

// var vmCmd = &cobra.Command{
// 	Use:   "vm",
// 	Short: "Accesses VM specific functionality.",
// 	Long:  `Accesses VM specific functionality.`,
// 	PersistentPreRun: func(cmd *cobra.Command, args []string) {
// 		core.LoggingInit("vm")
// 	},
// }
//
// var vmPrimeCmd = &cobra.Command{
// 	Use:   "prime",
// 	Short: "Primes the VM functionality.",
// 	Long:  `Primes the VM functionality by preparing the necessary VM construction artifacts.`,
// 	RunE: func(cmd *cobra.Command, args []string) error {
// 		return stop()
// 	},
// }

var networkListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Lists all network peers.",
	Long:    `Returns a list of all existing network connections for the target peer node, includes both validating and non-validating peers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return networkList()
	},
}

// login related variables.
var (
	loginPW string
)

// Peer command version flag
var versionFlag bool

func main() {
	// For environment variables.
	viper.SetEnvPrefix(cmdRoot)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	// Define command-line flags that are valid for all peer commands and
	// subcommands.
	mainFlags := mainCmd.PersistentFlags()
	mainFlags.BoolVarP(&versionFlag, "version", "v", false, "Display current version of fabric peer server")

	mainFlags.String("logging-level", "", "Default logging level and overrides, see core.yaml for full syntax")
	viper.BindPFlag("logging_level", mainFlags.Lookup("logging-level"))
	testCoverProfile := ""
	mainFlags.StringVarP(&testCoverProfile, "test.coverprofile", "", "coverage.cov", "Done")

	var alternativeCfgPath = os.Getenv("PEER_CFG_PATH")
	if alternativeCfgPath != "" {
		logger.Info("User defined config file path: %s", alternativeCfgPath)
		viper.AddConfigPath(alternativeCfgPath) // Path to look for the config file in
	} else {
		viper.AddConfigPath("./") // Path to look for the config file in
		// Path to look for the config file in based on GOPATH
		gopath := os.Getenv("GOPATH")
		for _, p := range filepath.SplitList(gopath) {
			peerpath := filepath.Join(p, "src/github.com/hyperledger/fabric/peer")
			viper.AddConfigPath(peerpath)
		}
	}

	// Now set the configuration file.
	viper.SetConfigName(cmdRoot) // Name of config file (without extension)

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error when reading %s config file: %s\n", cmdRoot, err))
	}

	mainCmd.AddCommand(version.Cmd())
	mainCmd.AddCommand(node.Cmd())
	// Set the flags on the login command.
	networkLoginCmd.PersistentFlags().StringVarP(&loginPW, "password", "p", undefinedParamValue, "The password for user. You will be requested to enter the password if this flag is not specified.")

	networkCmd.AddCommand(networkLoginCmd)

	// vmCmd.AddCommand(vmPrimeCmd)
	// mainCmd.AddCommand(vmCmd)

	networkCmd.AddCommand(networkListCmd)

	mainCmd.AddCommand(networkCmd)

	mainCmd.AddCommand(peerchaincode.Cmd())

	runtime.GOMAXPROCS(viper.GetInt("peer.gomaxprocs"))

	// Init the crypto layer
	if err := crypto.Init(); err != nil {
		panic(fmt.Errorf("Failed to initialize the crypto layer: %s", err))
	}

	// On failure Cobra prints the usage message and error string, so we only
	// need to exit with a non-0 status
	if mainCmd.Execute() != nil {
		os.Exit(1)
	}
	logger.Info("Exiting.....")
}

// login confirms the enrollmentID and secret password of the client with the
// CA and stores the enrollment certificate and key in the Devops server.
func networkLogin(args []string) (err error) {
	logger.Info("CLI client login...")

	// Check for username argument
	if len(args) == 0 {
		err = errors.New("Must supply username")
		return
	}

	// Check for other extraneous arguments
	if len(args) != 1 {
		err = errors.New("Must supply username as the 1st and only parameter")
		return
	}

	// Retrieve the CLI data storage path
	// Returns /var/openchain/production/client/
	localStore := util.GetCliFilePath()
	logger.Infof("Local data store for client loginToken: %s", localStore)

	// If the user is already logged in, return
	if _, err = os.Stat(localStore + "loginToken_" + args[0]); err == nil {
		logger.Infof("User '%s' is already logged in.\n", args[0])
		return
	}

	// If the '--password' flag is not specified, need read it from the terminal
	if loginPW == "" {
		// User is not logged in, prompt for password
		fmt.Printf("Enter password for user '%s': ", args[0])
		var pw []byte
		if pw, err = gopass.GetPasswdMasked(); err != nil {
			err = fmt.Errorf("Error trying to read password from console: %s", err)
			return
		}
		loginPW = string(pw)
	}

	// Log in the user
	logger.Infof("Logging in user '%s' on CLI interface...\n", args[0])

	// Get a devopsClient to perform the login
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		err = fmt.Errorf("Error trying to connect to local peer: %s", err)
		return
	}
	devopsClient := pb.NewDevopsClient(clientConn)

	// Build the login spec and login
	loginSpec := &pb.Secret{EnrollId: args[0], EnrollSecret: loginPW}
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
		logger.Infof("Storing login token for user '%s'.\n", args[0])
		err = ioutil.WriteFile(localStore+"loginToken_"+args[0], []byte(args[0]), 0755)
		if err != nil {
			panic(fmt.Errorf("Fatal error when storing client login token: %s\n", err))
		}

		logger.Infof("Login successful for user '%s'.\n", args[0])
	} else {
		err = fmt.Errorf("Error on client login: %s", string(loginResult.Msg))
		return
	}

	return nil
}

// Show a list of all existing network connections for the target peer node,
// includes both validating and non-validating peers
func networkList() (err error) {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		err = fmt.Errorf("Error trying to connect to local peer: %s", err)
		return
	}
	openchainClient := pb.NewOpenchainClient(clientConn)
	peers, err := openchainClient.GetPeers(context.Background(), &google_protobuf.Empty{})

	if err != nil {
		err = fmt.Errorf("Error trying to get peers: %s", err)
		return
	}

	jsonOutput, _ := json.Marshal(peers)
	fmt.Println(string(jsonOutput))
	return nil
}
