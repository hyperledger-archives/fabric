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

package chaincode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hyperledger/fabric/core"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/peer/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"golang.org/x/net/context"
)

const (
	chainFuncName       = "chaincode"
	undefinedParamValue = ""
)

var logger = logging.MustGetLogger("chaincodeCmd")

// Cmd returns the cobra command for Chaincode
func Cmd() *cobra.Command {
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeLang, "lang", "l", "golang", fmt.Sprintf("Language the %s is written in", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeCtorJSON, "ctor", "c", "{}", fmt.Sprintf("Constructor message for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeAttributesJSON, "attributes", "a", "[]", fmt.Sprintf("User attributes for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodePath, "path", "p", undefinedParamValue, fmt.Sprintf("Path to %s", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeName, "name", "n", undefinedParamValue, fmt.Sprintf("Name of the chaincode returned by the deploy transaction"))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeUsr, "username", "u", undefinedParamValue, fmt.Sprintf("Username for chaincode operations when security is enabled"))
	chaincodeCmd.PersistentFlags().StringVarP(&customIDGenAlg, "tid", "t", undefinedParamValue, fmt.Sprintf("Name of a custom ID generation algorithm (hashing and decoding) e.g. sha256base64"))

	chaincodeQueryCmd.Flags().BoolVarP(&chaincodeQueryRaw, "raw", "r", false, "If true, output the query value as raw bytes, otherwise format as a printable string")
	chaincodeQueryCmd.Flags().BoolVarP(&chaincodeQueryHex, "hex", "x", false, "If true, output the query value byte array in hexadecimal. Incompatible with --raw")

	chaincodeCmd.AddCommand(chaincodeDeployCmd)
	chaincodeCmd.AddCommand(chaincodeInvokeCmd)
	chaincodeCmd.AddCommand(chaincodeQueryCmd)

	return chaincodeCmd
}

// Chaincode-related variables.
var (
	chaincodeLang           string
	chaincodeCtorJSON       string
	chaincodePath           string
	chaincodeName           string
	chaincodeDevMode        bool
	chaincodeUsr            string
	chaincodeQueryRaw       bool
	chaincodeQueryHex       bool
	chaincodeAttributesJSON string
	customIDGenAlg          string
)

var chaincodeCmd = &cobra.Command{
	Use:   chainFuncName,
	Short: fmt.Sprintf("%s specific commands.", chainFuncName),
	Long:  fmt.Sprintf("%s specific commands.", chainFuncName),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.LoggingInit(chainFuncName)
	},
}

var chaincodeDeployCmd = &cobra.Command{
	Use:       "deploy",
	Short:     fmt.Sprintf("Deploy the specified %s to the network.", chainFuncName),
	Long:      fmt.Sprintf(`Deploy the specified %s to the network.`, chainFuncName),
	ValidArgs: []string{"1"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return chaincodeDeploy(cmd, args)
	},
}

var chaincodeInvokeCmd = &cobra.Command{
	Use:       "invoke",
	Short:     fmt.Sprintf("Invoke the specified %s.", chainFuncName),
	Long:      fmt.Sprintf(`Invoke the specified %s.`, chainFuncName),
	ValidArgs: []string{"1"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return chaincodeInvoke(cmd, args)
	},
}

var chaincodeQueryCmd = &cobra.Command{
	Use:       "query",
	Short:     fmt.Sprintf("Query using the specified %s.", chainFuncName),
	Long:      fmt.Sprintf(`Query using the specified %s.`, chainFuncName),
	ValidArgs: []string{"1"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return chaincodeQuery(cmd, args)
	},
}

