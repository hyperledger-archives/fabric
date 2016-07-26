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

package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"google/protobuf"
	"io/ioutil"
	"os"

	"golang.org/x/net/context"

	"github.com/howeyc/gopass"
	"github.com/hyperledger/fabric/core"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
)

const networkFuncName = "network"

var logger = logging.MustGetLogger("networkCmd")

// login related variables.
var (
	loginPW string
)

// Cmd returns the cobra command for Network
func Cmd() *cobra.Command {
	// Set the flags on the login command.
	networkLoginCmd.PersistentFlags().StringVarP(&loginPW, "password", "p", common.UndefinedParamValue, "The password for user. You will be requested to enter the password if this flag is not specified.")

	networkCmd.AddCommand(networkLoginCmd)
	networkCmd.AddCommand(networkListCmd)

	return networkCmd
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

var networkListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Lists all network peers.",
	Long:    `Returns a list of all existing network connections for the target peer node, includes both validating and non-validating peers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return networkList()
	},
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