// chaincodeDeploy deploys the chaincode. On success, the chaincode name
// (hash) is printed to STDOUT for use by subsequent chaincode-related CLI
// commands.
func chaincodeDeploy(cmd *cobra.Command, args []string) (err error) {
	if err = checkChaincodeCmdParams(cmd); err != nil {
		return
	}
	devopsClient, err := getDevopsClient(cmd)
	if err != nil {
		err = fmt.Errorf("Error building %s: %s", chainFuncName, err)
		return
	}
	// Build the spec
	input := &pb.ChaincodeInput{}
	if err = json.Unmarshal([]byte(chaincodeCtorJSON), &input); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	var attributes []string
	if err = json.Unmarshal([]byte(chaincodeAttributesJSON), &attributes); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	chaincodeLang = strings.ToUpper(chaincodeLang)
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[chaincodeLang]),
		ChaincodeID: &pb.ChaincodeID{Path: chaincodePath, Name: chaincodeName}, CtorMsg: input, Attributes: attributes}

	// If security is enabled, add client login token
	if core.SecurityEnabled() {
		logger.Debug("Security is enabled. Include security context in deploy spec")
		if chaincodeUsr == undefinedParamValue {
			err = errors.New("Must supply username for chaincode when security is enabled")
			return
		}

		// Retrieve the CLI data storage path
		// Returns /var/openchain/production/client/
		localStore := util.GetCliFilePath()

		// Check if the user is logged in before sending transaction
		if _, err = os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Infof("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				logger.Info("Set confidentiality level to CONFIDENTIAL.\n")
				spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				err = fmt.Errorf("User '%s' not logged in. Use the 'login' command to obtain a security token.", chaincodeUsr)
				return
			}
			// Unexpected error
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	} else {
		if chaincodeUsr != undefinedParamValue {
			logger.Warning("Username supplied but security is disabled.")
		}
		if viper.GetBool("security.privacy") {
			panic(errors.New("Privacy cannot be enabled as requested because security is disabled"))
		}
	}

	chaincodeDeploymentSpec, err := devopsClient.Deploy(context.Background(), spec)
	if err != nil {
		err = fmt.Errorf("Error building %s: %s\n", chainFuncName, err)
		return
	}
	logger.Infof("Deploy result: %s", chaincodeDeploymentSpec.ChaincodeSpec)
	fmt.Println(chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name)
	return nil
}

func chaincodeInvoke(cmd *cobra.Command, args []string) error {
	return chaincodeInvokeOrQuery(cmd, args, true)
}

func chaincodeQuery(cmd *cobra.Command, args []string) error {
	return chaincodeInvokeOrQuery(cmd, args, false)
}

// chaincodeInvokeOrQuery invokes or queries the chaincode. If successful, the
// INVOKE form prints the transaction ID on STDOUT, and the QUERY form prints
// the query result on STDOUT. A command-line flag (-r, --raw) determines
// whether the query result is output as raw bytes, or as a printable string.
// The printable form is optionally (-x, --hex) a hexadecimal representation
// of the query response. If the query response is NIL, nothing is output.
func chaincodeInvokeOrQuery(cmd *cobra.Command, args []string, invoke bool) (err error) {

	if err = checkChaincodeCmdParams(cmd); err != nil {
		return
	}

	if chaincodeName == "" {
		err = errors.New("Name not given for invoke/query")
		return
	}

	devopsClient, err := getDevopsClient(cmd)
	if err != nil {
		err = fmt.Errorf("Error building %s: %s", chainFuncName, err)
		return
	}
	// Build the spec
	input := &pb.ChaincodeInput{}
	if err = json.Unmarshal([]byte(chaincodeCtorJSON), &input); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	var attributes []string
	if err = json.Unmarshal([]byte(chaincodeAttributesJSON), &attributes); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	chaincodeLang = strings.ToUpper(chaincodeLang)
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[chaincodeLang]),
		ChaincodeID: &pb.ChaincodeID{Name: chaincodeName}, CtorMsg: input, Attributes: attributes}

	// If security is enabled, add client login token
	if core.SecurityEnabled() {
		if chaincodeUsr == undefinedParamValue {
			err = errors.New("Must supply username for chaincode when security is enabled")
			return
		}

		// Retrieve the CLI data storage path
		// Returns /var/openchain/production/client/
		localStore := util.GetCliFilePath()

		// Check if the user is logged in before sending transaction
		if _, err = os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Infof("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				logger.Info("Set confidentiality level to CONFIDENTIAL.\n")
				spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				err = fmt.Errorf("User '%s' not logged in. Use the 'login' command to obtain a security token.", chaincodeUsr)
				return
			}
			// Unexpected error
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	} else {
		if chaincodeUsr != undefinedParamValue {
			logger.Warning("Username supplied but security is disabled.")
		}
		if viper.GetBool("security.privacy") {
			panic(errors.New("Privacy cannot be enabled as requested because security is disabled"))
		}
	}

	// Build the ChaincodeInvocationSpec message
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	if customIDGenAlg != undefinedParamValue {
		invocation.IdGenerationAlg = customIDGenAlg
	}

	var resp *pb.Response
	if invoke {
		resp, err = devopsClient.Invoke(context.Background(), invocation)
	} else {
		resp, err = devopsClient.Query(context.Background(), invocation)
	}

	if err != nil {
		if invoke {
			err = fmt.Errorf("Error invoking %s: %s\n", chainFuncName, err)
		} else {
			err = fmt.Errorf("Error querying %s: %s\n", chainFuncName, err)
		}
		return
	}
	if invoke {
		transactionID := string(resp.Msg)
		logger.Infof("Successfully invoked transaction: %s(%s)", invocation, transactionID)
		fmt.Println(transactionID)
	} else {
		logger.Infof("Successfully queried transaction: %s", invocation)
		if resp != nil {
			if chaincodeQueryRaw {
				if chaincodeQueryHex {
					err = errors.New("Options --raw (-r) and --hex (-x) are not compatible\n")
					return
				}
				os.Stdout.Write(resp.Msg)
			} else {
				if chaincodeQueryHex {
					fmt.Printf("%x\n", resp.Msg)
				} else {
					fmt.Println(string(resp.Msg))
				}
			}
		}
	}
	return nil
}

func checkChaincodeCmdParams(cmd *cobra.Command) (err error) {

	if chaincodeName == undefinedParamValue {
		if chaincodePath == undefinedParamValue {
			err = fmt.Errorf("Must supply value for %s path parameter.\n", chainFuncName)
			return
		}
	}

	// Check that non-empty chaincode parameters contain only Function and
	// Args keys. Type checking is done later when the JSON is actually
	// unmarshaled into a pb.ChaincodeInput. To better understand what's going
	// on here with JSON parsing see http://blog.golang.org/json-and-go -
	// Generic JSON with interface{}
	if chaincodeCtorJSON != "{}" {
		var f interface{}
		err = json.Unmarshal([]byte(chaincodeCtorJSON), &f)
		if err != nil {
			err = fmt.Errorf("Chaincode argument error: %s", err)
			return
		}
		m := f.(map[string]interface{})
		if len(m) != 2 {
			err = fmt.Errorf("Non-empty JSON chaincode parameters must contain exactly 2 keys - 'Function' and 'Args'")
			return
		}
		for k := range m {
			switch strings.ToLower(k) {
			case "function":
			case "args":
			default:
				err = fmt.Errorf("Illegal chaincode key '%s' - must be either 'Function' or 'Args'", k)
				return
			}
		}
	} else {
		err = errors.New("Empty JSON chaincode parameters must contain exactly 2 keys - 'Function' and 'Args'")
		return
	}

	if chaincodeAttributesJSON != "[]" {
		var f interface{}
		err = json.Unmarshal([]byte(chaincodeAttributesJSON), &f)
		if err != nil {
			err = fmt.Errorf("Chaincode argument error: %s", err)
			return
		}
	}

	return
}

func getDevopsClient(cmd *cobra.Command) (pb.DevopsClient, error) {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		return nil, fmt.Errorf("Error trying to connect to local peer: %s", err)
	}
	devopsClient := pb.NewDevopsClient(clientConn)
	return devopsClient, nil
}
